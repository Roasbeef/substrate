package web

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/mail"
)

// sendMail sends a mail message via the shared mail client.
func (s *Server) sendMail(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	return s.mailClient.SendMail(ctx, req)
}

// fetchInbox fetches inbox messages via the shared mail client.
func (s *Server) fetchInbox(
	ctx context.Context, req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	return s.mailClient.FetchInbox(ctx, req)
}

// readMessage reads a message via the shared mail client and marks it as read.
func (s *Server) readMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	return s.mailClient.ReadMessage(ctx, agentID, messageID)
}

// updateMessageState updates a message's state via the shared mail client.
func (s *Server) updateMessageState(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	return s.mailClient.UpdateMessageState(ctx, agentID, messageID, newState, snoozedUntil)
}

// ackMessage acknowledges a message via the shared mail client.
func (s *Server) ackMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	return s.mailClient.AckMessage(ctx, agentID, messageID)
}

// getAgentStatus gets an agent's mail status via the shared mail client.
func (s *Server) getAgentStatus(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	return s.mailClient.GetAgentStatus(ctx, agentID)
}

// pollChanges polls for new messages via the shared mail client.
func (s *Server) pollChanges(
	ctx context.Context, agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	return s.mailClient.PollChanges(ctx, agentID, sinceOffsets)
}

// publishMessage publishes a message to a topic via the shared mail client.
func (s *Server) publishMessage(
	ctx context.Context, req mail.PublishRequest,
) (mail.PublishResponse, error) {
	return s.mailClient.Publish(ctx, req)
}

// recordActivity records an activity event via the shared activity client.
func (s *Server) recordActivity(ctx context.Context, req activity.RecordActivityRequest) {
	s.actClient.RecordActivity(ctx, req)
}

// listRecentActivities lists recent activities via the shared activity client.
func (s *Server) listRecentActivities(
	ctx context.Context, limit int,
) ([]activity.Activity, error) {
	return s.actClient.ListRecentActivities(ctx, limit)
}
