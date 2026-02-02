package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCreateMessage_SetsTimestamp verifies that CreateMessage sets created_at
// to the current time automatically.
func TestCreateMessage_SetsTimestamp(t *testing.T) {
	t.Parallel()

	store := NewMockStore()
	ctx := context.Background()

	// Create sender.
	sender, err := store.CreateAgent(ctx, CreateAgentParams{
		Name: "TimestampTestSender",
	})
	require.NoError(t, err)

	// Create topic.
	topic, err := store.GetOrCreateTopic(ctx, "timestamp-test", "inbox")
	require.NoError(t, err)

	// Get offset.
	offset, err := store.NextLogOffset(ctx, topic.ID)
	require.NoError(t, err)

	// Record time before creating message.
	beforeCreate := time.Now().Unix()

	// Create message.
	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID:  "test-thread",
		TopicID:   topic.ID,
		LogOffset: offset,
		SenderID:  sender.ID,
		Subject:   "Timestamp Test",
		Body:      "Testing that timestamp is set",
		Priority:  "normal",
	})
	require.NoError(t, err)

	// Record time after creating message.
	afterCreate := time.Now().Unix()

	// Verify the timestamp is set and is within the expected range.
	require.NotZero(t, msg.CreatedAt, "created_at should not be zero")

	createdAtUnix := msg.CreatedAt.Unix()
	require.GreaterOrEqual(t, createdAtUnix, beforeCreate,
		"created_at should be >= time before create")
	require.LessOrEqual(t, createdAtUnix, afterCreate,
		"created_at should be <= time after create")
}

// TestCreateMessage_TimestampOrder verifies that messages created later have
// later timestamps, ensuring proper sorting.
func TestCreateMessage_TimestampOrder(t *testing.T) {
	t.Parallel()

	store := NewMockStore()
	ctx := context.Background()

	// Create sender.
	sender, err := store.CreateAgent(ctx, CreateAgentParams{
		Name: "OrderTestSender",
	})
	require.NoError(t, err)

	// Create topic.
	topic, err := store.GetOrCreateTopic(ctx, "order-test", "inbox")
	require.NoError(t, err)

	// Create first message.
	offset1, _ := store.NextLogOffset(ctx, topic.ID)
	msg1, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID:  "thread-1",
		TopicID:   topic.ID,
		LogOffset: offset1,
		SenderID:  sender.ID,
		Subject:   "First Message",
		Body:      "Created first",
		Priority:  "normal",
	})
	require.NoError(t, err)

	// Small delay to ensure different timestamp.
	time.Sleep(10 * time.Millisecond)

	// Create second message.
	offset2, _ := store.NextLogOffset(ctx, topic.ID)
	msg2, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID:  "thread-2",
		TopicID:   topic.ID,
		LogOffset: offset2,
		SenderID:  sender.ID,
		Subject:   "Second Message",
		Body:      "Created second",
		Priority:  "normal",
	})
	require.NoError(t, err)

	// Second message should have a timestamp >= first message.
	require.GreaterOrEqual(t, msg2.CreatedAt.Unix(), msg1.CreatedAt.Unix(),
		"second message should have timestamp >= first message")
}
