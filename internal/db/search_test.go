package db

import (
	"context"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestSearchMessages(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent and topic.
	now := time.Now().Unix()
	agent, err := store.Queries().CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         "Searcher",
		CreatedAt:    now,
		LastActiveAt: now,
	})
	require.NoError(t, err)

	topic, err := store.Queries().CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      "search-topic",
		TopicType: "broadcast",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Create messages with different content.
	messages := []struct {
		subject string
		body    string
	}{
		{"Meeting Tomorrow", "Please join the team meeting at 10am"},
		{"Code Review", "The pull request needs your attention"},
		{"Lunch Plans", "Team lunch at noon tomorrow"},
		{"Project Update", "Status update on the current sprint"},
	}

	for i, m := range messages {
		_, err := store.Queries().CreateMessage(ctx, sqlc.CreateMessageParams{
			ThreadID:  "thread-" + string(rune('A'+i)),
			TopicID:   topic.ID,
			LogOffset: int64(i + 1),
			SenderID:  agent.ID,
			Subject:   m.subject,
			BodyMd:    m.body,
			Priority:  "normal",
			CreatedAt: time.Now().Unix(),
		})
		require.NoError(t, err)
	}

	// Search for "meeting".
	results, err := store.SearchMessages(ctx, "meeting", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Meeting Tomorrow", results[0].Subject)

	// Search for "tomorrow".
	results, err = store.SearchMessages(ctx, "tomorrow", 10)
	require.NoError(t, err)
	require.Len(t, results, 2) // Meeting Tomorrow and Lunch Plans

	// Search with limit.
	results, err = store.SearchMessages(ctx, "team", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestSearchMessagesForAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create agents.
	now := time.Now().Unix()
	sender, err := store.Queries().CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         "Sender",
		CreatedAt:    now,
		LastActiveAt: now,
	})
	require.NoError(t, err)

	recipient1, err := store.Queries().CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         "Recipient1",
		CreatedAt:    now,
		LastActiveAt: now,
	})
	require.NoError(t, err)

	recipient2, err := store.Queries().CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         "Recipient2",
		CreatedAt:    now,
		LastActiveAt: now,
	})
	require.NoError(t, err)

	// Create topic.
	topic, err := store.Queries().CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      "agent-search-topic",
		TopicType: "broadcast",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Create messages.
	msg1, err := store.Queries().CreateMessage(ctx, sqlc.CreateMessageParams{
		ThreadID:  "thread-1",
		TopicID:   topic.ID,
		LogOffset: 1,
		SenderID:  sender.ID,
		Subject:   "Task Assignment",
		BodyMd:    "Important task for recipient 1",
		Priority:  "normal",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	msg2, err := store.Queries().CreateMessage(ctx, sqlc.CreateMessageParams{
		ThreadID:  "thread-2",
		TopicID:   topic.ID,
		LogOffset: 2,
		SenderID:  sender.ID,
		Subject:   "Task Update",
		BodyMd:    "Task update for everyone",
		Priority:  "normal",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Add recipients.
	err = store.Queries().CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: msg1.ID,
		AgentID:   recipient1.ID,
	})
	require.NoError(t, err)

	err = store.Queries().CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: msg2.ID,
		AgentID:   recipient1.ID,
	})
	require.NoError(t, err)

	err = store.Queries().CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: msg2.ID,
		AgentID:   recipient2.ID,
	})
	require.NoError(t, err)

	// Search for recipient1 - should find both messages.
	results, err := store.SearchMessagesForAgent(ctx, "Task", recipient1.ID, 10)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Search for recipient2 - should find only msg2.
	results, err = store.SearchMessagesForAgent(ctx, "Task", recipient2.ID, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Task Update", results[0].Subject)
}

func TestSearchMessages_NoResults(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Search with no messages.
	results, err := store.SearchMessages(ctx, "nonexistent", 10)
	require.NoError(t, err)
	require.Empty(t, results)
}
