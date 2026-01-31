package activity

import (
	"context"
	"fmt"

	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/store"
)

// Service is the activity service actor behavior.
type Service struct {
	store store.ActivityStore
}

// ServiceConfig holds configuration for the activity service.
type ServiceConfig struct {
	// Store is the activity store implementation.
	Store store.ActivityStore
}

// NewService creates a new activity service with the given configuration.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		store: cfg.Store,
	}
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (s *Service) Receive(
	ctx context.Context, msg ActivityRequest,
) fn.Result[ActivityResponse] {

	switch m := msg.(type) {
	case RecordActivityRequest:
		resp := s.handleRecordActivity(ctx, m)
		return fn.Ok[ActivityResponse](resp)

	case ListRecentRequest:
		resp := s.handleListRecent(ctx, m)
		return fn.Ok[ActivityResponse](resp)

	case ListByAgentRequest:
		resp := s.handleListByAgent(ctx, m)
		return fn.Ok[ActivityResponse](resp)

	case ListSinceRequest:
		resp := s.handleListSince(ctx, m)
		return fn.Ok[ActivityResponse](resp)

	case CleanupRequest:
		resp := s.handleCleanup(ctx, m)
		return fn.Ok[ActivityResponse](resp)

	default:
		return fn.Err[ActivityResponse](fmt.Errorf(
			"unknown message type: %T", msg,
		))
	}
}

// handleRecordActivity processes a RecordActivityRequest.
func (s *Service) handleRecordActivity(
	ctx context.Context, req RecordActivityRequest,
) RecordActivityResponse {

	err := s.store.CreateActivity(ctx, store.CreateActivityParams{
		AgentID:      req.AgentID,
		ActivityType: req.ActivityType,
		Description:  req.Description,
		Metadata:     req.Metadata,
	})

	return RecordActivityResponse{Error: err}
}

// handleListRecent processes a ListRecentRequest.
func (s *Service) handleListRecent(
	ctx context.Context, req ListRecentRequest,
) ListRecentResponse {

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	activities, err := s.store.ListRecentActivities(ctx, limit)
	if err != nil {
		return ListRecentResponse{Error: err}
	}

	return ListRecentResponse{
		Activities: convertActivities(activities),
	}
}

// handleListByAgent processes a ListByAgentRequest.
func (s *Service) handleListByAgent(
	ctx context.Context, req ListByAgentRequest,
) ListByAgentResponse {

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	activities, err := s.store.ListActivitiesByAgent(ctx, req.AgentID, limit)
	if err != nil {
		return ListByAgentResponse{Error: err}
	}

	return ListByAgentResponse{
		Activities: convertActivities(activities),
	}
}

// handleListSince processes a ListSinceRequest.
func (s *Service) handleListSince(
	ctx context.Context, req ListSinceRequest,
) ListSinceResponse {

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	activities, err := s.store.ListActivitiesSince(ctx, req.Since, limit)
	if err != nil {
		return ListSinceResponse{Error: err}
	}

	return ListSinceResponse{
		Activities: convertActivities(activities),
	}
}

// handleCleanup processes a CleanupRequest.
func (s *Service) handleCleanup(
	ctx context.Context, req CleanupRequest,
) CleanupResponse {

	err := s.store.DeleteOldActivities(ctx, req.OlderThan)
	return CleanupResponse{Error: err}
}

// convertActivities converts store activities to service activities.
func convertActivities(storeActivities []store.Activity) []Activity {
	activities := make([]Activity, len(storeActivities))
	for i, a := range storeActivities {
		activities[i] = Activity{
			ID:           a.ID,
			AgentID:      a.AgentID,
			ActivityType: a.ActivityType,
			Description:  a.Description,
			Metadata:     a.Metadata,
			CreatedAt:    a.CreatedAt,
		}
	}
	return activities
}

// ActivityActorRef is the typed actor reference for the activity service.
type ActivityActorRef = actor.ActorRef[ActivityRequest, ActivityResponse]

// NewActivityActor creates a new activity actor with the given configuration.
func NewActivityActor(cfg ServiceConfig) *actor.Actor[ActivityRequest, ActivityResponse] {
	svc := NewService(cfg)
	return actor.NewActor(actor.ActorConfig[ActivityRequest, ActivityResponse]{
		ID:          "activity-service",
		Behavior:    svc,
		MailboxSize: 100,
	})
}

// Ensure Service implements ActorBehavior.
var _ actor.ActorBehavior[ActivityRequest, ActivityResponse] = (*Service)(nil)
