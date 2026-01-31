package mail

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestMailActor_SendAndReceive tests the full actor-based message flow.
func TestMailActor_SendAndReceive(t *testing.T) {
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create the mail actor.
	mailActor := NewMailActor(ActorConfig{
		ID:          "test-mail-actor",
		Store:       storage,
		MailboxSize: 10,
	})
	mailActor.Start()
	defer mailActor.Stop()

	ref := mailActor.Ref()

	// Create sender and recipient agents directly via store.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "TestSender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "TestRecipient",
	})
	require.NoError(t, err)

	// Send a message via the actor.
	sendFuture := ref.Ask(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"TestRecipient"},
		Subject:        "Hello from actor test",
		Body:           "This is a test message sent via the actor system.",
		Priority:       PriorityNormal,
	})

	sendResult := sendFuture.Await(ctx)
	sendResp, err := sendResult.Unpack()
	require.NoError(t, err)

	resp := sendResp.(SendMailResponse)
	require.NoError(t, resp.Error)
	require.NotZero(t, resp.MessageID)
	require.NotEmpty(t, resp.ThreadID)

	t.Logf("Sent message with ID %d, thread %s", resp.MessageID, resp.ThreadID)

	// Fetch inbox via the actor.
	inboxFuture := ref.Ask(ctx, FetchInboxRequest{
		AgentID: recipient.ID,
		Limit:   10,
	})

	inboxResult := inboxFuture.Await(ctx)
	inboxResp, err := inboxResult.Unpack()
	require.NoError(t, err)

	inbox := inboxResp.(FetchInboxResponse)
	require.NoError(t, inbox.Error)
	require.Len(t, inbox.Messages, 1)
	require.Equal(t, "Hello from actor test", inbox.Messages[0].Subject)

	// Read the message via the actor.
	readFuture := ref.Ask(ctx, ReadMessageRequest{
		AgentID:   recipient.ID,
		MessageID: resp.MessageID,
	})

	readResult := readFuture.Await(ctx)
	readResp, err := readResult.Unpack()
	require.NoError(t, err)

	read := readResp.(ReadMessageResponse)
	require.NoError(t, read.Error)
	require.NotNil(t, read.Message)
	require.Contains(t, read.Message.Body, "test message sent via the actor system")

	// Get status via the actor.
	statusFuture := ref.Ask(ctx, GetStatusRequest{
		AgentID: recipient.ID,
	})

	statusResult := statusFuture.Await(ctx)
	statusResp, err := statusResult.Unpack()
	require.NoError(t, err)

	status := statusResp.(GetStatusResponse)
	require.NoError(t, status.Error)
	require.Equal(t, "TestRecipient", status.Status.AgentName)
}

// TestMailActor_ConcurrentRequests tests concurrent message handling.
func TestMailActor_ConcurrentRequests(t *testing.T) {
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create the mail actor.
	mailActor := NewMailActor(ActorConfig{
		ID:          "test-concurrent-actor",
		Store:       storage,
		MailboxSize: 100,
	})
	mailActor.Start()
	defer mailActor.Stop()

	ref := mailActor.Ref()

	// Create agents.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "ConcurrentSender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "ConcurrentRecipient",
	})
	require.NoError(t, err)

	// Send multiple messages concurrently.
	numMessages := 10
	futures := make([]actor.Future[MailResponse], numMessages)

	for i := range numMessages {
		futures[i] = ref.Ask(ctx, SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: []string{"ConcurrentRecipient"},
			Subject:        "Concurrent message",
			Body:           "Test body",
			Priority:       PriorityNormal,
		})
	}

	// Wait for all messages to be sent.
	for i, future := range futures {
		result := future.Await(ctx)
		resp, err := result.Unpack()
		require.NoError(t, err)

		sendResp := resp.(SendMailResponse)
		require.NoError(t, sendResp.Error, "message %d failed", i)
	}

	// Verify all messages were received.
	inboxFuture := ref.Ask(ctx, FetchInboxRequest{
		AgentID: recipient.ID,
		Limit:   100,
	})

	inboxResult := inboxFuture.Await(ctx)
	inboxResp, err := inboxResult.Unpack()
	require.NoError(t, err)

	inbox := inboxResp.(FetchInboxResponse)
	require.NoError(t, inbox.Error)
	require.Len(t, inbox.Messages, numMessages)
}

