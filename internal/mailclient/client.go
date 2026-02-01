// Package mailclient provides a unified actor-based client for mail operations.
// This client is used by both the REST API and gRPC servers to interact with
// the mail service actor, ensuring consistent behavior across all API surfaces.
package mailclient

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/actorutil"
	"github.com/roasbeef/subtrate/internal/mail"
)

// Client provides actor-based mail operations. It wraps a mail actor reference
// and provides type-safe methods for common mail operations.
type Client struct {
	mailRef mail.MailActorRef
}

// NewClient creates a new mail client wrapping the given actor reference.
func NewClient(mailRef mail.MailActorRef) *Client {
	return &Client{mailRef: mailRef}
}

// SendMail sends a mail message via the actor system.
func (c *Client) SendMail(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.SendMailResponse,
	](ctx, c.mailRef, req)
}

// FetchInbox fetches inbox messages via the actor system.
func (c *Client) FetchInbox(
	ctx context.Context, req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.FetchInboxResponse,
	](ctx, c.mailRef, req)
}

// ReadMessage reads a message and marks it as read via the actor system.
func (c *Client) ReadMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.ReadMessageResponse,
	](ctx, c.mailRef, mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// UpdateMessageState updates a message's state via the actor system.
func (c *Client) UpdateMessageState(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.UpdateStateResponse,
	](ctx, c.mailRef, mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    messageID,
		NewState:     newState,
		SnoozedUntil: snoozedUntil,
	})
}

// AckMessage acknowledges a message via the actor system.
func (c *Client) AckMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.AckMessageResponse,
	](ctx, c.mailRef, mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// GetAgentStatus gets an agent's mail status via the actor system.
func (c *Client) GetAgentStatus(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.GetStatusResponse,
	](ctx, c.mailRef, mail.GetStatusRequest{
		AgentID: agentID,
	})
}

// PollChanges polls for new messages since given offsets via the actor system.
func (c *Client) PollChanges(
	ctx context.Context, agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PollChangesResponse,
	](ctx, c.mailRef, mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	})
}

// Publish publishes a message to a topic via the actor system.
func (c *Client) Publish(
	ctx context.Context, req mail.PublishRequest,
) (mail.PublishResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PublishResponse,
	](ctx, c.mailRef, req)
}

// Ref returns the underlying mail actor reference. This is useful when direct
// actor access is needed for advanced use cases like streaming subscriptions.
func (c *Client) Ref() mail.MailActorRef {
	return c.mailRef
}
