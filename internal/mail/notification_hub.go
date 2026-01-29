package mail

import (
	"context"

	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/lightningnetwork/lnd/fn/v2"
)

// NotificationHubKey is the service key for the notification hub actor.
var NotificationHubKey = actor.NewServiceKey[NotificationRequest, NotificationResponse](
	"notification-hub",
)

// NotificationHub is the actor that manages notification subscriptions and
// delivers messages to subscribers. It provides a fully actor-based alternative
// to the channel-based NotificationRegistry.
//
// The hub maintains a map of agent IDs to their subscribers. When a message is
// sent to an agent, all subscribers for that agent receive the message via
// their registered delivery channel.
//
// This design follows the actor model:
//   - All state mutations happen within the actor's Receive method.
//   - External code communicates via Tell (fire-and-forget) or Ask (request-response).
//   - No mutexes are needed since messages are processed serially.
type NotificationHub struct {
	// agentSubscribers maps agent IDs to their subscribers.
	agentSubscribers map[int64][]subscriber

	// topicSubscribers maps topic IDs to their subscribers.
	topicSubscribers map[int64][]subscriber
}

// NewNotificationHub creates a new notification hub actor.
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		agentSubscribers: make(map[int64][]subscriber),
		topicSubscribers: make(map[int64][]subscriber),
	}
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (n *NotificationHub) Receive(ctx context.Context,
	msg NotificationRequest) fn.Result[NotificationResponse] {

	switch m := msg.(type) {
	case SubscribeAgentMsg:
		resp := n.handleSubscribeAgent(m)
		return fn.Ok[NotificationResponse](resp)

	case UnsubscribeAgentMsg:
		resp := n.handleUnsubscribeAgent(m)
		return fn.Ok[NotificationResponse](resp)

	case NotifyAgentMsg:
		resp := n.handleNotifyAgent(m)
		return fn.Ok[NotificationResponse](resp)

	case NotifyTopicMsg:
		resp := n.handleNotifyTopic(m)
		return fn.Ok[NotificationResponse](resp)

	default:
		return fn.Err[NotificationResponse](
			ErrUnknownRequestType,
		)
	}
}

// handleSubscribeAgent adds a subscriber for an agent.
func (n *NotificationHub) handleSubscribeAgent(
	msg SubscribeAgentMsg) SubscribeAgentResponse {

	// Check if this subscriber already exists.
	subs := n.agentSubscribers[msg.AgentID]
	for _, s := range subs {
		if s.id == msg.SubscriberID {
			// Already subscribed, just return success.
			return SubscribeAgentResponse{Success: true}
		}
	}

	// Add the new subscriber.
	n.agentSubscribers[msg.AgentID] = append(subs, subscriber{
		id:           msg.SubscriberID,
		deliveryChan: msg.DeliveryChan,
	})

	return SubscribeAgentResponse{Success: true}
}

// handleUnsubscribeAgent removes a subscriber for an agent.
func (n *NotificationHub) handleUnsubscribeAgent(
	msg UnsubscribeAgentMsg) UnsubscribeAgentResponse {

	subs := n.agentSubscribers[msg.AgentID]
	for i, s := range subs {
		if s.id == msg.SubscriberID {
			// Remove this subscriber by replacing with last and truncating.
			n.agentSubscribers[msg.AgentID] = append(
				subs[:i], subs[i+1:]...,
			)

			// Clean up empty slices.
			if len(n.agentSubscribers[msg.AgentID]) == 0 {
				delete(n.agentSubscribers, msg.AgentID)
			}

			return UnsubscribeAgentResponse{Success: true}
		}
	}

	// Subscriber not found, still return success (idempotent).
	return UnsubscribeAgentResponse{Success: true}
}

// handleNotifyAgent sends a message to all subscribers for an agent.
func (n *NotificationHub) handleNotifyAgent(
	msg NotifyAgentMsg) NotifyAgentResponse {

	deliveredCount := 0
	for _, s := range n.agentSubscribers[msg.AgentID] {
		// Non-blocking send. If the channel is full, we skip.
		select {
		case s.deliveryChan <- msg.Message:
			deliveredCount++
		default:
			// Channel full, skip to avoid blocking the actor.
		}
	}

	return NotifyAgentResponse{DeliveredCount: deliveredCount}
}

// handleNotifyTopic sends a message to all subscribers for the given agents.
func (n *NotificationHub) handleNotifyTopic(
	msg NotifyTopicMsg) NotifyTopicResponse {

	deliveredCount := 0
	for _, agentID := range msg.AgentIDs {
		for _, s := range n.agentSubscribers[agentID] {
			// Non-blocking send. If the channel is full, we skip.
			select {
			case s.deliveryChan <- msg.Message:
				deliveredCount++
			default:
				// Channel full, skip to avoid blocking the actor.
			}
		}
	}

	return NotifyTopicResponse{DeliveredCount: deliveredCount}
}

// SubscriberCount returns the number of active subscribers for an agent.
// This is a convenience method for testing; in production, use Ask with a
// dedicated message type.
func (n *NotificationHub) SubscriberCount(agentID int64) int {
	return len(n.agentSubscribers[agentID])
}

// TotalSubscribers returns the total number of active subscriptions.
// This is a convenience method for testing; in production, use Ask with a
// dedicated message type.
func (n *NotificationHub) TotalSubscribers() int {
	total := 0
	for _, subs := range n.agentSubscribers {
		total += len(subs)
	}
	return total
}
