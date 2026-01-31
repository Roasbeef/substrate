package web

import (
	"context"

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

// sendMail sends a mail message via the actor system.
func (s *Server) sendMail(ctx context.Context, req mail.SendMailRequest) (mail.SendMailResponse, error) {
	if s.actorRefs.HasActors() {
		return actorutil.AskAwaitTyped[mail.MailRequest, mail.MailResponse, mail.SendMailResponse](
			ctx, s.actorRefs.MailRef, req,
		)
	}
	// Fallback to direct service call (for backward compatibility).
	return mail.SendMailResponse{}, nil
}

// fetchInbox fetches inbox messages via the actor system.
func (s *Server) fetchInbox(ctx context.Context, req mail.FetchInboxRequest) (mail.FetchInboxResponse, error) {
	if s.actorRefs.HasActors() {
		return actorutil.AskAwaitTyped[mail.MailRequest, mail.MailResponse, mail.FetchInboxResponse](
			ctx, s.actorRefs.MailRef, req,
		)
	}
	return mail.FetchInboxResponse{}, nil
}

// recordActivity records an activity event via the actor system.
func (s *Server) recordActivity(ctx context.Context, req activity.RecordActivityRequest) {
	if s.actorRefs.HasActors() && s.actorRefs.ActivityRef != nil {
		// Fire and forget.
		s.actorRefs.ActivityRef.Tell(ctx, req)
	}
}

// listRecentActivities lists recent activities via the actor system.
func (s *Server) listRecentActivities(ctx context.Context, limit int) ([]activity.Activity, error) {
	if s.actorRefs.HasActors() && s.actorRefs.ActivityRef != nil {
		resp, err := actorutil.AskAwaitTyped[activity.ActivityRequest, activity.ActivityResponse, activity.ListRecentResponse](
			ctx, s.actorRefs.ActivityRef, activity.ListRecentRequest{Limit: limit},
		)
		if err != nil {
			return nil, err
		}
		return resp.Activities, resp.Error
	}
	return nil, nil
}
