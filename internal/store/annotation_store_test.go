package store

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// TestCreatePlanAnnotation verifies plan annotation creation and retrieval.
func TestCreatePlanAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	// Create a plan review to satisfy the FK constraint.
	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-ann-001",
		ThreadID:     "thread-ann-001",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Test Plan",
		SessionID:    "session-ann-001",
	})
	require.NoError(t, err)

	// Create a plan annotation.
	ann, err := store.CreatePlanAnnotation(ctx, CreatePlanAnnotationParams{
		PlanReviewID:   review.PlanReviewID,
		AnnotationID:   "ann-001",
		BlockID:        "block-0",
		AnnotationType: "COMMENT",
		Text:           "This needs more detail.",
		OriginalText:   "vague section",
		StartOffset:    5,
		EndOffset:      18,
	})
	require.NoError(t, err)
	require.Equal(t, "ann-001", ann.AnnotationID)
	require.Equal(t, "COMMENT", ann.AnnotationType)
	require.Equal(t, "This needs more detail.", ann.Text)
	require.Equal(t, "vague section", ann.OriginalText)
	require.Equal(t, 5, ann.StartOffset)
	require.Equal(t, 18, ann.EndOffset)
	require.NotZero(t, ann.CreatedAt)

	// Retrieve by annotation ID.
	fetched, err := store.GetPlanAnnotation(ctx, "ann-001")
	require.NoError(t, err)
	require.Equal(t, ann.AnnotationID, fetched.AnnotationID)
	require.Equal(t, ann.Text, fetched.Text)
}

// TestListPlanAnnotationsByReview verifies listing annotations for a
// plan review.
func TestListPlanAnnotationsByReview(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-list-001",
		ThreadID:     "thread-list-001",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Test Plan",
		SessionID:    "session-list-001",
	})
	require.NoError(t, err)

	// Create multiple annotations.
	for i := 0; i < 3; i++ {
		_, err := store.CreatePlanAnnotation(ctx, CreatePlanAnnotationParams{
			PlanReviewID:   review.PlanReviewID,
			AnnotationID:   "ann-list-" + string(rune('a'+i)),
			BlockID:        "block-" + string(rune('0'+i)),
			AnnotationType: "COMMENT",
			Text:           "Comment " + string(rune('A'+i)),
			OriginalText:   "text",
		})
		require.NoError(t, err)
	}

	// List all annotations for the review.
	annotations, err := store.ListPlanAnnotationsByReview(
		ctx, review.PlanReviewID,
	)
	require.NoError(t, err)
	require.Len(t, annotations, 3)
}

// TestUpdatePlanAnnotation verifies annotation text updates.
func TestUpdatePlanAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-upd-001",
		ThreadID:     "thread-upd-001",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Test Plan",
		SessionID:    "session-upd-001",
	})
	require.NoError(t, err)

	_, err = store.CreatePlanAnnotation(ctx, CreatePlanAnnotationParams{
		PlanReviewID:   review.PlanReviewID,
		AnnotationID:   "ann-upd-001",
		BlockID:        "block-0",
		AnnotationType: "COMMENT",
		Text:           "original comment",
		OriginalText:   "text",
	})
	require.NoError(t, err)

	// Update the annotation.
	_, err = store.UpdatePlanAnnotation(ctx, UpdatePlanAnnotationParams{
		AnnotationID: "ann-upd-001",
		Text:         "updated comment",
		OriginalText: "text",
		StartOffset:  0,
		EndOffset:    4,
	})
	require.NoError(t, err)

	// Verify the update.
	ann, err := store.GetPlanAnnotation(ctx, "ann-upd-001")
	require.NoError(t, err)
	require.Equal(t, "updated comment", ann.Text)
}

// TestDeletePlanAnnotation verifies annotation deletion.
func TestDeletePlanAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	review, err := store.CreatePlanReview(ctx, CreatePlanReviewParams{
		PlanReviewID: "pr-del-001",
		ThreadID:     "thread-del-001",
		RequesterID:  agentID,
		ReviewerName: "User",
		PlanPath:     "/tmp/plan.md",
		PlanTitle:    "Test Plan",
		SessionID:    "session-del-001",
	})
	require.NoError(t, err)

	_, err = store.CreatePlanAnnotation(ctx, CreatePlanAnnotationParams{
		PlanReviewID:   review.PlanReviewID,
		AnnotationID:   "ann-del-001",
		BlockID:        "block-0",
		AnnotationType: "DELETION",
		OriginalText:   "remove me",
	})
	require.NoError(t, err)

	// Delete the annotation.
	err = store.DeletePlanAnnotation(ctx, "ann-del-001")
	require.NoError(t, err)

	// Verify it's gone.
	_, err = store.GetPlanAnnotation(ctx, "ann-del-001")
	require.Error(t, err)
}