// TestMailActor_StateTransitions tests message state transitions via actor.
func TestMailActor_StateTransitions(t *testing.T) {
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	mailActor := NewMailActor(ActorConfig{
		ID:          "test-state-actor",
		Store:       storage,
		MailboxSize: 10,
	})
	mailActor.Start()
	defer mailActor.Stop()

	ref := mailActor.Ref()

	// Create agents.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "StateSender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "StateRecipient",
	})
	require.NoError(t, err)

	// Send a message.
	sendResult := ref.Ask(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"StateRecipient"},
		Subject:        "State test",
		Body:           "Testing state transitions",
		Priority:       PriorityNormal,
	}).Await(ctx)

	sendResp, err := sendResult.Unpack()
	require.NoError(t, err)

	msgID := sendResp.(SendMailResponse).MessageID

	// Test read state transition.
	readResult := ref.Ask(ctx, ReadMessageRequest{
		AgentID:   recipient.ID,
		MessageID: msgID,
	}).Await(ctx)

	readResp, err := readResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, readResp.(ReadMessageResponse).Error)

	// Test star state transition.
	starResult := ref.Ask(ctx, UpdateStateRequest{
		AgentID:   recipient.ID,
		MessageID: msgID,
		NewState:  "starred",
	}).Await(ctx)

	starResp, err := starResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, starResp.(UpdateStateResponse).Error)

	// Test archive state transition.
	archiveResult := ref.Ask(ctx, UpdateStateRequest{
		AgentID:   recipient.ID,
		MessageID: msgID,
		NewState:  "archived",
	}).Await(ctx)

	archiveResp, err := archiveResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, archiveResp.(UpdateStateResponse).Error)

	// Test ack.
	ackResult := ref.Ask(ctx, AckMessageRequest{
		AgentID:   recipient.ID,
		MessageID: msgID,
	}).Await(ctx)

	ackResp, err := ackResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, ackResp.(AckMessageResponse).Error)
}

// TestMailActor_MultipleRecipients tests sending to multiple recipients.
func TestMailActor_MultipleRecipients(t *testing.T) {
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	mailActor := NewMailActor(ActorConfig{
		ID:          "test-multi-recipient-actor",
		Store:       storage,
		MailboxSize: 10,
	})
	mailActor.Start()
	defer mailActor.Stop()

	ref := mailActor.Ref()

	// Create sender and multiple recipients.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "MultiSender",
	})
	require.NoError(t, err)

	recipients := make([]store.Agent, 5)
	recipientNames := make([]string, 5)
	for i := range 5 {
		recipients[i], err = storage.CreateAgent(ctx, store.CreateAgentParams{
			Name: fmt.Sprintf("MultiRecipient%d", i),
		})
		require.NoError(t, err)
		recipientNames[i] = recipients[i].Name
	}

	// Send message to all recipients.
	sendResult := ref.Ask(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: recipientNames,
		Subject:        "Multi-recipient test",
		Body:           "This goes to everyone",
		Priority:       PriorityNormal,
	}).Await(ctx)

	sendResp, err := sendResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, sendResp.(SendMailResponse).Error)

	// Verify each recipient received the message.
	for _, r := range recipients {
		inboxResult := ref.Ask(ctx, FetchInboxRequest{
			AgentID: r.ID,
			Limit:   10,
		}).Await(ctx)

		inboxResp, err := inboxResult.Unpack()
		require.NoError(t, err)

		inbox := inboxResp.(FetchInboxResponse)
		require.NoError(t, inbox.Error)
		require.Len(t, inbox.Messages, 1, "recipient %s should have 1 message", r.Name)
		require.Equal(t, "Multi-recipient test", inbox.Messages[0].Subject)
	}
}

// TestMailActor_ThreadReply tests replying within a thread.
func TestMailActor_ThreadReply(t *testing.T) {
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	mailActor := NewMailActor(ActorConfig{
		ID:          "test-thread-reply-actor",
		Store:       storage,
		MailboxSize: 10,
	})
	mailActor.Start()
	defer mailActor.Stop()

	ref := mailActor.Ref()

	// Create two agents.
	alice, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Alice",
	})
	require.NoError(t, err)

	bob, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Bob",
	})
	require.NoError(t, err)

	// Alice sends initial message.
	sendResult := ref.Ask(ctx, SendMailRequest{
		SenderID:       alice.ID,
		RecipientNames: []string{"Bob"},
		Subject:        "Hello Bob",
		Body:           "How are you?",
		Priority:       PriorityNormal,
	}).Await(ctx)

	sendResp, err := sendResult.Unpack()
	require.NoError(t, err)

	firstMsg := sendResp.(SendMailResponse)
	require.NoError(t, firstMsg.Error)
	threadID := firstMsg.ThreadID

	// Bob replies in the same thread.
	replyResult := ref.Ask(ctx, SendMailRequest{
		SenderID:       bob.ID,
		RecipientNames: []string{"Alice"},
		Subject:        "Re: Hello Bob",
		Body:           "I am fine, thanks!",
		Priority:       PriorityNormal,
		ThreadID:       threadID,
	}).Await(ctx)

	replyResp, err := replyResult.Unpack()
	require.NoError(t, err)

	reply := replyResp.(SendMailResponse)
	require.NoError(t, reply.Error)
	require.Equal(t, threadID, reply.ThreadID, "reply should be in same thread")

	// Alice replies again.
	reply2Result := ref.Ask(ctx, SendMailRequest{
		SenderID:       alice.ID,
		RecipientNames: []string{"Bob"},
		Subject:        "Re: Re: Hello Bob",
		Body:           "Great to hear!",
		Priority:       PriorityNormal,
		ThreadID:       threadID,
	}).Await(ctx)

	reply2Resp, err := reply2Result.Unpack()
	require.NoError(t, err)

	reply2 := reply2Resp.(SendMailResponse)
	require.NoError(t, reply2.Error)
	require.Equal(t, threadID, reply2.ThreadID, "second reply should be in same thread")
}

