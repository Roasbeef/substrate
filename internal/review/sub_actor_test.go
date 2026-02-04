package review

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
)

// TestParseReviewerResponse_Approve tests parsing an approve decision.
func TestParseReviewerResponse_Approve(t *testing.T) {
	response := `Here is my review of the changes:

The code looks clean, well-structured, and follows project conventions.
No issues found.

` + "```yaml\n" + `decision: approve
summary: "Code is well-written with proper error handling and tests."
files_reviewed: 8
lines_analyzed: 450
issues: []
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Equal(t, "approve", result.Decision)
	require.Equal(t,
		"Code is well-written with proper error handling and tests.",
		result.Summary,
	)
	require.Equal(t, 8, result.FilesReviewed)
	require.Equal(t, 450, result.LinesAnalyzed)
	require.Empty(t, result.Issues)
}

// TestParseReviewerResponse_RequestChanges tests parsing with issues.
func TestParseReviewerResponse_RequestChanges(t *testing.T) {
	response := `After reviewing the changes, I found a few issues.

` + "```yaml\n" + `decision: request_changes
summary: "Found 2 issues: a potential nil pointer and a missing test."
files_reviewed: 5
lines_analyzed: 300
issues:
  - title: "Potential nil pointer dereference"
    type: bug
    severity: high
    file: "internal/review/service.go"
    line_start: 42
    line_end: 45
    description: "The error return is not checked before accessing the result."
    suggestion: "Add an error check before using the result."
  - title: "Missing test for edge case"
    type: style
    severity: medium
    file: "internal/review/service_test.go"
    line_start: 100
    line_end: 100
    description: "No test for the empty list case."
    suggestion: "Add a test case for when there are no reviews."
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Equal(t, "request_changes", result.Decision)
	require.Len(t, result.Issues, 2)

	// First issue.
	require.Equal(t,
		"Potential nil pointer dereference", result.Issues[0].Title,
	)
	require.Equal(t, "bug", result.Issues[0].IssueType)
	require.Equal(t, "high", result.Issues[0].Severity)
	require.Equal(t,
		"internal/review/service.go", result.Issues[0].FilePath,
	)
	require.Equal(t, 42, result.Issues[0].LineStart)
	require.Equal(t, 45, result.Issues[0].LineEnd)

	// Second issue.
	require.Equal(t,
		"Missing test for edge case", result.Issues[1].Title,
	)
	require.Equal(t, "style", result.Issues[1].IssueType)
	require.Equal(t, "medium", result.Issues[1].Severity)
}

