package web

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/actorutil"
	"github.com/roasbeef/subtrate/internal/mail"
)

// ActorRefs holds references to the actor-based services.
type ActorRefs struct {
	// MailRef is the actor reference for the mail service.
	MailRef mail.MailActorRef

	// ActivityRef is the actor reference for the activity service.
	ActivityRef activity.ActivityActorRef
}

// HasActors returns true if actor refs are configured.
func (a *ActorRefs) HasActors() bool {
	return a != nil && a.MailRef != nil
}

// HasMailActor returns true if a mail actor ref is configured.
func (a *ActorRefs) HasMailActor() bool {
	return a != nil && a.MailRef != nil
}

// HasActivityActor returns true if an activity actor ref is configured.
func (a *ActorRefs) HasActivityActor() bool {
	return a != nil && a.ActivityRef != nil
}

// sendMail sends a mail message via the actor system.
func (s *Server) sendMail(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.SendMailResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.SendMailResponse,
	](ctx, s.actorRefs.MailRef, req)
}

// fetchInbox fetches inbox messages via the actor system.
func (s *Server) fetchInbox(
	ctx context.Context, req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.FetchInboxResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.FetchInboxResponse,
	](ctx, s.actorRefs.MailRef, req)
}

// readMessage reads a message via the actor system and marks it as read.
func (s *Server) readMessage(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.ReadMessageResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.ReadMessageResponse,
	](ctx, s.actorRefs.MailRef, mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// updateMessageState updates a message's state via the actor system.
func (s *Server) updateMessageState(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.UpdateStateResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.UpdateStateResponse,
	](ctx, s.actorRefs.MailRef, mail.UpdateStateRequest{
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

	if !s.actorRefs.HasActors() {
		return mail.AckMessageResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.AckMessageResponse,
	](ctx, s.actorRefs.MailRef, mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// getAgentStatus gets an agent's mail status via the actor system.
func (s *Server) getAgentStatus(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.GetStatusResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.GetStatusResponse,
	](ctx, s.actorRefs.MailRef, mail.GetStatusRequest{
		AgentID: agentID,
	})
}

// pollChanges polls for new messages via the actor system.
func (s *Server) pollChanges(
	ctx context.Context, agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.PollChangesResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PollChangesResponse,
	](ctx, s.actorRefs.MailRef, mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	})
}

// publishMessage publishes a message to a topic via the actor system.
func (s *Server) publishMessage(
	ctx context.Context, req mail.PublishRequest,
) (mail.PublishResponse, error) {

	if !s.actorRefs.HasActors() {
		return mail.PublishResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PublishResponse,
	](ctx, s.actorRefs.MailRef, req)
}

// recordActivity records an activity event via the actor system.
func (s *Server) recordActivity(ctx context.Context, req activity.RecordActivityRequest) {
	if s.actorRefs.HasActors() && s.actorRefs.ActivityRef != nil {
		// Fire and forget.
		s.actorRefs.ActivityRef.Tell(ctx, req)
	}
}

// listRecentActivities lists recent activities via the actor system.
func (s *Server) listRecentActivities(
	ctx context.Context, limit int,
) ([]activity.Activity, error) {

	if !s.actorRefs.HasActors() || s.actorRefs.ActivityRef == nil {
		return nil, nil
	}

	resp, err := actorutil.AskAwaitTyped[
		activity.ActivityRequest, activity.ActivityResponse,
		activity.ListRecentResponse,
	](ctx, s.actorRefs.ActivityRef, activity.ListRecentRequest{Limit: limit})
	if err != nil {
		return nil, err
	}

	return resp.Activities, resp.Error
}
