package review

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
)

// testDB creates a temporary test database with all migrations applied.
func testDB(t *testing.T) (store.Storage, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "subtrate-review-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	sqlDB, err := db.OpenSQLite(dbPath)
	require.NoError(t, err)

	migrationsDir := findMigrationsDir(t)
	err = db.RunMigrations(sqlDB, migrationsDir)
	require.NoError(t, err)

	storage := store.FromDB(sqlDB)

	cleanup := func() {
		storage.Close()
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

// findMigrationsDir locates the migrations directory relative to the test.
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	paths := []string{
		"../db/migrations",
		"../../internal/db/migrations",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		p := filepath.Join(
			gopath,
			"src/github.com/roasbeef/subtrate/internal/db/migrations",
		)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatal("Could not find migrations directory")
	return ""
}

// createTestAgent creates an agent record for testing.
func createTestAgent(
	t *testing.T, storage store.Storage, name string,
) store.Agent {
	t.Helper()

	agent, err := storage.CreateAgent(
		context.Background(), store.CreateAgentParams{
			Name: name,
		},
	)
	require.NoError(t, err)

	return agent
}

// newTestService creates a review Service backed by a real database and a
// test actor system for sub-actor lifecycle management.
func newTestService(
	t *testing.T,
) (*Service, store.Storage, func()) {
	t.Helper()

	as := actor.NewActorSystem()
	storage, cleanup := testDB(t)
	svc := NewService(ServiceConfig{
		Store:       storage,
		ActorSystem: as,
	})

	return svc, storage, func() {
		// Use a short timeout for shutdown in tests since spawned
		// reviewer actors may be stuck connecting to the Claude CLI
		// which doesn't exist in the test environment.
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		as.Shutdown(shutdownCtx)
		cleanup()
	}
}

// TestService_CreateReview tests creating a new review and verifying the
// response contains the expected fields.
func TestService_CreateReview(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Developer")

	result := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		PRNumber:    42,
		Branch:      "feature/auth",
		BaseBranch:  "main",
		CommitSHA:   "abc123def",
		RepoPath:    "/tmp/repo",
		RemoteURL:   "https://github.com/org/repo",
		ReviewType:  "full",
		Priority:    "normal",
	})

	val, err := result.Unpack()
	require.NoError(t, err)

	resp, ok := val.(CreateReviewResp)
	require.True(t, ok)
	require.NoError(t, resp.Error)
	require.NotEmpty(t, resp.ReviewID)
	require.NotEmpty(t, resp.ThreadID)

	// After create, the SpawnReviewerAgent outbox event fires
	// spawnReviewer(), which transitions pending_review → under_review.
	require.Equal(t, "under_review", resp.State)
}

// TestService_CreateReview_Defaults tests that review type and priority
// default when not provided.
func TestService_CreateReview_Defaults(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Dev")

	result := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "fix/bug",
		CommitSHA:   "deadbeef",
		RepoPath:    "/tmp/repo",
	})

	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(CreateReviewResp)
	require.NoError(t, resp.Error)
	require.NotEmpty(t, resp.ReviewID)

	// Verify defaults were applied by reading back.
	getResult := svc.Receive(ctx, GetReviewMsg{
		ReviewID: resp.ReviewID,
	})
	getVal, err := getResult.Unpack()
	require.NoError(t, err)

	getResp := getVal.(GetReviewResp)
	require.NoError(t, getResp.Error)
	require.Equal(t, "full", getResp.ReviewType)
}