// TestParseReviewerResponse_Reject tests parsing a reject decision.
func TestParseReviewerResponse_Reject(t *testing.T) {
	response := "```yaml\n" + `decision: reject
summary: "Fundamental architecture issues that cannot be resolved with minor changes."
files_reviewed: 3
lines_analyzed: 200
issues:
  - title: "Wrong abstraction level"
    type: architecture
    severity: critical
    file: "internal/review/service.go"
    line_start: 1
    line_end: 100
    description: "The entire service needs to be restructured."
    suggestion: "Consider using the actor pattern instead."
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Equal(t, "reject", result.Decision)
	require.Len(t, result.Issues, 1)
	require.Equal(t, "critical", result.Issues[0].Severity)
}

// TestParseReviewerResponse_NoYAMLBlock tests error when no YAML found.
func TestParseReviewerResponse_NoYAMLBlock(t *testing.T) {
	response := "This is just a plain text response with no YAML block."

	_, err := ParseReviewerResponse(response)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no YAML frontmatter block")
}

// TestParseReviewerResponse_InvalidYAML tests error on malformed YAML.
func TestParseReviewerResponse_InvalidYAML(t *testing.T) {
	response := "```yaml\n" + `decision: approve
summary: [this is not valid yaml
` + "```"

	_, err := ParseReviewerResponse(response)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse YAML frontmatter")
}

// TestParseReviewerResponse_MissingDecision tests error on missing decision.
func TestParseReviewerResponse_MissingDecision(t *testing.T) {
	response := "```yaml\n" + `summary: "Looks good"
files_reviewed: 5
` + "```"

	_, err := ParseReviewerResponse(response)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required field: decision")
}

// TestParseReviewerResponse_InvalidDecision tests error on bad decision.
func TestParseReviewerResponse_InvalidDecision(t *testing.T) {
	response := "```yaml\n" + `decision: maybe
summary: "Not sure"
` + "```"

	_, err := ParseReviewerResponse(response)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid decision")
}

// TestParseReviewerResponse_MultipleYAMLBlocks takes the last block.
func TestParseReviewerResponse_MultipleYAMLBlocks(t *testing.T) {
	response := `Here's a code example:

` + "```yaml\n" + `# This is just an example config
key: value
` + "```" + `

And here's the actual review result:

` + "```yaml\n" + `decision: approve
summary: "All good"
files_reviewed: 3
lines_analyzed: 100
issues: []
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Equal(t, "approve", result.Decision)
	require.Equal(t, "All good", result.Summary)
}

// TestParseReviewerResponse_YmlMarker tests the ```yml alternative.
func TestParseReviewerResponse_YmlMarker(t *testing.T) {
	response := "```yml\n" + `decision: approve
summary: "Looks good"
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Equal(t, "approve", result.Decision)
}

// TestExtractYAMLBlock_Variants tests various YAML extraction edge cases.
func TestExtractYAMLBlock_Variants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard yaml block",
			input:    "text\n```yaml\nfoo: bar\n```\n",
			expected: "foo: bar",
		},
		{
			name:     "yml variant",
			input:    "text\n```yml\nfoo: bar\n```\n",
			expected: "foo: bar",
		},
		{
			name:     "no yaml block",
			input:    "just text, no yaml",
			expected: "",
		},
		{
			name:     "unclosed yaml block",
			input:    "text\n```yaml\nfoo: bar\n",
			expected: "",
		},
		{
			name:     "empty yaml block",
			input:    "text\n```yaml\n```\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractYAMLBlock(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildSystemPrompt_Default tests the default reviewer system prompt.
func TestBuildSystemPrompt_Default(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-review-1",
		config:   DefaultReviewerConfig(),
	}

	prompt := actor.buildSystemPrompt()

	require.Contains(t, prompt, "CodeReviewer")
	require.Contains(t, prompt, "Focus Areas")
	require.Contains(t, prompt, "bugs")
	require.Contains(t, prompt, "security_vulnerabilities")
	require.Contains(t, prompt, "YAML frontmatter")
	require.Contains(t, prompt, "decision: approve")
}

// TestBuildSystemPrompt_CustomPrompt uses the config's SystemPrompt if set.
func TestBuildSystemPrompt_CustomPrompt(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-review-1",
		config: &ReviewerConfig{
			Name:         "CustomReviewer",
			SystemPrompt: "You are a custom reviewer.",
			Model:        "claude-sonnet-4-20250514",
			Timeout:      5 * time.Minute,
		},
	}

	prompt := actor.buildSystemPrompt()
	require.Equal(t, "You are a custom reviewer.", prompt)
}

// TestBuildSystemPrompt_WithIgnorePatterns includes ignore patterns.
func TestBuildSystemPrompt_WithIgnorePatterns(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-review-1",
		config: &ReviewerConfig{
			Name:  "TestReviewer",
			Model: "claude-sonnet-4-20250514",
			FocusAreas: []string{
				"bugs",
			},
			IgnorePatterns: []string{
				"vendor/*",
				"*.generated.go",
			},
			Timeout: 5 * time.Minute,
		},
	}

	prompt := actor.buildSystemPrompt()
	require.Contains(t, prompt, "Ignore Patterns")
	require.Contains(t, prompt, "vendor/*")
	require.Contains(t, prompt, "*.generated.go")
}

// TestBuildReviewPrompt verifies the user prompt content.
func TestBuildReviewPrompt(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "abc-123",
		config:   DefaultReviewerConfig(),
	}

	prompt := actor.buildReviewPrompt()
	require.Contains(t, prompt, "abc-123")
	require.Contains(t, prompt, "git diff main...HEAD")
	require.Contains(t, prompt, "YAML frontmatter")
}

