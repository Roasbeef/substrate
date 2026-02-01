package mailclient

import (
	"context"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/actorutil"
)

// ActivityClient provides actor-based activity operations.
type ActivityClient struct {
	activityRef activity.ActivityActorRef
}

// NewActivityClient creates a new activity client wrapping the given actor
// reference.
func NewActivityClient(activityRef activity.ActivityActorRef) *ActivityClient {
	return &ActivityClient{activityRef: activityRef}
}

// RecordActivity records an activity event via the actor system. This uses
// fire-and-forget semantics (Tell) since activity recording is non-critical.
func (c *ActivityClient) RecordActivity(
	ctx context.Context, req activity.RecordActivityRequest,
) {
	c.activityRef.Tell(ctx, req)
}

// RecordActivitySync records an activity event and waits for confirmation.
// Use this when you need to ensure the activity was recorded.
func (c *ActivityClient) RecordActivitySync(
	ctx context.Context, req activity.RecordActivityRequest,
) error {
	resp, err := actorutil.AskAwaitTyped[
		activity.ActivityRequest, activity.ActivityResponse,
		activity.RecordActivityResponse,
	](ctx, c.activityRef, req)
	if err != nil {
		return err
	}

	return resp.Error
}

// ListRecentActivities lists recent activities via the actor system.
func (c *ActivityClient) ListRecentActivities(
	ctx context.Context, limit int,
) ([]activity.Activity, error) {
	resp, err := actorutil.AskAwaitTyped[
		activity.ActivityRequest, activity.ActivityResponse,
		activity.ListRecentResponse,
	](ctx, c.activityRef, activity.ListRecentRequest{Limit: limit})
	if err != nil {
		return nil, err
	}

	return resp.Activities, resp.Error
}

// ListActivitiesByAgent lists activities for a specific agent.
func (c *ActivityClient) ListActivitiesByAgent(
	ctx context.Context, agentID int64, limit int,
) ([]activity.Activity, error) {
	resp, err := actorutil.AskAwaitTyped[
		activity.ActivityRequest, activity.ActivityResponse,
		activity.ListByAgentResponse,
	](ctx, c.activityRef, activity.ListByAgentRequest{
		AgentID: agentID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}

	return resp.Activities, resp.Error
}

// Ref returns the underlying activity actor reference.
func (c *ActivityClient) Ref() activity.ActivityActorRef {
	return c.activityRef
}
