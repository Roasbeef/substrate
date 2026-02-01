package subtraterpc

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/actorutil"
	"github.com/roasbeef/subtrate/internal/mail"
)

// ActorRefs holds optional actor references for the gRPC server.
type ActorRefs struct {
	// MailRef is the actor reference for mail operations.
	MailRef mail.MailActorRef

	// ActivityRef is the actor reference for activity operations.
	ActivityRef activity.ActivityActorRef
}

// HasMailActor returns true if a mail actor ref is configured.
func (a *ActorRefs) HasMailActor() bool {
	return a != nil && a.MailRef != nil
}

// HasActivityActor returns true if an activity actor ref is configured.
func (a *ActorRefs) HasActivityActor() bool {
	return a != nil && a.ActivityRef != nil
}

// sendMailActor sends a mail message via the actor system.
func (s *Server) sendMailActor(
	ctx context.Context, req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	if s.mailRef == nil {
		return mail.SendMailResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.SendMailResponse,
	](ctx, s.mailRef, req)
}

// fetchInboxActor fetches inbox messages via the actor system.
func (s *Server) fetchInboxActor(
	ctx context.Context, req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	if s.mailRef == nil {
		return mail.FetchInboxResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.FetchInboxResponse,
	](ctx, s.mailRef, req)
}

// readMessageActor reads a message via the actor system and marks it as read.
func (s *Server) readMessageActor(
	ctx context.Context, agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	if s.mailRef == nil {
		return mail.ReadMessageResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.ReadMessageResponse,
	](ctx, s.mailRef, mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// updateMessageStateActor updates a message's state via the actor system.
func (s *Server) updateMessageStateActor(
	ctx context.Context, agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	if s.mailRef == nil {
		return mail.UpdateStateResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.UpdateStateResponse,
	](ctx, s.mailRef, mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    messageID,
		NewState:     newState,
		SnoozedUntil: snoozedUntil,
	})
}

// ackMessageActor acknowledges a message via the actor system.
func (s *Server) ackMessageActor(
	ctx context.Context, agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	if s.mailRef == nil {
		return mail.AckMessageResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.AckMessageResponse,
	](ctx, s.mailRef, mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
}

// getAgentStatusActor gets an agent's mail status via the actor system.
func (s *Server) getAgentStatusActor(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	if s.mailRef == nil {
		return mail.GetStatusResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.GetStatusResponse,
	](ctx, s.mailRef, mail.GetStatusRequest{
		AgentID: agentID,
	})
}

// pollChangesActor polls for new messages via the actor system.
func (s *Server) pollChangesActor(
	ctx context.Context, agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	if s.mailRef == nil {
		return mail.PollChangesResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PollChangesResponse,
	](ctx, s.mailRef, mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	})
}

// publishMessageActor publishes a message to a topic via the actor system.
func (s *Server) publishMessageActor(
	ctx context.Context, req mail.PublishRequest,
) (mail.PublishResponse, error) {
	if s.mailRef == nil {
		return mail.PublishResponse{}, nil
	}

	return actorutil.AskAwaitTyped[
		mail.MailRequest, mail.MailResponse, mail.PublishResponse,
	](ctx, s.mailRef, req)
}

// recordActivity records an activity event via the actor system.
func (s *Server) recordActivity(ctx context.Context, req activity.RecordActivityRequest) {
	if s.actorRefs != nil && s.actorRefs.ActivityRef != nil {
		// Fire and forget.
		s.actorRefs.ActivityRef.Tell(ctx, req)
	}
}
