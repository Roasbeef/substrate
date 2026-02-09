package store

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// TestCreatePlanReview verifies plan review creation and retrieval.
func TestCreatePlanReview(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-001",
		ThreadID:     "thread-001",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Test Plan",
		PlanSummary:  "A test plan summary.",
		SessionID:    "session-001",
	})
	require.NoError(t, err)
	require.Equal(t, "pr-001", review.PlanReviewID)
	require.Equal(t, "pending", review.State)
	require.Equal(t, "Test Plan", review.PlanTitle)
	require.Equal(t, "A test plan summary.", review.PlanSummary)
	require.Equal(t, "User", review.ReviewerName)
	require.Equal(t, agentID, review.RequesterID)
	require.Equal(t, "session-001", review.SessionID)
	require.Nil(t, review.MessageID)
	require.Nil(t, review.ReviewedBy)
	require.Nil(t, review.ReviewedAt)
	require.NotZero(t, review.CreatedAt)
	require.NotZero(t, review.UpdatedAt)
}

// TestCreatePlanReviewWithMessageID verifies plan review with a linked message.
func TestCreatePlanReviewWithMessageID(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	// Create a real message to satisfy the FK constraint.
	topic, err := store.CreateTopic(ctx, CreateTopicParams{
		Name:      "inbox:test-agent",
		TopicType: "direct",
	})
	require.NoError(t, err)

	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID: "thread-002",
		TopicID:  topic.ID,
		SenderID: agentID,
		Subject:  "[PLAN] Test",
		Body:     "plan body",
		Priority: "normal",
	})
	require.NoError(t, err)

	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-002",
		MessageID:    &msg.ID,
		ThreadID:     "thread-002",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Plan With Message",
	})
	require.NoError(t, err)
	require.NotNil(t, review.MessageID)
	require.Equal(t, msg.ID, *review.MessageID)

	// Verify GetPlanReviewByMessage works.
	found, err := store.GetPlanReviewByMessage(ctx, msg.ID)
	require.NoError(t, err)
	require.Equal(t, "pr-002", found.PlanReviewID)
}

// TestGetPlanReview verifies retrieval by plan_review_id.
func TestGetPlanReview(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-get-001",
		ThreadID:     "thread-get",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Get Test",
	})
	require.NoError(t, err)

	review, err := store.GetPlanReview(ctx, "pr-get-001")
	require.NoError(t, err)
	require.Equal(t, "pr-get-001", review.PlanReviewID)
	require.Equal(t, "Get Test", review.PlanTitle)

	// Not found case.
	_, err = store.GetPlanReview(ctx, "nonexistent")
	require.Error(t, err)
}

// TestGetPlanReviewByThread verifies retrieval by thread_id returns the latest.
func TestGetPlanReviewByThread(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	// Create two reviews for the same thread.
	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-thread-001",
		ThreadID:     "shared-thread",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan1.md",
		PlanTitle:    "First Plan",
	})
	require.NoError(t, err)

	_, err = store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-thread-002",
		ThreadID:     "shared-thread",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan2.md",
		PlanTitle:    "Second Plan",
	})
	require.NoError(t, err)

	// Should return the latest one.
	review, err := store.GetPlanReviewByThread(ctx, "shared-thread")
	require.NoError(t, err)
	require.Equal(t, "pr-thread-002", review.PlanReviewID)
}

// TestGetPlanReviewBySession verifies retrieval of pending review by session.
func TestGetPlanReviewBySession(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-session-001",
		ThreadID:     "thread-s1",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Session Plan",
		SessionID:    "session-abc",
	})
	require.NoError(t, err)

	review, err := store.GetPlanReviewBySession(ctx, "session-abc")
	require.NoError(t, err)
	require.Equal(t, "pr-session-001", review.PlanReviewID)
	require.Equal(t, "pending", review.State)

	// After approving, session lookup should fail (only pending returned).
	err = store.UpdatePlanReviewState(ctx, UpdatePlanReviewStateParams{
		PlanReviewID: "pr-session-001",
		State:        "approved",
	})
	require.NoError(t, err)

	_, err = store.GetPlanReviewBySession(ctx, "session-abc")
	require.Error(t, err)
}

// TestUpdatePlanReviewState verifies state transitions.
func TestUpdatePlanReviewState(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")
	reviewerID := createTestAgent(t, store, "reviewer")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-state-001",
		ThreadID:     "thread-state",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "State Plan",
	})
	require.NoError(t, err)

	// Transition to approved.
	err = store.UpdatePlanReviewState(ctx, UpdatePlanReviewStateParams{
		PlanReviewID:    "pr-state-001",
		State:           "approved",
		ReviewerComment: "Looks good!",
		ReviewedBy:      &reviewerID,
	})
	require.NoError(t, err)

	review, err := store.GetPlanReview(ctx, "pr-state-001")
	require.NoError(t, err)
	require.Equal(t, "approved", review.State)
	require.Equal(t, "Looks good!", review.ReviewerComment)
	require.NotNil(t, review.ReviewedBy)
	require.Equal(t, reviewerID, *review.ReviewedBy)
	require.NotNil(t, review.ReviewedAt)
}