// TestBuildClientOptions verifies SDK options are correctly constructed.
func TestBuildClientOptions(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-1",
		repoPath: "/tmp/test-repo",
		config: &ReviewerConfig{
			Name:    "TestReviewer",
			Model:   "claude-sonnet-4-20250514",
			Timeout: 5 * time.Minute,
		},
		spawnCfg: &SpawnConfig{
			CLIPath:              "claude",
			MaxTurns:             15,
			NoSessionPersistence: true,
			ConfigDir:            "/tmp/claude-config",
		},
	}

	opts := actor.buildClientOptions()

	// We can't inspect the options directly, but we can verify that
	// buildClientOptions doesn't panic and returns a non-empty slice.
	require.NotEmpty(t, opts)
}

// TestSubActorManager_SpawnAndStop tests the manager lifecycle.
func TestSubActorManager_SpawnAndStop(t *testing.T) {
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(mockStore, &SpawnConfig{
		NoSessionPersistence: true,
	})

	require.Equal(t, 0, mgr.ActiveCount())

	// We can't actually spawn a real Claude agent in tests, but we can
	// verify the manager tracks actors correctly by using the internal
	// structure.
	mgr.mu.Lock()
	mgr.actors["test-review-1"] = &reviewSubActor{
		reviewID: "test-review-1",
	}
	mgr.mu.Unlock()

	require.Equal(t, 1, mgr.ActiveCount())

	// Stop a specific reviewer.
	mgr.StopReviewer("test-review-1")
	require.Equal(t, 0, mgr.ActiveCount())

	// StopAll with multiple actors.
	mgr.mu.Lock()
	mgr.actors["review-a"] = &reviewSubActor{reviewID: "review-a"}
	mgr.actors["review-b"] = &reviewSubActor{reviewID: "review-b"}
	mgr.mu.Unlock()

	require.Equal(t, 2, mgr.ActiveCount())
	mgr.StopAll()
	require.Equal(t, 0, mgr.ActiveCount())
}

// TestSubActorManager_DuplicateSpawnPrevented verifies that duplicate spawns
// for the same review ID are prevented.
func TestSubActorManager_DuplicateSpawnPrevented(t *testing.T) {
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(mockStore, &SpawnConfig{
		NoSessionPersistence: true,
	})

	// Manually insert an actor to simulate an active review.
	mgr.mu.Lock()
	mgr.actors["test-review-1"] = &reviewSubActor{
		reviewID: "test-review-1",
	}
	mgr.mu.Unlock()

	callCount := 0
	callback := func(_ context.Context, _ *SubActorResult) {
		callCount++
	}

	// Attempt to spawn a duplicate - should be a no-op.
	mgr.SpawnReviewer(
		context.Background(),
		"test-review-1", "thread-1", "/tmp/repo",
		1, DefaultReviewerConfig(), callback,
	)

	// The callback should not have been invoked since the spawn was
	// skipped due to duplicate detection.
	require.Equal(t, 0, callCount)
	require.Equal(t, 1, mgr.ActiveCount())
}

// TestHandleSubActorResult_Approve tests the service handling an approve
// result from the sub-actor.
func TestHandleSubActorResult_Approve(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	agent := createTestAgent(t, st, "ApproveTest")
	svc := NewService(ServiceConfig{Store: st})

	// Create a review to get an active FSM. The spawnReviewer
	// auto-transitions to under_review.
	createResp := svc.handleCreateReview(ctx, CreateReviewMsg{
		RequesterID: agent.ID,
		Branch:      "test-branch",
		BaseBranch:  "main",
		CommitSHA:   "abc123",
		RepoPath:    "/tmp/repo",
		ReviewType:  "full",
	})
	require.NoError(t, createResp.Error)
	reviewID := createResp.ReviewID

	svc.mu.RLock()
	fsm := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.NotNil(t, fsm)
	require.Equal(t, "under_review", fsm.CurrentState())

	// Now simulate the sub-actor completing with an approve decision.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID:  reviewID,
		SessionID: "sess-1",
		Result: &ReviewerResult{
			Decision:      "approve",
			Summary:       "All looks good",
			FilesReviewed: 5,
			LinesAnalyzed: 200,
		},
	})

	// The FSM should have transitioned to approved.
	require.Equal(t, "approved", fsm.CurrentState())

	// The review should be removed from activeReviews since it's
	// terminal.
	svc.mu.RLock()
	_, exists := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.False(t, exists)
}