// TestService_GetReview tests retrieving review details after creation.
func TestService_GetReview(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Author")

	// Create a review first.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		PRNumber:    99,
		Branch:      "feature/websocket",
		BaseBranch:  "develop",
		CommitSHA:   "f00baa",
		RepoPath:    "/tmp/repo",
		ReviewType:  "security",
		Priority:    "urgent",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// Now get the review.
	getResult := svc.Receive(ctx, GetReviewMsg{
		ReviewID: createResp.ReviewID,
	})
	getVal, err := getResult.Unpack()
	require.NoError(t, err)

	getResp := getVal.(GetReviewResp)
	require.NoError(t, getResp.Error)
	require.Equal(t, createResp.ReviewID, getResp.ReviewID)
	require.Equal(t, createResp.ThreadID, getResp.ThreadID)
	// After creation, spawnReviewer auto-transitions to under_review.
	require.Equal(t, "under_review", getResp.State)
	require.Equal(t, "feature/websocket", getResp.Branch)
	require.Equal(t, "develop", getResp.BaseBranch)
	require.Equal(t, "security", getResp.ReviewType)
	require.Equal(t, 0, getResp.Iterations)
	require.Equal(t, int64(0), getResp.OpenIssues)
}

// TestService_GetReview_NotFound tests that getting a nonexistent review
// returns an error.
func TestService_GetReview_NotFound(t *testing.T) {
	t.Parallel()

	svc, _, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()

	result := svc.Receive(ctx, GetReviewMsg{
		ReviewID: "nonexistent-id",
	})
	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(GetReviewResp)
	require.Error(t, resp.Error)
}

// TestService_ListReviews tests listing reviews with pagination.
func TestService_ListReviews(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Lister")

	// Create multiple reviews.
	for i := 0; i < 3; i++ {
		result := svc.Receive(ctx, CreateReviewMsg{
			RequesterID: requester.ID,
			Branch:      "branch-" + string(rune('a'+i)),
			CommitSHA:   "sha-" + string(rune('a'+i)),
			RepoPath:    "/tmp/repo",
		})
		val, err := result.Unpack()
		require.NoError(t, err)
		require.NoError(t, val.(CreateReviewResp).Error)
	}

	// List all reviews.
	listResult := svc.Receive(ctx, ListReviewsMsg{
		Limit: 10,
	})
	listVal, err := listResult.Unpack()
	require.NoError(t, err)

	listResp := listVal.(ListReviewsResp)
	require.NoError(t, listResp.Error)
	require.Len(t, listResp.Reviews, 3)

	// Verify each review has expected fields.
	for _, r := range listResp.Reviews {
		require.NotEmpty(t, r.ReviewID)
		require.NotEmpty(t, r.ThreadID)
		require.Equal(t, requester.ID, r.RequesterID)
		require.NotZero(t, r.CreatedAt)
	}
}

// TestService_ListReviewsByState tests filtering reviews by state.
func TestService_ListReviewsByState(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "StateFilter")

	// Create a review (spawnReviewer auto-transitions to under_review).
	result := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/state-test",
		CommitSHA:   "abc",
		RepoPath:    "/tmp/repo",
	})
	val, err := result.Unpack()
	require.NoError(t, err)
	require.NoError(t, val.(CreateReviewResp).Error)

	// List by under_review state (auto-transitioned from pending_review).
	listResult := svc.Receive(ctx, ListReviewsMsg{
		State: "under_review",
		Limit: 10,
	})
	listVal, err := listResult.Unpack()
	require.NoError(t, err)

	listResp := listVal.(ListReviewsResp)
	require.NoError(t, listResp.Error)
	require.Len(t, listResp.Reviews, 1)
	require.Equal(t, "under_review", listResp.Reviews[0].State)

	// List by a state with no reviews.
	emptyResult := svc.Receive(ctx, ListReviewsMsg{
		State: "approved",
		Limit: 10,
	})
	emptyVal, err := emptyResult.Unpack()
	require.NoError(t, err)

	emptyResp := emptyVal.(ListReviewsResp)
	require.NoError(t, emptyResp.Error)
	require.Empty(t, emptyResp.Reviews)
}