// TestUpdatePlanReviewStateRejected verifies rejection state transition.
func TestUpdatePlanReviewStateRejected(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-reject-001",
		ThreadID:     "thread-reject",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Rejected Plan",
	})
	require.NoError(t, err)

	err = store.UpdatePlanReviewState(ctx, UpdatePlanReviewStateParams{
		PlanReviewID:    "pr-reject-001",
		State:           "rejected",
		ReviewerComment: "Too complex.",
	})
	require.NoError(t, err)

	review, err := store.GetPlanReview(ctx, "pr-reject-001")
	require.NoError(t, err)
	require.Equal(t, "rejected", review.State)
	require.Equal(t, "Too complex.", review.ReviewerComment)
}

// TestUpdatePlanReviewStateChangesRequested verifies changes_requested
// transition.
func TestUpdatePlanReviewStateChangesRequested(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-changes-001",
		ThreadID:     "thread-changes",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Changes Plan",
	})
	require.NoError(t, err)

	err = store.UpdatePlanReviewState(ctx, UpdatePlanReviewStateParams{
		PlanReviewID:    "pr-changes-001",
		State:           "changes_requested",
		ReviewerComment: "Please add error handling.",
	})
	require.NoError(t, err)

	review, err := store.GetPlanReview(ctx, "pr-changes-001")
	require.NoError(t, err)
	require.Equal(t, "changes_requested", review.State)
}

// TestListPlanReviews verifies paginated listing.
func TestListPlanReviews(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	for i := 0; i < 5; i++ {
		_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
			PlanReviewID: "pr-list-" + string(rune('a'+i)),
			ThreadID:     "thread-list-" + string(rune('a'+i)),
			RequesterID:  agentID,
			ReviewerName: "User",
			PlanPath:     "/tmp/plan.md",
			PlanTitle:    "List Plan " + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	// Get first page.
	reviews, err := store.ListPlanReviews(ctx, 3, 0)
	require.NoError(t, err)
	require.Len(t, reviews, 3)

	// Get second page.
	reviews, err = store.ListPlanReviews(ctx, 3, 3)
	require.NoError(t, err)
	require.Len(t, reviews, 2)
}

// TestListPlanReviewsByState verifies filtering by state.
func TestListPlanReviewsByState(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	// Create 3 reviews, approve one.
	for i := 0; i < 3; i++ {
		_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
			PlanReviewID: "pr-bystate-" + string(rune('a'+i)),
			ThreadID:     "thread-bystate-" + string(rune('a'+i)),
			RequesterID:  agentID,
			ReviewerName: "User",
			PlanPath:     "/tmp/plan.md",
			PlanTitle:    "State Plan " + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	err := store.UpdatePlanReviewState(ctx, UpdatePlanReviewStateParams{
		PlanReviewID: "pr-bystate-a",
		State:        "approved",
	})
	require.NoError(t, err)

	pending, err := store.ListPlanReviewsByState(ctx, "pending", 10)
	require.NoError(t, err)
	require.Len(t, pending, 2)

	approved, err := store.ListPlanReviewsByState(ctx, "approved", 10)
	require.NoError(t, err)
	require.Len(t, approved, 1)
}

// TestListPlanReviewsByRequester verifies filtering by requester.
func TestListPlanReviewsByRequester(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agent1 := createTestAgent(t, store, "agent-1")
	agent2 := createTestAgent(t, store, "agent-2")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-req-001",
		ThreadID:     "thread-req-1",
		RequesterID:  agent1,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Agent 1 Plan",
	})
	require.NoError(t, err)

	_, err = store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-req-002",
		ThreadID:     "thread-req-2",
		RequesterID:  agent2,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Agent 2 Plan",
	})
	require.NoError(t, err)

	reviews, err := store.ListPlanReviewsByRequester(ctx, agent1, 10)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	require.Equal(t, "pr-req-001", reviews[0].PlanReviewID)
}

// TestDeletePlanReview verifies deletion.
func TestDeletePlanReview(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-del-001",
		ThreadID:     "thread-del",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Delete Plan",
	})
	require.NoError(t, err)

	err = store.DeletePlanReview(ctx, "pr-del-001")
	require.NoError(t, err)

	_, err = store.GetPlanReview(ctx, "pr-del-001")
	require.Error(t, err)
}

// TestDuplicatePlanReviewID verifies uniqueness constraint on plan_review_id.
func TestDuplicatePlanReviewID(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	_, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-dup-001",
		ThreadID:     "thread-dup-1",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "First",
	})
	require.NoError(t, err)

	// Creating with the same plan_review_id should fail.
	_, err = store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-dup-001",
		ThreadID:     "thread-dup-2",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan2.md",
		PlanTitle:    "Duplicate",
	})
	require.Error(t, err)
}