// Property-based tests using the actor system.

// TestMailActorProperty_DeliveryInvariant verifies all messages reach all
// recipients via the actor system.
func TestMailActorProperty_DeliveryInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		storage, cleanup := testDB(t)
		defer cleanup()

		ctx := context.Background()

		mailActor := NewMailActor(ActorConfig{
			ID:          "property-test-actor",
			Store:       storage,
			MailboxSize: 100,
		})
		mailActor.Start()
		defer mailActor.Stop()

		ref := mailActor.Ref()

		// Create sender.
		senderName := rapid.StringMatching(`Sender[A-Z][a-z]+`).Draw(rt, "sender")
		sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
			Name: senderName,
		})
		if err != nil {
			rt.Skip("duplicate sender name")
		}

		// Create recipients (1-5).
		numRecipients := rapid.IntRange(1, 5).Draw(rt, "numRecipients")
		recipients := make([]store.Agent, 0, numRecipients)
		recipientNames := make([]string, 0, numRecipients)

		for i := 0; i < numRecipients; i++ {
			name := rapid.StringMatching(`Recip[A-Z][a-z]+[0-9]+`).Draw(rt, "recipient")
			r, err := storage.CreateAgent(ctx, store.CreateAgentParams{
				Name: name,
			})
			if err != nil {
				continue // skip duplicates
			}
			recipients = append(recipients, r)
			recipientNames = append(recipientNames, name)
		}

		if len(recipients) == 0 {
			rt.Skip("no valid recipients")
		}

		// Send message via actor.
		subject := rapid.String().Draw(rt, "subject")
		body := rapid.String().Draw(rt, "body")

		sendResult := ref.Ask(ctx, SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: recipientNames,
			Subject:        subject,
			Body:           body,
			Priority:       PriorityNormal,
		}).Await(ctx)

		sendResp, err := sendResult.Unpack()
		if err != nil {
			rt.Fatalf("actor request failed: %v", err)
		}

		resp := sendResp.(SendMailResponse)
		if resp.Error != nil {
			rt.Fatalf("send failed: %v", resp.Error)
		}

		// PROPERTY: Message reaches ALL recipients via actor.
		for _, r := range recipients {
			inboxResult := ref.Ask(ctx, FetchInboxRequest{
				AgentID: r.ID,
				Limit:   100,
			}).Await(ctx)

			inboxResp, err := inboxResult.Unpack()
			if err != nil {
				rt.Fatalf("inbox fetch failed: %v", err)
			}

			inbox := inboxResp.(FetchInboxResponse)
			if inbox.Error != nil {
				rt.Fatalf("inbox error: %v", inbox.Error)
			}

			found := false
			for _, m := range inbox.Messages {
				if m.ID == resp.MessageID {
					found = true
					break
				}
			}

			if !found {
				rt.Fatalf("recipient %s should have message %d", r.Name, resp.MessageID)
			}
		}
	})
}