// TestService_ListReviewsByRequester tests filtering reviews by requester.
func TestService_ListReviewsByRequester(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	alice := createTestAgent(t, storage, "Alice")
	bob := createTestAgent(t, storage, "Bob")

	// Alice creates 2 reviews.
	for i := 0; i < 2; i++ {
		result := svc.Receive(ctx, CreateReviewMsg{
			RequesterID: alice.ID,
			Branch:      "alice-branch",
			CommitSHA:   "alice-sha",
			RepoPath:    "/tmp/repo",
		})
		val, err := result.Unpack()
		require.NoError(t, err)
		require.NoError(t, val.(CreateReviewResp).Error)
	}

	// Bob creates 1 review.
	result := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: bob.ID,
		Branch:      "bob-branch",
		CommitSHA:   "bob-sha",
		RepoPath:    "/tmp/repo",
	})
	val, err := result.Unpack()
	require.NoError(t, err)
	require.NoError(t, val.(CreateReviewResp).Error)

	// List Alice's reviews.
	aliceResult := svc.Receive(ctx, ListReviewsMsg{
		RequesterID: alice.ID,
		Limit:       10,
	})
	aliceVal, err := aliceResult.Unpack()
	require.NoError(t, err)

	aliceResp := aliceVal.(ListReviewsResp)
	require.NoError(t, aliceResp.Error)
	require.Len(t, aliceResp.Reviews, 2)

	// List Bob's reviews.
	bobResult := svc.Receive(ctx, ListReviewsMsg{
		RequesterID: bob.ID,
		Limit:       10,
	})
	bobVal, err := bobResult.Unpack()
	require.NoError(t, err)

	bobResp := bobVal.(ListReviewsResp)
	require.NoError(t, bobResp.Error)
	require.Len(t, bobResp.Reviews, 1)
}

// TestService_Resubmit tests resubmitting a review after changes requested.
func TestService_Resubmit(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Resubmitter")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/resubmit",
		CommitSHA:   "original-sha",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// The review auto-transitions to under_review via spawnReviewer.
	// Advance the FSM to changes_requested state.
	svc.mu.RLock()
	fsm := svc.activeReviews[createResp.ReviewID]
	svc.mu.RUnlock()
	require.NotNil(t, fsm)
	require.Equal(t, "under_review", fsm.CurrentState())

	// under_review → changes_requested.
	_, err = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "TestReviewer",
		Issues: []ReviewIssueSummary{
			{Title: "Fix this", Severity: "high"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "changes_requested", fsm.CurrentState())

	// Now resubmit through the service.
	resubResult := svc.Receive(ctx, ResubmitMsg{
		ReviewID:  createResp.ReviewID,
		CommitSHA: "new-sha-after-fixes",
	})
	resubVal, err := resubResult.Unpack()
	require.NoError(t, err)

	resubResp := resubVal.(ResubmitResp)
	require.NoError(t, resubResp.Error)
	require.Equal(t, createResp.ReviewID, resubResp.ReviewID)

	// After resubmit, the SpawnReviewerAgent outbox event triggers
	// spawnReviewer(), which auto-transitions re_review → under_review.
	require.Equal(t, "under_review", resubResp.NewState)
}

// TestService_Resubmit_RecoverFromDB tests that resubmit can recover an FSM
// from the database when the review is not in active memory.
func TestService_Resubmit_RecoverFromDB(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Recoverer")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/recover",
		CommitSHA:   "sha-recover",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// FSM auto-transitions to under_review. Advance to changes_requested.
	svc.mu.RLock()
	fsm := svc.activeReviews[createResp.ReviewID]
	svc.mu.RUnlock()
	require.NotNil(t, fsm)
	require.Equal(t, "under_review", fsm.CurrentState())

	_, err = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
	})
	require.NoError(t, err)

	// Persist the changes_requested state to DB.
	err = storage.UpdateReviewState(
		ctx, createResp.ReviewID, "changes_requested",
	)
	require.NoError(t, err)

	// Remove from active reviews to simulate a restart.
	svc.mu.Lock()
	delete(svc.activeReviews, createResp.ReviewID)
	svc.mu.Unlock()

	// Resubmit should recover from DB.
	resubResult := svc.Receive(ctx, ResubmitMsg{
		ReviewID:  createResp.ReviewID,
		CommitSHA: "new-sha",
	})
	resubVal, err := resubResult.Unpack()
	require.NoError(t, err)

	resubResp := resubVal.(ResubmitResp)
	require.NoError(t, resubResp.Error)

	// After resubmit + spawnReviewer, the FSM transitions through
	// re_review → under_review automatically.
	require.Equal(t, "under_review", resubResp.NewState)

	// Verify the FSM was restored into active reviews.
	svc.mu.RLock()
	_, exists := svc.activeReviews[createResp.ReviewID]
	svc.mu.RUnlock()
	require.True(t, exists)
}

