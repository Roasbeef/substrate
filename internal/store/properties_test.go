package store

import (
	"context"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// TestMessageDeliveryInvariant verifies that messages are delivered to all
// recipients.
func TestMessageDeliveryInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewMockStore()
		ctx := context.Background()

		// Create sender.
		sender, err := store.CreateAgent(ctx, CreateAgentParams{
			Name: rapid.StringMatching(`[A-Z][a-z]+`).Draw(t, "sender"),
		})
		if err != nil {
			t.Skip("duplicate name")
		}

		// Create recipients (1-5).
		numRecipients := rapid.IntRange(1, 5).Draw(t, "numRecipients")
		recipients := make([]Agent, numRecipients)
		for i := 0; i < numRecipients; i++ {
			r, err := store.CreateAgent(ctx, CreateAgentParams{
				Name: rapid.StringMatching(`[A-Z][a-z]+[0-9]+`).Draw(t, "recipient"),
			})
			if err != nil {
				t.Skip("duplicate name")
			}
			recipients[i] = r
		}

		// Create a topic for the message.
		topic, err := store.GetOrCreateTopic(ctx, "test-topic", "inbox")
		if err != nil {
			t.Fatal(err)
		}

		// Send message.
		offset, err := store.NextLogOffset(ctx, topic.ID)
		if err != nil {
			t.Fatal(err)
		}

		msg, err := store.CreateMessage(ctx, CreateMessageParams{
			ThreadID:  "thread-1",
			TopicID:   topic.ID,
			LogOffset: offset,
			SenderID:  sender.ID,
			Subject:   rapid.String().Draw(t, "subject"),
			Body:      rapid.String().Draw(t, "body"),
			Priority:  "normal",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Add recipients.
		for _, r := range recipients {
			err := store.CreateMessageRecipient(ctx, msg.ID, r.ID)
			if err != nil {
				t.Fatal(err)
			}
		}

		// PROPERTY: Message reaches ALL recipients.
		for _, r := range recipients {
			inbox, err := store.GetInboxMessages(ctx, r.ID, 100)
			if err != nil {
				t.Fatal(err)
			}

			found := false
			for _, m := range inbox {
				if m.ID == msg.ID {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("recipient %d should have message %d", r.ID, msg.ID)
			}
		}

		// PROPERTY: Store remains consistent.
		if !store.IsConsistent() {
			t.Fatal("store consistency violated")
		}
	})
}

// TestConcurrentActorAccess verifies that concurrent operations don't corrupt
// state.
func TestConcurrentActorAccess(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewMockStore()
		ctx := context.Background()

		// Create some initial agents.
		numAgents := rapid.IntRange(2, 10).Draw(t, "numAgents")
		agents := make([]Agent, numAgents)
		for i := 0; i < numAgents; i++ {
			a, err := store.CreateAgent(ctx, CreateAgentParams{
				Name: rapid.StringMatching(`Agent[0-9]+`).Draw(t, "agent"),
			})
			if err != nil {
				t.Skip("duplicate name")
			}
			agents[i] = a
		}

		// Create a topic.
		topic, err := store.GetOrCreateTopic(ctx, "concurrent-topic", "inbox")
		if err != nil {
			t.Fatal(err)
		}

		// Perform concurrent operations.
		numOps := rapid.IntRange(10, 50).Draw(t, "numOps")
		var wg sync.WaitGroup

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(opIdx int) {
				defer wg.Done()

				senderIdx := opIdx % len(agents)
				recipientIdx := (opIdx + 1) % len(agents)

				// Send a message.
				offset, _ := store.NextLogOffset(ctx, topic.ID)
				msg, err := store.CreateMessage(ctx, CreateMessageParams{
					ThreadID:  "concurrent-thread",
					TopicID:   topic.ID,
					LogOffset: offset,
					SenderID:  agents[senderIdx].ID,
					Subject:   "Concurrent message",
					Body:      "Test body",
					Priority:  "normal",
				})
				if err != nil {
					return
				}

				_ = store.CreateMessageRecipient(ctx, msg.ID, agents[recipientIdx].ID)
			}(i)
		}

		wg.Wait()

		// PROPERTY: No panics, no data corruption.
		// Verify store consistency.
		if !store.IsConsistent() {
			t.Fatal("store consistency violated after concurrent access")
		}

		// PROPERTY: All messages have valid senders.
		for msgID, msg := range store.messages {
			if _, err := store.GetAgent(ctx, msg.SenderID); err != nil {
				t.Fatalf("message %d has invalid sender %d", msgID, msg.SenderID)
			}
		}
	})
}

