package store

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/stretchr/testify/require"
)

// newTestStore creates a real SqlcStore backed by a temporary SQLite
// database with migrations auto-applied.
func newTestStore(t *testing.T) Storage {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	sqliteStore, err := db.NewSqliteStore(&db.SqliteConfig{
		DatabaseFileName:      dbPath,
		SkipMigrationDBBackup: true,
	}, log)
	require.NoError(t, err)

	storage := FromDB(sqliteStore.DB())
	t.Cleanup(func() {
		storage.Close()
	})

	return storage
}

// createAgent is a helper to create an agent and return it.
func createAgent(
	t *testing.T, s Storage, name string,
) Agent {

	t.Helper()

	agent, err := s.CreateAgent(
		context.Background(), CreateAgentParams{Name: name},
	)
	require.NoError(t, err)

	return agent
}

// sendMessage is a helper to create a message from sender to recipient.
func sendMessage(
	t *testing.T, s Storage, senderID int64,
	recipientID int64, subject, body string,
) Message {

	t.Helper()
	ctx := context.Background()

	topic, err := s.GetOrCreateTopic(ctx, "test-topic", "direct")
	require.NoError(t, err)

	offset, err := s.NextLogOffset(ctx, topic.ID)
	require.NoError(t, err)

	msg, err := s.CreateMessage(ctx, CreateMessageParams{
		ThreadID:  "review-test-thread",
		TopicID:   topic.ID,
		LogOffset: offset,
		SenderID:  senderID,
		Subject:   subject,
		Body:      body,
		Priority:  "normal",
	})
	require.NoError(t, err)

	err = s.CreateMessageRecipient(ctx, msg.ID, recipientID)
	require.NoError(t, err)

	return msg
}

// TestGetMessagesBySenderNamePrefix_ReturnsReadState verifies that the
// sender-prefix query (used by CodeReviewer aggregate view) returns
// per-recipient read state. This is a regression test for the bug
// where messages always appeared unread because the SQL query did not
// join message_recipients.
func TestGetMessagesBySenderNamePrefix_ReturnsReadState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	reviewer := createAgent(t, s, "reviewer-CodeReviewer-abc123")
	recipient := createAgent(t, s, "AzureHaven")

	sendMessage(
		t, s, reviewer.ID, recipient.ID,
		"Review: approve", "Looks good!",
	)

	// Fetch via sender prefix — should be unread initially.
	messages, err := s.GetMessagesBySenderNamePrefix(
		ctx, "reviewer-", 100,
	)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "unread", messages[0].State)
	require.Nil(t, messages[0].ReadAt)

	// Mark the message as read.
	err = s.MarkMessageRead(ctx, messages[0].ID, recipient.ID)
	require.NoError(t, err)

	// Fetch again — should now show as read with ReadAt set.
	messages, err = s.GetMessagesBySenderNamePrefix(
		ctx, "reviewer-", 100,
	)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "read", messages[0].State)
	require.NotNil(t, messages[0].ReadAt,
		"ReadAt should be set after marking read",
	)
}

// TestGetMessagesBySenderNamePrefix_FiltersArchived verifies that the
// sender-prefix query excludes archived and trashed messages, matching
// the behavior of GetInboxMessages and GetAllInboxMessages.
func TestGetMessagesBySenderNamePrefix_FiltersArchived(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	reviewer := createAgent(t, s, "reviewer-SecurityReviewer-xyz")
	recipient := createAgent(t, s, "TestAgent")

	// Create two messages.
	msg1 := sendMessage(
		t, s, reviewer.ID, recipient.ID,
		"Review 1", "First review",
	)
	msg2 := sendMessage(
		t, s, reviewer.ID, recipient.ID,
		"Review 2", "Second review",
	)

	// Archive the first message.
	err := s.UpdateRecipientState(
		ctx, msg1.ID, recipient.ID, "archived",
	)
	require.NoError(t, err)

	// Fetch — should only see the second (non-archived) message.
	messages, err := s.GetMessagesBySenderNamePrefix(
		ctx, "reviewer-", 100,
	)
	require.NoError(t, err)
	require.Len(t, messages, 1,
		"archived message should be excluded",
	)
	require.Equal(t, msg2.ID, messages[0].ID)
}

// TestGetMessagesBySenderNamePrefix_PrefixFiltering verifies that only
// messages from agents whose name starts with the given prefix are
// returned, not messages from other agents.
func TestGetMessagesBySenderNamePrefix_PrefixFiltering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	reviewer := createAgent(t, s, "reviewer-CodeReviewer-abc")
	regularAgent := createAgent(t, s, "SomeOtherAgent")
	recipient := createAgent(t, s, "RecipientAgent")

	// Send one message from each sender.
	sendMessage(
		t, s, reviewer.ID, recipient.ID,
		"Review: approve", "Good code",
	)
	sendMessage(
		t, s, regularAgent.ID, recipient.ID,
		"Hello", "Regular message",
	)

	// Fetch with "reviewer-" prefix — should only get the
	// reviewer's message.
	messages, err := s.GetMessagesBySenderNamePrefix(
		ctx, "reviewer-", 100,
	)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t,
		"reviewer-CodeReviewer-abc", messages[0].SenderName,
	)
}

// TestGetMessagesBySenderNamePrefix_SnoozedState verifies that snoozed
// messages include the snoozed_until timestamp.
func TestGetMessagesBySenderNamePrefix_SnoozedState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	reviewer := createAgent(t, s, "reviewer-Snooze-test")
	recipient := createAgent(t, s, "SnoozeRecipient")

	msg := sendMessage(
		t, s, reviewer.ID, recipient.ID,
		"Review: snooze test", "Body",
	)

	// Snooze the message.
	snoozeTime := time.Now().Add(24 * time.Hour)
	err := s.SnoozeMessage(ctx, msg.ID, recipient.ID, snoozeTime)
	require.NoError(t, err)

	// Fetch — should show as snoozed with SnoozedUntil set.
	messages, err := s.GetMessagesBySenderNamePrefix(
		ctx, "reviewer-", 100,
	)
	require.NoError(t, err)

	// Snoozed messages are filtered out by the NOT IN
	// ('archived', 'trash') clause, but 'snoozed' is allowed.
	require.Len(t, messages, 1)
	require.Equal(t, "snoozed", messages[0].State)
	require.NotNil(t, messages[0].SnoozedUntil,
		"SnoozedUntil should be set for snoozed messages",
	)
}