// TestService_Cancel tests cancelling an active review.
func TestService_Cancel(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Canceller")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/cancel",
		CommitSHA:   "cancel-sha",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// The review is tracked as active.
	require.Equal(t, 1, svc.ActiveReviewCount())

	// Cancel the review.
	cancelResult := svc.Receive(ctx, CancelReviewMsg{
		ReviewID: createResp.ReviewID,
		Reason:   "PR was closed",
	})
	cancelVal, err := cancelResult.Unpack()
	require.NoError(t, err)

	cancelResp := cancelVal.(CancelReviewResp)
	require.NoError(t, cancelResp.Error)

	// The review should be removed from active reviews.
	require.Equal(t, 0, svc.ActiveReviewCount())
}

// TestService_Cancel_RecoverFromDB tests cancelling a review that needs to be
// recovered from the database.
func TestService_Cancel_RecoverFromDB(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "CancelRecover")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/cancel-recover",
		CommitSHA:   "sha-cr",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// Remove from active reviews to simulate restart.
	svc.mu.Lock()
	delete(svc.activeReviews, createResp.ReviewID)
	svc.mu.Unlock()

	// Cancel should recover from DB and still work.
	cancelResult := svc.Receive(ctx, CancelReviewMsg{
		ReviewID: createResp.ReviewID,
		Reason:   "no longer needed",
	})
	cancelVal, err := cancelResult.Unpack()
	require.NoError(t, err)

	cancelResp := cancelVal.(CancelReviewResp)
	require.NoError(t, cancelResp.Error)
	require.Equal(t, 0, svc.ActiveReviewCount())
}

// TestService_GetIssues tests retrieving issues for a review.
func TestService_GetIssues(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "IssueViewer")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/issues",
		CommitSHA:   "issue-sha",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// Create some issues directly in the store.
	_, err = storage.CreateReviewIssue(ctx, store.CreateReviewIssueParams{
		ReviewID:     createResp.ReviewID,
		IterationNum: 1,
		IssueType:    "bug",
		Severity:     "high",
		FilePath:     "main.go",
		LineStart:    42,
		Title:        "Null pointer dereference",
		Description:  "The pointer is never checked before use.",
		Suggestion:   "Add nil check.",
	})
	require.NoError(t, err)

	_, err = storage.CreateReviewIssue(ctx, store.CreateReviewIssueParams{
		ReviewID:     createResp.ReviewID,
		IterationNum: 1,
		IssueType:    "style",
		Severity:     "low",
		FilePath:     "utils.go",
		LineStart:    10,
		Title:        "Missing comment",
		Description:  "Public function lacks documentation.",
	})
	require.NoError(t, err)

	// Get issues through the service.
	issuesResult := svc.Receive(ctx, GetIssuesMsg{
		ReviewID: createResp.ReviewID,
	})
	issuesVal, err := issuesResult.Unpack()
	require.NoError(t, err)

	issuesResp := issuesVal.(GetIssuesResp)
	require.NoError(t, issuesResp.Error)
	require.Len(t, issuesResp.Issues, 2)

	// Verify issue fields.
	require.Equal(t, "bug", issuesResp.Issues[0].IssueType)
	require.Equal(t, "high", issuesResp.Issues[0].Severity)
	require.Equal(t, "main.go", issuesResp.Issues[0].FilePath)
	require.Equal(t, 42, issuesResp.Issues[0].LineStart)
	require.Equal(t, "Null pointer dereference", issuesResp.Issues[0].Title)
}