// TestHandleSubActorResult_RequestChanges tests handling a request_changes
// result.
func TestHandleSubActorResult_RequestChanges(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	agent := createTestAgent(t, st, "RequestChangesTest")
	svc := NewService(ServiceConfig{Store: st})

	createResp := svc.handleCreateReview(ctx, CreateReviewMsg{
		RequesterID: agent.ID,
		Branch:      "test-branch",
		BaseBranch:  "main",
		CommitSHA:   "abc123",
		RepoPath:    "/tmp/repo",
		ReviewType:  "full",
	})
	require.NoError(t, createResp.Error)
	reviewID := createResp.ReviewID

	svc.mu.RLock()
	fsm := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.Equal(t, "under_review", fsm.CurrentState())

	// Simulate request_changes result.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID:  reviewID,
		SessionID: "sess-2",
		Result: &ReviewerResult{
			Decision: "request_changes",
			Summary:  "Found some issues",
			Issues: []ReviewerIssue{
				{
					Title:    "Missing error check",
					Severity: "high",
				},
			},
		},
	})

	// Should be in changes_requested (not terminal).
	require.Equal(t, "changes_requested", fsm.CurrentState())

	// Review should still be in activeReviews.
	svc.mu.RLock()
	_, exists := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.True(t, exists)
}

// TestHandleSubActorResult_Error tests that errors don't crash the FSM.
func TestHandleSubActorResult_Error(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	agent := createTestAgent(t, st, "ErrorTest")
	svc := NewService(ServiceConfig{Store: st})

	createResp := svc.handleCreateReview(ctx, CreateReviewMsg{
		RequesterID: agent.ID,
		Branch:      "test-branch",
		BaseBranch:  "main",
		CommitSHA:   "abc123",
		RepoPath:    "/tmp/repo",
		ReviewType:  "full",
	})
	require.NoError(t, createResp.Error)
	reviewID := createResp.ReviewID

	svc.mu.RLock()
	fsm := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.Equal(t, "under_review", fsm.CurrentState())

	// Simulate an error result.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID: reviewID,
		Error: fmt.Errorf(
			"failed to connect to Claude CLI",
		),
	})

	// The FSM should remain in under_review (error doesn't transition).
	require.Equal(t, "under_review", fsm.CurrentState())

	// Review should still be active so it can be retried.
	svc.mu.RLock()
	_, exists := svc.activeReviews[reviewID]
	svc.mu.RUnlock()
	require.True(t, exists)
}

// TestHandleSubActorResult_NoActiveFSM tests handling a result when the FSM
// has been removed (e.g., cancelled during review).
func TestHandleSubActorResult_NoActiveFSM(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(ServiceConfig{Store: st})

	// Call with a non-existent review ID - should not panic.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID:  "nonexistent-review",
		SessionID: "sess-3",
		Result: &ReviewerResult{
			Decision: "approve",
			Summary:  "Looks good",
		},
	})
}

// TestPersistResults_CreatesIterationAndIssues tests that the sub-actor
// correctly persists iteration and issue records.
func TestPersistResults_CreatesIterationAndIssues(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	// Create an agent for the FK constraint.
	agent := createTestAgent(t, st, "PersistTest")

	// Create a review record in the DB first.
	review, err := st.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    "review-persist-1",
		ThreadID:    "thread-persist-1",
		RequesterID: agent.ID,
		Branch:      "test",
		BaseBranch:  "main",
		CommitSHA:   "abc123",
		RepoPath:    "/tmp/repo",
		ReviewType:  "full",
		Priority:    "normal",
	})
	require.NoError(t, err)

	actor := &reviewSubActor{
		reviewID: review.ReviewID,
		config:   DefaultReviewerConfig(),
		store:    st,
	}

	startTime := time.Now().Add(-5 * time.Second)
	result := &SubActorResult{
		ReviewID:  review.ReviewID,
		SessionID: "sess-persist-1",
		CostUSD:   0.05,
		Duration:  5 * time.Second,
		Result: &ReviewerResult{
			Decision:      "request_changes",
			Summary:       "Found some issues",
			FilesReviewed: 3,
			LinesAnalyzed: 150,
			Issues: []ReviewerIssue{
				{
					Title:       "Bug in handler",
					IssueType:   "bug",
					Severity:    "high",
					FilePath:    "handler.go",
					LineStart:   10,
					LineEnd:     15,
					Description: "Missing nil check",
					Suggestion:  "Add nil check",
				},
				{
					Title:       "Style issue",
					IssueType:   "style",
					Severity:    "low",
					FilePath:    "util.go",
					LineStart:   5,
					LineEnd:     5,
					Description: "Function name should be camelCase",
				},
			},
		},
	}

	actor.persistResults(ctx, result, startTime)

	// Verify iteration was created.
	iters, err := st.GetReviewIterations(ctx, review.ReviewID)
	require.NoError(t, err)
	require.Len(t, iters, 1)
	require.Equal(t, 1, iters[0].IterationNum)
	require.Equal(t, "CodeReviewer", iters[0].ReviewerID)
	require.Equal(t, "request_changes", iters[0].Decision)
	require.Equal(t, "Found some issues", iters[0].Summary)

	// Verify issues were created.
	issues, err := st.GetReviewIssues(ctx, review.ReviewID)
	require.NoError(t, err)
	require.Len(t, issues, 2)
	require.Equal(t, "Bug in handler", issues[0].Title)
	require.Equal(t, "high", issues[0].Severity)
	require.Equal(t, "Style issue", issues[1].Title)
	require.Equal(t, "low", issues[1].Severity)
}