// TestMessageStateTransitions verifies valid state transitions.
func TestMessageStateTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewMockStore()
		ctx := context.Background()

		// Create sender and recipient.
		sender, _ := store.CreateAgent(ctx, CreateAgentParams{Name: "Sender"})
		recipient, _ := store.CreateAgent(ctx, CreateAgentParams{Name: "Recipient"})
		topic, _ := store.GetOrCreateTopic(ctx, "test-topic", "inbox")

		// Create a message.
		msg, _ := store.CreateMessage(ctx, CreateMessageParams{
			ThreadID:  "thread-1",
			TopicID:   topic.ID,
			LogOffset: 1,
			SenderID:  sender.ID,
			Subject:   "Test",
			Body:      "Body",
			Priority:  "normal",
		})
		_ = store.CreateMessageRecipient(ctx, msg.ID, recipient.ID)

		// Verify initial state is "unread".
		recip, _ := store.GetMessageRecipient(ctx, msg.ID, recipient.ID)
		if recip.State != "unread" {
			t.Fatalf("expected initial state 'unread', got %q", recip.State)
		}

		// Apply random valid state transitions.
		validTransitions := map[string][]string{
			"unread":   {"read", "starred", "archived", "trash"},
			"read":     {"starred", "archived", "trash"},
			"starred":  {"read", "archived", "trash"},
			"archived": {"read"},
			"trash":    {"read"},
		}

		numTransitions := rapid.IntRange(1, 10).Draw(t, "numTransitions")
		currentState := "unread"

		for i := 0; i < numTransitions; i++ {
			nextStates := validTransitions[currentState]
			if len(nextStates) == 0 {
				break
			}

			nextState := nextStates[rapid.IntRange(0, len(nextStates)-1).Draw(t, "stateIdx")]

			err := store.UpdateRecipientState(ctx, msg.ID, recipient.ID, nextState)
			if err != nil {
				t.Fatalf("failed to transition from %s to %s: %v",
					currentState, nextState, err)
			}

			// Verify the state changed.
			recip, _ = store.GetMessageRecipient(ctx, msg.ID, recipient.ID)
			if recip.State != nextState {
				t.Fatalf("expected state %q after transition, got %q",
					nextState, recip.State)
			}

			currentState = nextState
		}
	})
}

// TestUnreadCountInvariant verifies unread count consistency.
func TestUnreadCountInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewMockStore()
		ctx := context.Background()

		// Create agents.
		sender, _ := store.CreateAgent(ctx, CreateAgentParams{Name: "Sender"})
		recipient, _ := store.CreateAgent(ctx, CreateAgentParams{Name: "Recipient"})
		topic, _ := store.GetOrCreateTopic(ctx, "test-topic", "inbox")

		// Send some messages.
		numMessages := rapid.IntRange(1, 20).Draw(t, "numMessages")
		for i := 0; i < numMessages; i++ {
			msg, _ := store.CreateMessage(ctx, CreateMessageParams{
				ThreadID:  "thread-1",
				TopicID:   topic.ID,
				LogOffset: int64(i + 1),
				SenderID:  sender.ID,
				Subject:   "Test",
				Body:      "Body",
				Priority:  "normal",
			})
			_ = store.CreateMessageRecipient(ctx, msg.ID, recipient.ID)
		}

		// Verify initial unread count.
		count, _ := store.CountUnreadByAgent(ctx, recipient.ID)
		if count != int64(numMessages) {
			t.Fatalf("expected %d unread, got %d", numMessages, count)
		}

		// Mark some as read.
		numToRead := rapid.IntRange(0, numMessages).Draw(t, "numToRead")
		inbox, _ := store.GetInboxMessages(ctx, recipient.ID, 100)

		for i := 0; i < numToRead && i < len(inbox); i++ {
			_ = store.MarkMessageRead(ctx, inbox[i].ID, recipient.ID)
		}

		// Verify unread count.
		count, _ = store.CountUnreadByAgent(ctx, recipient.ID)
		expected := int64(numMessages - numToRead)
		if count != expected {
			t.Fatalf("expected %d unread after reading %d, got %d",
				expected, numToRead, count)
		}
	})
}

// TestTopicSubscriptionInvariant verifies subscription consistency.
func TestTopicSubscriptionInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewMockStore()
		ctx := context.Background()

		// Create agents with unique names.
		numAgents := rapid.IntRange(2, 10).Draw(t, "numAgents")
		agents := make([]Agent, 0, numAgents)
		for i := 0; i < numAgents; i++ {
			name := rapid.StringMatching(`Sub[A-Z][a-z]+[0-9]+`).Draw(t, "agent")
			a, err := store.CreateAgent(ctx, CreateAgentParams{
				Name: name,
			})
			if err != nil {
				// Skip duplicates.
				continue
			}
			agents = append(agents, a)
		}

		// Need at least 2 agents for this test.
		if len(agents) < 2 {
			t.Skip("not enough unique agents")
		}

		// Create topic.
		topic, _ := store.CreateTopic(ctx, CreateTopicParams{
			Name:      "broadcast-topic",
			TopicType: "broadcast",
		})

		// Subscribe random agents.
		subscribed := make(map[int64]bool)
		for _, a := range agents {
			if rapid.Bool().Draw(t, "subscribe") {
				_ = store.CreateSubscription(ctx, a.ID, topic.ID)
				subscribed[a.ID] = true
			}
		}

		// Verify subscriptions.
		subs, _ := store.ListSubscriptionsByTopic(ctx, topic.ID)
		if len(subs) != len(subscribed) {
			t.Fatalf("expected %d subscribers, got %d",
				len(subscribed), len(subs))
		}

		for _, s := range subs {
			if !subscribed[s.ID] {
				t.Fatalf("agent %d listed as subscriber but wasn't subscribed",
					s.ID)
			}
		}

		// Unsubscribe some.
		for _, a := range agents {
			if subscribed[a.ID] && rapid.Bool().Draw(t, "unsubscribe") {
				_ = store.DeleteSubscription(ctx, a.ID, topic.ID)
				delete(subscribed, a.ID)
			}
		}

		// Verify final subscriptions.
		subs, _ = store.ListSubscriptionsByTopic(ctx, topic.ID)
		if len(subs) != len(subscribed) {
			t.Fatalf("after unsubscribe: expected %d subscribers, got %d",
				len(subscribed), len(subs))
		}
	})
}