// TestService_UpdateIssue tests updating the status of a review issue.
func TestService_UpdateIssue(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "IssueUpdater")

	// Create a review.
	createResult := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "feature/update-issue",
		CommitSHA:   "update-sha",
		RepoPath:    "/tmp/repo",
	})
	createVal, err := createResult.Unpack()
	require.NoError(t, err)
	createResp := createVal.(CreateReviewResp)
	require.NoError(t, createResp.Error)

	// Create an issue.
	issue, err := storage.CreateReviewIssue(
		ctx, store.CreateReviewIssueParams{
			ReviewID:     createResp.ReviewID,
			IterationNum: 1,
			IssueType:    "bug",
			Severity:     "critical",
			FilePath:     "handler.go",
			LineStart:    100,
			Title:        "SQL injection vulnerability",
			Description:  "User input is not sanitized.",
		},
	)
	require.NoError(t, err)

	// Update the issue status to fixed.
	updateResult := svc.Receive(ctx, UpdateIssueMsg{
		ReviewID: createResp.ReviewID,
		IssueID:  issue.ID,
		Status:   "fixed",
	})
	updateVal, err := updateResult.Unpack()
	require.NoError(t, err)

	updateResp := updateVal.(UpdateIssueResp)
	require.NoError(t, updateResp.Error)

	// Verify the issue status changed.
	issues, err := storage.GetReviewIssues(ctx, createResp.ReviewID)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "fixed", issues[0].Status)
}

// TestService_RecoverActiveReviews tests that active reviews can be recovered
// from the database on startup.
func TestService_RecoverActiveReviews(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "RecoverTest")

	// Create two reviews.
	for i := 0; i < 2; i++ {
		result := svc.Receive(ctx, CreateReviewMsg{
			RequesterID: requester.ID,
			Branch:      "recover-branch",
			CommitSHA:   "recover-sha",
			RepoPath:    "/tmp/repo",
		})
		val, err := result.Unpack()
		require.NoError(t, err)
		require.NoError(t, val.(CreateReviewResp).Error)
	}

	require.Equal(t, 2, svc.ActiveReviewCount())

	// Create a fresh service to simulate restart.
	as2 := actor.NewActorSystem()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		as2.Shutdown(shutdownCtx)
	}()
	svc2 := NewService(ServiceConfig{
		Store:       storage,
		ActorSystem: as2,
	})
	require.Equal(t, 0, svc2.ActiveReviewCount())

	// Recover active reviews from DB.
	err := svc2.RecoverActiveReviews(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, svc2.ActiveReviewCount())
}

// TestService_ActiveReviewCount tracks count through create and cancel.
func TestService_ActiveReviewCount(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()
	requester := createTestAgent(t, storage, "Counter")

	require.Equal(t, 0, svc.ActiveReviewCount())

	// Create first review.
	r1 := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "b1",
		CommitSHA:   "s1",
		RepoPath:    "/tmp/repo",
	})
	v1, err := r1.Unpack()
	require.NoError(t, err)
	resp1 := v1.(CreateReviewResp)
	require.NoError(t, resp1.Error)

	require.Equal(t, 1, svc.ActiveReviewCount())

	// Create second review.
	r2 := svc.Receive(ctx, CreateReviewMsg{
		RequesterID: requester.ID,
		Branch:      "b2",
		CommitSHA:   "s2",
		RepoPath:    "/tmp/repo",
	})
	v2, err := r2.Unpack()
	require.NoError(t, err)
	require.NoError(t, v2.(CreateReviewResp).Error)

	require.Equal(t, 2, svc.ActiveReviewCount())

	// Cancel first review.
	cr := svc.Receive(ctx, CancelReviewMsg{
		ReviewID: resp1.ReviewID,
		Reason:   "done",
	})
	cv, err := cr.Unpack()
	require.NoError(t, err)
	require.NoError(t, cv.(CancelReviewResp).Error)

	require.Equal(t, 1, svc.ActiveReviewCount())
}