// TestMailActorProperty_ConcurrentAccess verifies concurrent actor access
// doesn't cause panics or corruption.
func TestMailActorProperty_ConcurrentAccess(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		storage, cleanup := testDB(t)
		defer cleanup()

		ctx := context.Background()

		mailActor := NewMailActor(ActorConfig{
			ID:          "concurrent-property-actor",
			Store:       storage,
			MailboxSize: 200,
		})
		mailActor.Start()
		defer mailActor.Stop()

		ref := mailActor.Ref()

		// Create agents.
		numAgents := rapid.IntRange(2, 10).Draw(rt, "numAgents")
		agents := make([]store.Agent, 0, numAgents)

		for i := 0; i < numAgents; i++ {
			name := rapid.StringMatching(`ConcAgent[0-9]+`).Draw(rt, "agent")
			a, err := storage.CreateAgent(ctx, store.CreateAgentParams{
				Name: name,
			})
			if err != nil {
				continue // skip duplicates
			}
			agents = append(agents, a)
		}

		if len(agents) < 2 {
			rt.Skip("not enough unique agents")
		}

		// Perform concurrent operations.
		numOps := rapid.IntRange(10, 50).Draw(rt, "numOps")
		var wg sync.WaitGroup
		errChan := make(chan error, numOps)

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(opIdx int) {
				defer wg.Done()

				senderIdx := opIdx % len(agents)
				recipientIdx := (opIdx + 1) % len(agents)

				// Send a message via actor.
				sendResult := ref.Ask(ctx, SendMailRequest{
					SenderID:       agents[senderIdx].ID,
					RecipientNames: []string{agents[recipientIdx].Name},
					Subject:        fmt.Sprintf("Concurrent %d", opIdx),
					Body:           "Test body",
					Priority:       PriorityNormal,
				}).Await(ctx)

				resp, err := sendResult.Unpack()
				if err != nil {
					errChan <- fmt.Errorf("op %d: actor error: %w", opIdx, err)
					return
				}

				sendResp := resp.(SendMailResponse)
				if sendResp.Error != nil {
					errChan <- fmt.Errorf("op %d: send error: %w", opIdx, sendResp.Error)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for errors.
		for err := range errChan {
			rt.Fatalf("concurrent operation failed: %v", err)
		}

		// PROPERTY: All agents have consistent state.
		for _, a := range agents {
			statusResult := ref.Ask(ctx, GetStatusRequest{
				AgentID: a.ID,
			}).Await(ctx)

			statusResp, err := statusResult.Unpack()
			if err != nil {
				rt.Fatalf("status fetch failed for %s: %v", a.Name, err)
			}

			status := statusResp.(GetStatusResponse)
			if status.Error != nil {
				rt.Fatalf("status error for %s: %v", a.Name, status.Error)
			}

			// Verify the agent name matches.
			if status.Status.AgentName != a.Name {
				rt.Fatalf("agent name mismatch: expected %s, got %s",
					a.Name, status.Status.AgentName)
			}
		}
	})
}

// TestMailActorProperty_StateTransitions verifies valid state transitions via
// actor.
func TestMailActorProperty_StateTransitions(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		storage, cleanup := testDB(t)
		defer cleanup()

		ctx := context.Background()

		mailActor := NewMailActor(ActorConfig{
			ID:          "state-property-actor",
			Store:       storage,
			MailboxSize: 10,
		})
		mailActor.Start()
		defer mailActor.Stop()

		ref := mailActor.Ref()

		// Create sender and recipient.
		sender, _ := storage.CreateAgent(ctx, store.CreateAgentParams{
			Name: "PropertySender",
		})
		recipient, _ := storage.CreateAgent(ctx, store.CreateAgentParams{
			Name: "PropertyRecipient",
		})

		// Send a message.
		sendResult := ref.Ask(ctx, SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: []string{"PropertyRecipient"},
			Subject:        "State property test",
			Body:           "Testing transitions",
			Priority:       PriorityNormal,
		}).Await(ctx)

		sendResp, _ := sendResult.Unpack()
		msgID := sendResp.(SendMailResponse).MessageID

		// Valid state transitions.
		validTransitions := map[string][]string{
			"unread":   {"read", "starred", "archived", "trash"},
			"read":     {"starred", "archived", "trash"},
			"starred":  {"read", "archived", "trash"},
			"archived": {"read"},
			"trash":    {"read"},
		}

		numTransitions := rapid.IntRange(1, 10).Draw(rt, "numTransitions")
		currentState := "unread"

		for i := 0; i < numTransitions; i++ {
			nextStates := validTransitions[currentState]
			if len(nextStates) == 0 {
				break
			}

			nextState := nextStates[rapid.IntRange(0, len(nextStates)-1).Draw(rt, "stateIdx")]

			// Apply transition via actor.
			updateResult := ref.Ask(ctx, UpdateStateRequest{
				AgentID:   recipient.ID,
				MessageID: msgID,
				NewState:  nextState,
			}).Await(ctx)

			updateResp, err := updateResult.Unpack()
			if err != nil {
				rt.Fatalf("actor error during transition from %s to %s: %v",
					currentState, nextState, err)
			}

			resp := updateResp.(UpdateStateResponse)
			if resp.Error != nil {
				rt.Fatalf("failed to transition from %s to %s: %v",
					currentState, nextState, resp.Error)
			}

			currentState = nextState
		}
	})
}