// TestPersistResults_NilResult is a no-op when result is nil.
func TestPersistResults_NilResult(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	actor := &reviewSubActor{
		reviewID: "review-nil",
		store:    st,
		config:   DefaultReviewerConfig(),
	}

	// Should not panic.
	actor.persistResults(ctx, &SubActorResult{
		ReviewID: "review-nil",
	}, time.Now())
}

// TestPersistResults_IncrementingIterationNum tests that successive iterations
// get incrementing numbers.
func TestPersistResults_IncrementingIterationNum(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	agent := createTestAgent(t, st, "IterIncTest")

	_, err := st.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    "review-iter-inc",
		ThreadID:    "thread-iter-inc",
		RequesterID: agent.ID,
		Branch:      "test",
		BaseBranch:  "main",
		CommitSHA:   "abc123",
		RepoPath:    "/tmp/repo",
		ReviewType:  "full",
		Priority:    "normal",
	})
	require.NoError(t, err)

	actor := &reviewSubActor{
		reviewID: "review-iter-inc",
		config:   DefaultReviewerConfig(),
		store:    st,
	}

	// Persist first iteration.
	actor.persistResults(ctx, &SubActorResult{
		ReviewID:  "review-iter-inc",
		SessionID: "sess-1",
		Duration:  3 * time.Second,
		Result: &ReviewerResult{
			Decision: "request_changes",
			Summary:  "First round",
		},
	}, time.Now())

	// Persist second iteration.
	actor.persistResults(ctx, &SubActorResult{
		ReviewID:  "review-iter-inc",
		SessionID: "sess-2",
		Duration:  2 * time.Second,
		Result: &ReviewerResult{
			Decision: "approve",
			Summary:  "Second round - all fixed",
		},
	}, time.Now())

	iters, err := st.GetReviewIterations(ctx, "review-iter-inc")
	require.NoError(t, err)
	require.Len(t, iters, 2)
	require.Equal(t, 1, iters[0].IterationNum)
	require.Equal(t, 2, iters[1].IterationNum)
}

// TestReviewerResultWithClaudeMDRef tests YAML parsing with claude_md_ref.
func TestReviewerResultWithClaudeMDRef(t *testing.T) {
	response := "```yaml\n" + `decision: request_changes
summary: "CLAUDE.md compliance issue"
issues:
  - title: "Missing comment on exported function"
    type: style
    severity: medium
    file: "internal/review/sub_actor.go"
    line_start: 50
    line_end: 50
    description: "Exported function lacks godoc comment"
    claude_md_ref: "Every function and method must have a comment"
` + "```"

	result, err := ParseReviewerResponse(response)
	require.NoError(t, err)
	require.Len(t, result.Issues, 1)
	require.Equal(t,
		"Every function and method must have a comment",
		result.Issues[0].ClaudeMDRef,
	)
}

// TestDefaultSubActorSpawnConfig verifies defaults.
func TestDefaultSubActorSpawnConfig(t *testing.T) {
	cfg := DefaultSubActorSpawnConfig()
	require.Equal(t, "claude", cfg.CLIPath)
	require.Equal(t, 20, cfg.MaxTurns)
	require.True(t, cfg.AllowDangerouslySkipPermissions)
}