// TestService_UnknownMessageType tests that an unknown message type returns
// an error through the Result type.
func TestService_UnknownMessageType(t *testing.T) {
	t.Parallel()

	svc, _, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Send a message type that isn't handled.
	result := svc.Receive(ctx, unknownMsg{})
	_, err := result.Unpack()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown message type")
}

// TestService_ReviewerConfigs tests that the service has the expected reviewer
// configurations registered.
func TestService_ReviewerConfigs(t *testing.T) {
	t.Parallel()

	svc, _, cleanup := newTestService(t)
	defer cleanup()

	// The service should have the default "full" config plus specialized
	// reviewers.
	require.NotNil(t, svc.reviewers["full"])
	require.Equal(t, "CodeReviewer", svc.reviewers["full"].Name)

	require.NotNil(t, svc.reviewers["security"])
	require.Equal(t, "SecurityReviewer", svc.reviewers["security"].Name)

	require.NotNil(t, svc.reviewers["performance"])
	require.Equal(
		t, "PerformanceReviewer",
		svc.reviewers["performance"].Name,
	)

	require.NotNil(t, svc.reviewers["architecture"])
	require.Equal(
		t, "ArchitectureReviewer",
		svc.reviewers["architecture"].Name,
	)
}

// TestService_HandleSubActorResultTwice verifies that calling
// handleSubActorResult twice on the same review (first with request_changes,
// then with approve) correctly transitions the FSM through
// changes_requested → approved and reaches a terminal state.
func TestService_HandleSubActorResultTwice(t *testing.T) {
	t.Parallel()

	svc, storage, cleanup := newTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent to be the requester.
	agent := createTestAgent(t, storage, "TestAuthor")

	// Create a review record in the DB.
	review, err := storage.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    "review-double-callback",
		ThreadID:    "thread-double",
		RepoPath:    "/tmp/repo",
		BaseBranch:  "main",
		Branch:      "feature",
		ReviewType:  "full",
		Priority:    "normal",
		RequesterID: agent.ID,
	})
	require.NoError(t, err)

	// Set up an FSM already in under_review state and register it.
	fsm := NewReviewFSMFromDB(
		review.ReviewID, review.ThreadID,
		review.RepoPath, agent.ID, "under_review",
	)
	svc.mu.Lock()
	svc.activeReviews[review.ReviewID] = fsm
	svc.mu.Unlock()

	// First callback: request_changes.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID: review.ReviewID,
		Result: &ReviewerResult{
			Decision: "request_changes",
			Summary:  "Found issues",
			Issues: []ReviewerIssue{
				{
					Title:       "Missing validation",
					Severity:    "high",
					IssueType:   "bug",
					Description: "Input not validated",
				},
			},
		},
		Duration: 5 * time.Second,
	})

	require.Equal(t, "changes_requested", fsm.CurrentState())
	require.False(t, fsm.IsTerminal())

	// Second callback: approve (reviewer updated decision after
	// back-and-forth conversation).
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID: review.ReviewID,
		Result: &ReviewerResult{
			Decision: "approve",
			Summary:  "Issues addressed in discussion",
		},
		Duration: 3 * time.Second,
	})

	require.Equal(t, "approved", fsm.CurrentState())
	require.True(t, fsm.IsTerminal())

	// Verify the FSM was removed from active tracking since it
	// reached a terminal state.
	svc.mu.RLock()
	_, stillActive := svc.activeReviews[review.ReviewID]
	svc.mu.RUnlock()
	require.False(t, stillActive)
}

// unknownMsg is a test-only message type that the service doesn't handle.
type unknownMsg struct {
	actor.BaseMessage
}

func (unknownMsg) isReviewRequest()    {}
func (unknownMsg) MessageType() string { return "UnknownMsg" }
