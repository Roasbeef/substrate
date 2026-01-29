# Actor-Based Notification System

## Current Implementation

The current `NotificationRegistry` uses raw Go channels:
```go
type NotificationRegistry struct {
    mu          sync.RWMutex
    subscribers map[int64][]chan InboxMessage
}
```

This works but is inconsistent with the rest of the actor-based architecture.

## Proposed Actor-Based Design

### Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                    NotificationHubActor                       │
│  ServiceKey: "notification-hub"                               │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  agentSubscribers: map[int64][]actor.Ref                │ │
│  │  topicSubscribers: map[int64][]actor.Ref                │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
         │                        │
         │ NotifyAgentMsg         │ NotifyTopicMsg
         ▼                        ▼
┌─────────────────┐     ┌─────────────────┐
│ SubscriberActor │     │ SubscriberActor │
│ (gRPC stream)   │     │ (WebSocket)     │
└─────────────────┘     └─────────────────┘
```

### Message Types

```go
// SubscribeAgentMsg registers a subscriber for an agent's messages.
type SubscribeAgentMsg struct {
    actor.BaseMessage
    AgentID       int64
    SubscriberRef actor.Ref
}

// UnsubscribeAgentMsg removes a subscriber.
type UnsubscribeAgentMsg struct {
    actor.BaseMessage
    AgentID       int64
    SubscriberRef actor.Ref
}

// SubscribeTopicMsg registers a subscriber for a topic.
type SubscribeTopicMsg struct {
    actor.BaseMessage
    TopicID       int64
    SubscriberRef actor.Ref
}

// NotifyAgentMsg notifies all subscribers for an agent.
type NotifyAgentMsg struct {
    actor.BaseMessage
    AgentID int64
    Message InboxMessage
}

// NotifyTopicMsg notifies all subscribers for a topic.
type NotifyTopicMsg struct {
    actor.BaseMessage
    TopicID  int64
    AgentIDs []int64  // Recipients to notify
    Message  InboxMessage
}

// DeliverNotificationMsg is sent to individual subscribers.
type DeliverNotificationMsg struct {
    actor.BaseMessage
    Message InboxMessage
}
```

### NotificationHubActor Implementation

```go
type NotificationHubActor struct {
    agentSubscribers map[int64][]actor.Ref
    topicSubscribers map[int64][]actor.Ref
}

func (n *NotificationHubActor) Receive(ctx actor.Context) {
    switch msg := ctx.Message().(type) {
    case SubscribeAgentMsg:
        n.agentSubscribers[msg.AgentID] = append(
            n.agentSubscribers[msg.AgentID], msg.SubscriberRef,
        )

    case UnsubscribeAgentMsg:
        // Remove from slice
        n.removeAgentSubscriber(msg.AgentID, msg.SubscriberRef)

    case NotifyAgentMsg:
        for _, ref := range n.agentSubscribers[msg.AgentID] {
            // Fire-and-forget notification delivery
            ctx.Send(ref, DeliverNotificationMsg{Message: msg.Message})
        }

    case NotifyTopicMsg:
        // Notify all recipients who have active subscriptions
        for _, agentID := range msg.AgentIDs {
            for _, ref := range n.agentSubscribers[agentID] {
                ctx.Send(ref, DeliverNotificationMsg{Message: msg.Message})
            }
        }
    }
}
```

### gRPC Stream Subscriber

For the `SubscribeInbox` RPC, we spawn a temporary actor that bridges to the gRPC stream:

```go
type StreamSubscriberActor struct {
    stream  Mail_SubscribeInboxServer
    agentID int64
    hubRef  actor.Ref
}

func (s *StreamSubscriberActor) Receive(ctx actor.Context) {
    switch msg := ctx.Message().(type) {
    case actor.Started:
        // Register with hub
        ctx.Send(s.hubRef, SubscribeAgentMsg{
            AgentID:       s.agentID,
            SubscriberRef: ctx.Self(),
        })

    case actor.Stopped:
        // Unregister from hub
        ctx.Send(s.hubRef, UnsubscribeAgentMsg{
            AgentID:       s.agentID,
            SubscriberRef: ctx.Self(),
        })

    case DeliverNotificationMsg:
        // Send to gRPC stream
        if err := s.stream.Send(convertMessage(&msg.Message)); err != nil {
            // Stream broken, stop actor
            ctx.Stop(ctx.Self())
        }
    }
}
```

### Integration with Mail Service

When a message is sent, the MailServiceActor notifies the hub:

```go
// In MailServiceActor after successful message delivery:
ctx.Send(notificationHubRef, NotifyAgentMsg{
    AgentID: recipientID,
    Message: inboxMsg,
})

// For topic publishes:
ctx.Send(notificationHubRef, NotifyTopicMsg{
    TopicID:  topicID,
    AgentIDs: subscriberIDs,
    Message:  inboxMsg,
})
```

## Benefits

1. **Unified Message Passing**: No mixing of channels and actor messages
2. **Lifecycle Management**: Actor system handles subscriber cleanup on disconnect
3. **Supervision**: Hub actor can be supervised for fault tolerance
4. **Backpressure**: Actor mailboxes provide natural backpressure
5. **Extensibility**: Easy to add WebSocket subscribers, webhook deliveries, etc.

## Per-Topic Actors (Optional Enhancement)

For high-traffic topics, spawn dedicated notifier actors:

```go
// TopicNotifierActor handles notifications for a single topic
type TopicNotifierActor struct {
    topicID     int64
    subscribers []actor.Ref
}

// ServiceKey allows lookup by topic
func (t *TopicNotifierActor) ServiceKey() string {
    return fmt.Sprintf("topic-notifier/%d", t.topicID)
}
```

This distributes notification load across actors rather than bottlenecking on a single hub.

## Migration Path

1. Keep `NotificationRegistry` working for now
2. Implement `NotificationHubActor` alongside it
3. Add feature flag to switch between implementations
4. Remove channel-based registry once actor version is stable