// TestNewSubActorManager verifies manager creation.
func TestNewSubActorManager(t *testing.T) {
	mockStore := store.NewMockStore()

	// With nil config - uses defaults.
	mgr := NewSubActorManager(mockStore, nil)
	require.NotNil(t, mgr)
	require.Equal(t, 0, mgr.ActiveCount())

	// With custom config.
	mgr2 := NewSubActorManager(mockStore, &SpawnConfig{
		MaxTurns: 10,
	})
	require.NotNil(t, mgr2)
}

// TestStopSubActors verifies the service cleanup method.
func TestStopSubActors(t *testing.T) {
	st, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(ServiceConfig{Store: st})

	// Insert some fake actors.
	svc.subActorMgr.mu.Lock()
	svc.subActorMgr.actors["r1"] = &reviewSubActor{
		reviewID: "r1",
	}
	svc.subActorMgr.actors["r2"] = &reviewSubActor{
		reviewID: "r2",
	}
	svc.subActorMgr.mu.Unlock()

	require.Equal(t, 2, svc.subActorMgr.ActiveCount())
	svc.StopSubActors()
	require.Equal(t, 0, svc.subActorMgr.ActiveCount())
}

// TestSubActorCallbackCleanup verifies that the manager callback removes the
// actor from tracking after completion.
func TestSubActorCallbackCleanup(t *testing.T) {
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(mockStore, &SpawnConfig{
		NoSessionPersistence: true,
	})

	var wg sync.WaitGroup
	var receivedResult *SubActorResult

	wg.Add(1)
	callback := func(_ context.Context, result *SubActorResult) {
		receivedResult = result
		wg.Done()
	}

	// Manually simulate the cleanup callback that would be set up by
	// SpawnReviewer.
	wrappedCallback := func(
		ctx context.Context, result *SubActorResult,
	) {
		mgr.mu.Lock()
		delete(mgr.actors, "test-review-cleanup")
		mgr.mu.Unlock()

		callback(ctx, result)
	}

	// Simulate actor being tracked.
	mgr.mu.Lock()
	mgr.actors["test-review-cleanup"] = &reviewSubActor{
		reviewID: "test-review-cleanup",
	}
	mgr.mu.Unlock()

	require.Equal(t, 1, mgr.ActiveCount())

	// Invoke the callback as if the actor completed.
	go wrappedCallback(context.Background(), &SubActorResult{
		ReviewID: "test-review-cleanup",
		Result: &ReviewerResult{
			Decision: "approve",
			Summary:  "Done",
		},
	})

	wg.Wait()

	require.Equal(t, 0, mgr.ActiveCount())
	require.NotNil(t, receivedResult)
	require.Equal(t, "approve", receivedResult.Result.Decision)
}

// TestBuildSystemPrompt_SpecializedReviewers tests prompts for each
// specialized reviewer type.
func TestBuildSystemPrompt_SpecializedReviewers(t *testing.T) {
	specialized := SpecializedReviewers()

	for name, config := range specialized {
		t.Run(name, func(t *testing.T) {
			actor := &reviewSubActor{
				reviewID: "test-1",
				config:   config,
			}

			prompt := actor.buildSystemPrompt()
			require.Contains(t, prompt, config.Name)

			for _, area := range config.FocusAreas {
				require.Contains(t, prompt, area)
			}
		})
	}
}

// TestServiceWithSubActorManager verifies the service initializes the
// sub-actor manager correctly.
func TestServiceWithSubActorManager(t *testing.T) {
	st, cleanup := testDB(t)
	defer cleanup()

	// Default config.
	svc := NewService(ServiceConfig{Store: st})
	require.NotNil(t, svc.subActorMgr)
	require.Equal(t, 0, svc.subActorMgr.ActiveCount())

	// With custom spawn config.
	svc2 := NewService(ServiceConfig{
		Store: st,
		SpawnConfig: &SpawnConfig{
			MaxTurns: 5,
		},
	})
	require.NotNil(t, svc2.subActorMgr)
}

// fmt is used in TestHandleSubActorResult_Error.
var _ = fmt.Errorf
