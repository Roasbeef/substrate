package mail

import (
	"sync"
)

// NotificationRegistry manages subscriptions for real-time message notifications.
// This is used by the gRPC streaming endpoint to receive new messages without polling.
type NotificationRegistry struct {
	mu          sync.RWMutex
	subscribers map[int64][]chan InboxMessage // agentID -> list of channels
}

// NewNotificationRegistry creates a new notification registry.
func NewNotificationRegistry() *NotificationRegistry {
	return &NotificationRegistry{
		subscribers: make(map[int64][]chan InboxMessage),
	}
}

// Subscribe registers a channel to receive messages for the given agent.
// Returns an unsubscribe function that must be called when done.
func (r *NotificationRegistry) Subscribe(agentID int64, ch chan InboxMessage) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.subscribers[agentID] = append(r.subscribers[agentID], ch)

	// Return an unsubscribe function.
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		channels := r.subscribers[agentID]
		for i, c := range channels {
			if c == ch {
				// Remove this channel from the slice.
				r.subscribers[agentID] = append(channels[:i], channels[i+1:]...)
				close(ch)
				break
			}
		}

		// Clean up empty slices.
		if len(r.subscribers[agentID]) == 0 {
			delete(r.subscribers, agentID)
		}
	}
}

// Notify sends a message to all subscribers for the given agent.
// Non-blocking: if a subscriber's channel is full, the message is dropped.
func (r *NotificationRegistry) Notify(agentID int64, msg InboxMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ch := range r.subscribers[agentID] {
		select {
		case ch <- msg:
		default:
			// Channel full, skip to avoid blocking.
		}
	}
}

// NotifyAll sends a message to all subscribers for a list of agent IDs.
// Used when a message is sent to multiple recipients.
func (r *NotificationRegistry) NotifyAll(agentIDs []int64, msg InboxMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agentID := range agentIDs {
		for _, ch := range r.subscribers[agentID] {
			select {
			case ch <- msg:
			default:
				// Channel full, skip to avoid blocking.
			}
		}
	}
}

// SubscriberCount returns the number of active subscribers for an agent.
func (r *NotificationRegistry) SubscriberCount(agentID int64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subscribers[agentID])
}

// TotalSubscribers returns the total number of active subscriptions.
func (r *NotificationRegistry) TotalSubscribers() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, channels := range r.subscribers {
		total += len(channels)
	}
	return total
}