// TestCreateDiffAnnotation verifies diff annotation creation.
func TestCreateDiffAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	// Create a message to satisfy the FK constraint.
	topic, err := store.CreateTopic(ctx, CreateTopicParams{
		Name:      "inbox:test-agent-diff",
		TopicType: "direct",
	})
	require.NoError(t, err)

	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID: "thread-diff-001",
		TopicID:  topic.ID,
		SenderID: agentID,
		Subject:  "diff message",
		Body:     "some diff content",
		Priority: "normal",
	})
	require.NoError(t, err)

	// Create a diff annotation.
	ann, err := store.CreateDiffAnnotation(ctx, CreateDiffAnnotationParams{
		AnnotationID:   "diff-ann-001",
		MessageID:      msg.ID,
		AnnotationType: "comment",
		Scope:          "line",
		FilePath:       "main.go",
		LineStart:      42,
		LineEnd:        45,
		Side:           "new",
		Text:           "This logic is fragile.",
	})
	require.NoError(t, err)
	require.Equal(t, "diff-ann-001", ann.AnnotationID)
	require.Equal(t, msg.ID, ann.MessageID)
	require.Equal(t, "comment", ann.AnnotationType)
	require.Equal(t, "main.go", ann.FilePath)
	require.Equal(t, 42, ann.LineStart)
	require.Equal(t, 45, ann.LineEnd)
	require.Equal(t, "new", ann.Side)
	require.Equal(t, "This logic is fragile.", ann.Text)
}

// TestListDiffAnnotationsByMessage verifies listing diff annotations.
func TestListDiffAnnotationsByMessage(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	topic, err := store.CreateTopic(ctx, CreateTopicParams{
		Name:      "inbox:test-agent-diff-list",
		TopicType: "direct",
	})
	require.NoError(t, err)

	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID: "thread-diff-list",
		TopicID:  topic.ID,
		SenderID: agentID,
		Subject:  "diff message",
		Body:     "diff content",
		Priority: "normal",
	})
	require.NoError(t, err)

	// Create annotations on different files.
	for _, file := range []string{"a.go", "b.go"} {
		_, err := store.CreateDiffAnnotation(
			ctx, CreateDiffAnnotationParams{
				AnnotationID:   "diff-list-" + file,
				MessageID:      msg.ID,
				AnnotationType: "comment",
				Scope:          "line",
				FilePath:       file,
				LineStart:      1,
				LineEnd:        1,
				Side:           "new",
				Text:           "comment on " + file,
			},
		)
		require.NoError(t, err)
	}

	annotations, err := store.ListDiffAnnotationsByMessage(ctx, msg.ID)
	require.NoError(t, err)
	require.Len(t, annotations, 2)
}

// TestUpdateDiffAnnotation verifies diff annotation text updates.
func TestUpdateDiffAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	topic, err := store.CreateTopic(ctx, CreateTopicParams{
		Name:      "inbox:test-agent-diff-upd",
		TopicType: "direct",
	})
	require.NoError(t, err)

	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID: "thread-diff-upd",
		TopicID:  topic.ID,
		SenderID: agentID,
		Subject:  "diff message",
		Body:     "diff content",
		Priority: "normal",
	})
	require.NoError(t, err)

	_, err = store.CreateDiffAnnotation(ctx, CreateDiffAnnotationParams{
		AnnotationID:   "diff-upd-001",
		MessageID:      msg.ID,
		AnnotationType: "comment",
		Scope:          "line",
		FilePath:       "main.go",
		LineStart:      10,
		LineEnd:        12,
		Side:           "new",
		Text:           "original comment",
	})
	require.NoError(t, err)

	// Update the annotation.
	updated, err := store.UpdateDiffAnnotation(
		ctx, UpdateDiffAnnotationParams{
			AnnotationID:  "diff-upd-001",
			Text:          "updated comment",
			SuggestedCode: "fmt.Println(\"fixed\")",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "updated comment", updated.Text)
	require.Equal(t, "fmt.Println(\"fixed\")", updated.SuggestedCode)

	// Verify via get.
	fetched, err := store.GetDiffAnnotation(ctx, "diff-upd-001")
	require.NoError(t, err)
	require.Equal(t, "updated comment", fetched.Text)
}

// TestDeleteDiffAnnotation verifies diff annotation deletion.
func TestDeleteDiffAnnotation(t *testing.T) {
	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "test-agent")

	topic, err := store.CreateTopic(ctx, CreateTopicParams{
		Name:      "inbox:test-agent-diff-del",
		TopicType: "direct",
	})
	require.NoError(t, err)

	msg, err := store.CreateMessage(ctx, CreateMessageParams{
		ThreadID: "thread-diff-del",
		TopicID:  topic.ID,
		SenderID: agentID,
		Subject:  "diff message",
		Body:     "diff content",
		Priority: "normal",
	})
	require.NoError(t, err)

	_, err = store.CreateDiffAnnotation(ctx, CreateDiffAnnotationParams{
		AnnotationID:   "diff-del-001",
		MessageID:      msg.ID,
		AnnotationType: "suggestion",
		Scope:          "line",
		FilePath:       "main.go",
		LineStart:      1,
		LineEnd:        1,
		Side:           "new",
		Text:           "remove this",
	})
	require.NoError(t, err)

	// Delete the annotation.
	err = store.DeleteDiffAnnotation(ctx, "diff-del-001")
	require.NoError(t, err)

	// Verify it's gone.
	_, err = store.GetDiffAnnotation(ctx, "diff-del-001")
	require.Error(t, err)
}
