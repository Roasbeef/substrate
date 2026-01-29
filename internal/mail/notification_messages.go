package mail

import "github.com/lightninglabs/darepo-client/baselib/actor"

// NotificationRequest is the union type for all notification hub requests.
type NotificationRequest interface {
	actor.Message
	isNotificationRequest()
}

// NotificationResponse is the union type for all notification hub responses.
type NotificationResponse interface {
	isNotificationResponse()
}

// Ensure all request types implement NotificationRequest.
func (SubscribeAgentMsg) isNotificationRequest()   {}
func (UnsubscribeAgentMsg) isNotificationRequest() {}
func (NotifyAgentMsg) isNotificationRequest()      {}
func (NotifyTopicMsg) isNotificationRequest()      {}

// Ensure all response types implement NotificationResponse.
func (SubscribeAgentResponse) isNotificationResponse()   {}
func (UnsubscribeAgentResponse) isNotificationResponse() {}
func (NotifyAgentResponse) isNotificationResponse()      {}
func (NotifyTopicResponse) isNotificationResponse()      {}

// SubscribeAgentMsg registers a subscriber for an agent's messages.
type SubscribeAgentMsg struct {
	actor.BaseMessage

	// AgentID is the agent to subscribe to.
	AgentID int64

	// SubscriberID is a unique identifier for this subscriber.
	SubscriberID string

	// DeliveryChan is the channel to send notifications to.
	// The hub will send InboxMessage values to this channel.
	DeliveryChan chan<- InboxMessage
}

// MessageType implements actor.Message.
func (SubscribeAgentMsg) MessageType() string { return "SubscribeAgentMsg" }

// SubscribeAgentResponse is the response to SubscribeAgentMsg.
type SubscribeAgentResponse struct {
	Success bool
}

// UnsubscribeAgentMsg removes a subscriber.
type UnsubscribeAgentMsg struct {
	actor.BaseMessage

	// AgentID is the agent to unsubscribe from.
	AgentID int64

	// SubscriberID identifies which subscriber to remove.
	SubscriberID string
}

// MessageType implements actor.Message.
func (UnsubscribeAgentMsg) MessageType() string { return "UnsubscribeAgentMsg" }

// UnsubscribeAgentResponse is the response to UnsubscribeAgentMsg.
type UnsubscribeAgentResponse struct {
	Success bool
}

// NotifyAgentMsg notifies all subscribers for an agent.
type NotifyAgentMsg struct {
	actor.BaseMessage

	// AgentID is the agent whose subscribers should be notified.
	AgentID int64

	// Message is the inbox message to deliver.
	Message InboxMessage
}

// MessageType implements actor.Message.
func (NotifyAgentMsg) MessageType() string { return "NotifyAgentMsg" }

// NotifyAgentResponse is the response to NotifyAgentMsg.
type NotifyAgentResponse struct {
	// DeliveredCount is the number of subscribers that received the message.
	DeliveredCount int
}

// NotifyTopicMsg notifies all subscribers for a topic.
type NotifyTopicMsg struct {
	actor.BaseMessage

	// TopicID is the topic whose subscribers should be notified.
	TopicID int64

	// AgentIDs are the recipient agents to notify.
	AgentIDs []int64

	// Message is the inbox message to deliver.
	Message InboxMessage
}

// MessageType implements actor.Message.
func (NotifyTopicMsg) MessageType() string { return "NotifyTopicMsg" }

// NotifyTopicResponse is the response to NotifyTopicMsg.
type NotifyTopicResponse struct {
	// DeliveredCount is the number of subscribers that received the message.
	DeliveredCount int
}

// subscriber holds information about a single subscription.
type subscriber struct {
	id           string
	deliveryChan chan<- InboxMessage
}
