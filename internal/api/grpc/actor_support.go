package subtraterpc

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
)

// sendMailActor sends a mail message via the shared mail client.
func (s *Server) sendMailActor(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	return s.mailClient.SendMail(ctx, req)
}

// fetchInboxActor fetches inbox messages via the shared mail client.
func (s *Server) fetchInboxActor(
	ctx context.Context, req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	return s.mailClient.FetchInbox(ctx, req)
}

// readMessageActor reads a message via the shared mail client and marks it as
// read.
func (s *Server) readMessageActor(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	return s.mailClient.ReadMessage(ctx, agentID, messageID)
}

// updateMessageStateActor updates a message's state via the shared mail client.
func (s *Server) updateMessageStateActor(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	return s.mailClient.UpdateMessageState(ctx, agentID, messageID, newState, snoozedUntil)
}

// ackMessageActor acknowledges a message via the shared mail client.
func (s *Server) ackMessageActor(
	ctx context.Context, agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	return s.mailClient.AckMessage(ctx, agentID, messageID)
}

// getAgentStatusActor gets an agent's mail status via the shared mail client.
func (s *Server) getAgentStatusActor(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	return s.mailClient.GetAgentStatus(ctx, agentID)
}

// pollChangesActor polls for new messages since given offsets via the shared
// mail client.
func (s *Server) pollChangesActor(
	ctx context.Context, agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	return s.mailClient.PollChanges(ctx, agentID, sinceOffsets)
}

// publishMessageActor publishes a message to a topic via the shared mail
// client.
func (s *Server) publishMessageActor(
	ctx context.Context, req mail.PublishRequest,
) (mail.PublishResponse, error) {
	return s.mailClient.Publish(ctx, req)
}
