package web

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/actorutil"
	"github.com/roasbeef/subtrate/internal/mail"
)

// sendMail sends a mail message via the actor system.
func (s *Server) sendMail(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.SendMailResponse,
	](ctx, s.mailRef, req)
}

// readMessage reads a message via the actor system and marks it as read.
func (s *Server) readMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.ReadMessageResponse,
	](ctx, s.mailRef, mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// updateMessageState updates a message's state via the actor system.
func (s *Server) updateMessageState(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.UpdateStateResponse,
	](ctx, s.mailRef, mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    messageID,
		NewState:     newState,
		SnoozedUntil: snoozedUntil,
	})
}

// ackMessage acknowledges a message via the actor system.
func (s *Server) ackMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.AckMessageResponse,
	](ctx, s.mailRef, mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// getAgentStatus gets an agent's mail status via the actor system.
func (s *Server) getAgentStatus(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.GetStatusResponse,
	](ctx, s.mailRef, mail.GetStatusRequest{
		AgentID: agentID,
	})
}

// recordActivity records an activity event via the actor system.
func (s *Server) recordActivity(ctx context.Context, req activity.RecordActivityRequest) {
	// Fire and forget.
	s.activityRef.Tell(ctx, req)
}

// listRecentActivities lists recent activities via the actor system.
func (s *Server) listRecentActivities(
	ctx context.Context, limit int,
) ([]activity.Activity, error) {
	resp, err := actorutil.AskAwaitTyped[
		activity.ActivityRequest, activity.ActivityResponse,
		activity.ListRecentResponse,
	](ctx, s.activityRef, activity.ListRecentRequest{Limit: limit})
	if err != nil {
		return nil, err
	}

	return resp.Activities, resp.Error
}
