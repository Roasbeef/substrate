package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
)

// newTestActorSystem creates an actor system for testing and registers a
// cleanup function that shuts the system down with a short timeout.
func newTestActorSystem(t *testing.T) *actor.ActorSystem {
	t.Helper()
	as := actor.NewActorSystem()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		as.Shutdown(shutdownCtx)
	})
	return as
}

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

// TestBuildReviewPrompt verifies the user prompt content with branch info.
func TestBuildReviewPrompt(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		baseBranch string
		commitSHA  string
		expected   string
	}{
		{
			name:       "full branch info",
			branch:     "feature/auth",
			baseBranch: "main",
			expected:   "git diff main...feature/auth",
		},
		{
			name:       "base branch only",
			baseBranch: "develop",
			expected:   "git diff develop...HEAD",
		},
		{
			name:      "commit SHA only",
			commitSHA: "abc123def",
			expected:  "git show abc123def",
		},
		{
			name:     "fallback no info",
			expected: "git diff HEAD~1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &reviewSubActor{
				reviewID:   "abc-123",
				branch:     tt.branch,
				baseBranch: tt.baseBranch,
				commitSHA:  tt.commitSHA,
				config:     DefaultReviewerConfig(),
			}

			prompt := a.buildReviewPrompt()
			require.Contains(t, prompt, "abc-123")
			require.Contains(t, prompt, tt.expected)
			require.Contains(t, prompt, "YAML frontmatter")
		})
	}
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

	opts, configDir := actor.buildClientOptions()

	// We can't inspect the options directly, but we can verify that
	// buildClientOptions doesn't panic and returns a non-empty slice.
	require.NotEmpty(t, opts)

	// Clean up the temp config dir if one was created.
	if configDir != "" {
		os.RemoveAll(filepath.Dir(configDir))
	}
}

// TestSubActorManager_SpawnAndStop tests the manager lifecycle.
func TestSubActorManager_SpawnAndStop(t *testing.T) {
	as := newTestActorSystem(t)
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(as, mockStore, &SpawnConfig{
		NoSessionPersistence: true,
	})

	require.Equal(t, 0, mgr.ActiveCount())

	// We can't actually spawn a real Claude agent in tests, but we can
	// verify the manager tracks actors correctly by using the internal
	// structure.
	mgr.mu.Lock()
	mgr.actorIDs["test-review-1"] = "reviewer-test-review-1"
	mgr.mu.Unlock()

	require.Equal(t, 1, mgr.ActiveCount())

	// Stop a specific reviewer (StopAndRemoveActor will be a no-op since
	// the actor wasn't actually registered with the system, but the
	// manager's own tracking should be cleaned up).
	mgr.StopReviewer("test-review-1")
	require.Equal(t, 0, mgr.ActiveCount())

	// StopAll with multiple actors.
	mgr.mu.Lock()
	mgr.actorIDs["review-a"] = "reviewer-review-a"
	mgr.actorIDs["review-b"] = "reviewer-review-b"
	mgr.mu.Unlock()

	require.Equal(t, 2, mgr.ActiveCount())
	mgr.StopAll()
	require.Equal(t, 0, mgr.ActiveCount())
}

// TestSubActorManager_DuplicateSpawnPrevented verifies that duplicate spawns
// for the same review ID are prevented.
func TestSubActorManager_DuplicateSpawnPrevented(t *testing.T) {
	as := newTestActorSystem(t)
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(as, mockStore, &SpawnConfig{
		NoSessionPersistence: true,
	})

	// Manually insert an actor ID to simulate an active review.
	mgr.mu.Lock()
	mgr.actorIDs["test-review-1"] = "reviewer-test-review-1"
	mgr.mu.Unlock()

	callCount := 0
	callback := func(_ context.Context, _ *SubActorResult) {
		callCount++
	}

	// Attempt to spawn a duplicate - should be a no-op.
	mgr.SpawnReviewer(
		context.Background(),
		"test-review-1", "thread-1", "/tmp/repo",
		1, "feature-branch", "main", "abc123",
		DefaultReviewerConfig(), callback,
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
	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})

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
	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})

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
	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})

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

	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})

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
}

// TestNewSubActorManager verifies manager creation.
func TestNewSubActorManager(t *testing.T) {
	as := newTestActorSystem(t)
	mockStore := store.NewMockStore()

	// With nil config - uses defaults.
	mgr := NewSubActorManager(as, mockStore, nil)
	require.NotNil(t, mgr)
	require.Equal(t, 0, mgr.ActiveCount())

	// With custom config.
	mgr2 := NewSubActorManager(as, mockStore, &SpawnConfig{
		MaxTurns: 10,
	})
	require.NotNil(t, mgr2)
}

// TestOnStop verifies the service OnStop cleanup method.
func TestOnStop(t *testing.T) {
	as := newTestActorSystem(t)
	st, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: as,
	})

	// Insert some fake actor IDs.
	svc.subActorMgr.mu.Lock()
	svc.subActorMgr.actorIDs["r1"] = "reviewer-r1"
	svc.subActorMgr.actorIDs["r2"] = "reviewer-r2"
	svc.subActorMgr.mu.Unlock()

	require.Equal(t, 2, svc.subActorMgr.ActiveCount())
	err := svc.OnStop(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, svc.subActorMgr.ActiveCount())
}

// TestSubActorCallbackCleanup verifies that the manager callback removes the
// actor from tracking after completion.
func TestSubActorCallbackCleanup(t *testing.T) {
	as := newTestActorSystem(t)
	mockStore := store.NewMockStore()
	mgr := NewSubActorManager(as, mockStore, &SpawnConfig{
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
		delete(mgr.actorIDs, "test-review-cleanup")
		mgr.mu.Unlock()

		callback(ctx, result)
	}

	// Simulate actor being tracked.
	mgr.mu.Lock()
	mgr.actorIDs["test-review-cleanup"] = "reviewer-test-review-cleanup"
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
	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})
	require.NotNil(t, svc.subActorMgr)
	require.Equal(t, 0, svc.subActorMgr.ActiveCount())

	// With custom spawn config.
	svc2 := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
		SpawnConfig: &SpawnConfig{
			MaxTurns: 5,
		},
	})
	require.NotNil(t, svc2.subActorMgr)
}

// testWritePrefixes is the allowed write prefixes used in permission
// tests.
var testWritePrefixes = []string{
	"/tmp/substrate_reviews/",
	"/private/tmp/substrate_reviews/",
}

// TestReviewerPermissionPolicy_ReadOnlyTools tests that read-only tools are
// allowed by the permission policy.
func TestReviewerPermissionPolicy_ReadOnlyTools(t *testing.T) {
	allowedTools := []string{
		"Read", "Glob", "Grep", "LS",
		"WebFetch", "WebSearch", "NotebookRead",
	}

	for _, tool := range allowedTools {
		t.Run(tool, func(t *testing.T) {
			result := reviewerPermissionPolicy(
				claudeagent.ToolPermissionRequest{
					ToolName:  tool,
					Arguments: []byte(`{}`),
				},
				testWritePrefixes,
			)
			require.True(t, result.IsAllow(),
				"tool %q should be allowed", tool,
			)
		})
	}
}

// TestReviewerPermissionPolicy_WriteToolsDenied tests that write tools
// (other than Write to .reviews/) are denied by the permission policy.
func TestReviewerPermissionPolicy_WriteToolsDenied(t *testing.T) {
	// Write with empty args should be denied (no valid path).
	deniedTools := []string{
		"Edit", "MultiEdit",
		"NotebookEdit", "Task", "TodoWrite",
	}

	for _, tool := range deniedTools {
		t.Run(tool, func(t *testing.T) {
			result := reviewerPermissionPolicy(
				claudeagent.ToolPermissionRequest{
					ToolName:  tool,
					Arguments: []byte(`{}`),
				},
				testWritePrefixes,
			)
			require.False(t, result.IsAllow(),
				"tool %q should be denied", tool,
			)
		})
	}

	// Write with empty/invalid path should be denied.
	t.Run("Write with empty args", func(t *testing.T) {
		result := reviewerPermissionPolicy(
			claudeagent.ToolPermissionRequest{
				ToolName:  "Write",
				Arguments: []byte(`{}`),
			},
			testWritePrefixes,
		)
		require.False(t, result.IsAllow(),
			"Write with empty path should be denied",
		)
	})
}

// TestReviewerPermissionPolicy_BashReadOnly tests that safe Bash commands
// are allowed while destructive ones are denied.
func TestReviewerPermissionPolicy_BashReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		command string
		allowed bool
	}{
		{"git diff", `{"command":"git diff HEAD~1"}`, true},
		{"git log", `{"command":"git log --oneline -10"}`, true},
		{"git show", `{"command":"git show HEAD"}`, true},
		{"cat file", `{"command":"cat main.go"}`, true},
		{"ls dir", `{"command":"ls -la internal/"}`, true},
		{"go doc", `{"command":"go doc fmt.Println"}`, true},
		{"rm file", `{"command":"rm main.go"}`, false},
		{"git push", `{"command":"git push origin main"}`, false},
		{"git commit", `{"command":"git commit -m 'bad'"}`, false},
		{"git add", `{"command":"git add ."}`, false},
		{"git checkout", `{"command":"git checkout main"}`, false},
		{"make", `{"command":"make build"}`, false},
		{"redirect", `{"command":"echo bad > file.txt"}`, false},
		{"redirect denied", `{"command":"echo review > /tmp/claude/review.md"}`, false},
		{"redirect to project denied", `{"command":"echo review > review.md"}`, false},
		{"stderr redirect ok", `{"command":"git diff 2>&1"}`, true},
		{"chained rm via semicolon", `{"command":"git log; rm -rf /"}`, false},
		{"chained rm via &&", `{"command":"git log && rm file"}`, false},
		{"chained rm via ||", `{"command":"git log || rm file"}`, false},
		{"chained rm via pipe", `{"command":"echo x | rm file"}`, false},
		{"subshell $() denied", `{"command":"echo $(rm file)"}`, false},
		{"backtick subshell denied", "{ \"command\": \"echo `rm file`\"}", false},
		{"process substitution denied", `{"command":"cat <(rm file)"}`, false},
		{"chained git push via semicolon", `{"command":"echo hi; git push origin main"}`, false},
		{"chained make via &&", `{"command":"git diff && make build"}`, false},
		{"multiple safe chained", `{"command":"git diff 2>&1"}`, true},
		{"env denied", `{"command":"env | grep TOKEN"}`, false},
		{"printenv denied", `{"command":"printenv CLAUDE_CODE_OAUTH_TOKEN"}`, false},
		{"export denied", `{"command":"export -p"}`, false},
		{"set denied", `{"command":"set | grep API"}`, false},
		{"chained env via semicolon", `{"command":"git log; env"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reviewerPermissionPolicy(
				claudeagent.ToolPermissionRequest{
					ToolName:  "Bash",
					Arguments: []byte(tt.command),
				},
				testWritePrefixes,
			)
			if tt.allowed {
				require.True(t, result.IsAllow(),
					"command should be allowed",
				)
			} else {
				require.False(t, result.IsAllow(),
					"command should be denied",
				)
			}
		})
	}
}

// TestReviewerPermissionPolicy_UnknownToolDenied tests that unknown tools
// are denied for safety.
func TestReviewerPermissionPolicy_UnknownToolDenied(t *testing.T) {
	result := reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "SomeNewTool",
			Arguments: []byte(`{}`),
		},
		testWritePrefixes,
	)
	require.False(t, result.IsAllow())
}

// TestReviewerPermissionPolicy_SubstrateAllowed tests that substrate CLI
// commands pass the permission policy for inter-agent messaging.
func TestReviewerPermissionPolicy_SubstrateAllowed(t *testing.T) {
	substrateCmds := []string{
		`{"command":"substrate send --to User --subject \"Review\" --body \"done\""}`,
		`{"command":"substrate inbox"}`,
		`{"command":"substrate status"}`,
		`{"command":"substrate heartbeat"}`,
	}

	for _, cmd := range substrateCmds {
		t.Run(cmd, func(t *testing.T) {
			result := reviewerPermissionPolicy(
				claudeagent.ToolPermissionRequest{
					ToolName:  "Bash",
					Arguments: []byte(cmd),
				},
				testWritePrefixes,
			)
			require.True(t, result.IsAllow(),
				"substrate command should be allowed",
			)
		})
	}
}

// TestReviewerPermissionPolicy_WriteReviewsAllowed tests that the Write
// tool is allowed for /tmp/substrate_reviews/ paths but denied elsewhere.
func TestReviewerPermissionPolicy_WriteReviewsAllowed(t *testing.T) {
	// Write to /tmp/substrate_reviews/ should be allowed.
	result := reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Write",
			Arguments: []byte(`{"file_path":"/tmp/substrate_reviews/review-abc123.md"}`),
		},
		testWritePrefixes,
	)
	require.True(t, result.IsAllow(),
		"Write to /tmp/substrate_reviews/ should be allowed",
	)

	// Write to /private/tmp/substrate_reviews/ should also be
	// allowed (macOS symlink resolution).
	result = reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Write",
			Arguments: []byte(`{"file_path":"/private/tmp/substrate_reviews/review-abc123.md"}`),
		},
		testWritePrefixes,
	)
	require.True(t, result.IsAllow(),
		"Write to /private/tmp/substrate_reviews/ should be allowed",
	)

	// Write to project root should be denied.
	result = reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Write",
			Arguments: []byte(`{"file_path":"/test/repo/evil.go"}`),
		},
		testWritePrefixes,
	)
	require.False(t, result.IsAllow(),
		"Write to project root should be denied",
	)

	// Write with path traversal should be denied.
	result = reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Write",
			Arguments: []byte(`{"file_path":"/tmp/substrate_reviews/../evil.go"}`),
		},
		testWritePrefixes,
	)
	require.False(t, result.IsAllow(),
		"Write with path traversal should be denied",
	)

	// Write to /tmp/claude/ should be denied (wrong subdir).
	result = reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Write",
			Arguments: []byte(`{"file_path":"/tmp/claude/review.md"}`),
		},
		testWritePrefixes,
	)
	require.False(t, result.IsAllow(),
		"Write to /tmp/claude/ should be denied",
	)

	// Edit tool should still be denied even for review paths.
	result = reviewerPermissionPolicy(
		claudeagent.ToolPermissionRequest{
			ToolName:  "Edit",
			Arguments: []byte(`{"file_path":"/tmp/substrate_reviews/foo.md"}`),
		},
		testWritePrefixes,
	)
	require.False(t, result.IsAllow(),
		"Edit tool should be denied even for review paths",
	)
}

// TestReviewerAgentName verifies branch-based agent naming.
func TestReviewerAgentName(t *testing.T) {
	// Simple branch name.
	actor := &reviewSubActor{
		branch: "feature-x",
		config: &ReviewerConfig{Name: "CodeReviewer"},
	}

	name := actor.reviewerAgentName()
	require.Equal(t, "reviewer-feature-x", name)

	// Branch with slashes should be sanitized.
	actor2 := &reviewSubActor{
		branch: "feature/auth/login",
		config: &ReviewerConfig{Name: "SecurityReviewer"},
	}
	require.Equal(t,
		"reviewer-feature-auth-login",
		actor2.reviewerAgentName(),
	)

	// Empty branch should default to "unknown".
	actor3 := &reviewSubActor{
		branch: "",
		config: &ReviewerConfig{Name: "CodeReviewer"},
	}
	require.Equal(t, "reviewer-unknown", actor3.reviewerAgentName())
}

// TestBuildSubstrateHooks verifies the hook map is constructed correctly.
func TestBuildSubstrateHooks(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-hooks",
		config:   DefaultReviewerConfig(),
		store:    store.NewMockStore(),
	}

	hooks := actor.buildSubstrateHooks()

	// Should have SessionStart and Stop hooks.
	require.Contains(t, hooks, claudeagent.HookTypeSessionStart)
	require.Contains(t, hooks, claudeagent.HookTypeStop)
	require.Len(t, hooks[claudeagent.HookTypeSessionStart], 1)
	require.Len(t, hooks[claudeagent.HookTypeStop], 1)
}

// TestHookSessionStart_RegistersAgent tests that the session start hook
// creates an agent identity in the store.
func TestHookSessionStart_RegistersAgent(t *testing.T) {
	mockStore := store.NewMockStore()
	actor := &reviewSubActor{
		reviewID: "hook-test-1234",
		config:   &ReviewerConfig{Name: "full"},
		store:    mockStore,
		branch:   "feature-branch",
	}

	result, err := actor.hookSessionStart(
		context.Background(),
		claudeagent.SessionStartInput{
			BaseHookInput: claudeagent.BaseHookInput{
				SessionID: "test-session",
			},
			Source: "startup",
		},
	)

	require.NoError(t, err)
	require.True(t, result.Continue)

	// The agent should have been registered.
	require.NotZero(t, actor.agentID)

	// Verify agent exists in the store.
	agentName := actor.reviewerAgentName()
	agent, err := mockStore.GetAgentByName(
		context.Background(), agentName,
	)
	require.NoError(t, err)
	require.Equal(t, agentName, agent.Name)
}

// TestHookStop_NoAgentID tests that the stop hook approves exit when there
// is no agent ID (agent wasn't registered).
func TestHookStop_NoAgentID(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "hook-stop-test",
		config:   DefaultReviewerConfig(),
		store:    store.NewMockStore(),
		agentID:  0,
	}

	result, err := actor.hookStop(
		context.Background(),
		claudeagent.StopInput{},
	)

	require.NoError(t, err)
	require.Equal(t, "approve", result.Decision)
}

// TestHookStop_NoMessages tests that the stop hook eventually approves
// exit when there are no unread messages (using a cancelled context to
// avoid the full poll timeout).
func TestHookStop_NoMessages(t *testing.T) {
	mockStore := store.NewMockStore()

	// Create an agent in the store with branch-based naming.
	agent, err := mockStore.CreateAgent(
		context.Background(),
		store.CreateAgentParams{Name: "reviewer-hookstop-branch"},
	)
	require.NoError(t, err)

	actor := &reviewSubActor{
		branch:  "hookstop-branch",
		config:  DefaultReviewerConfig(),
		store:   mockStore,
		agentID: agent.ID,
	}

	// Use a cancelled context to exit the poll loop immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := actor.hookStop(ctx, claudeagent.StopInput{})

	require.NoError(t, err)
	require.Equal(t, "approve", result.Decision)
}

// TestFormatMailAsPrompt tests formatting inbox messages for injection.
func TestFormatMailAsPrompt(t *testing.T) {
	msgs := []store.InboxMessage{
		{
			Message: store.Message{
				Subject: "Re-review requested",
				Body:    "I fixed the issues, please review again.",
			},
			SenderName: "dev-agent",
		},
	}

	prompt := formatMailAsPrompt(msgs)
	require.Contains(t, prompt, "dev-agent")
	require.Contains(t, prompt, "Re-review requested")
	require.Contains(t, prompt, "I fixed the issues")
	require.Contains(t, prompt, "Message 1")
}

// TestBuildSystemPrompt_SubstrateSection verifies the system prompt
// includes substrate messaging instructions.
func TestBuildSystemPrompt_SubstrateSection(t *testing.T) {
	actor := &reviewSubActor{
		reviewID: "test-substrate-prompt",
		branch:   "feature-test",
		config:   DefaultReviewerConfig(),
		store:    store.NewMockStore(),
	}

	prompt := actor.buildSystemPrompt()
	require.Contains(t, prompt, "Substrate Messaging")
	require.Contains(t, prompt, "substrate send")
	require.Contains(t, prompt, "--body-file")
	require.Contains(t, prompt, "/tmp/substrate_reviews/review-")
	require.Contains(t, prompt, "Write tool")
	require.Contains(t, prompt, actor.reviewerAgentName())
}

// TestIsAuthError verifies auth error pattern detection in assistant text.
func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		expect bool
	}{
		{
			name:   "exact invalid API key",
			text:   "Invalid API key · Please run /login",
			expect: true,
		},
		{
			name:   "lowercase unauthorized",
			text:   "Error: Unauthorized access",
			expect: true,
		},
		{
			name:   "expired token",
			text:   "Authentication failed: expired token",
			expect: true,
		},
		{
			name:   "invalid_api_key code",
			text:   "error code: invalid_api_key",
			expect: true,
		},
		{
			name:   "normal assistant text",
			text:   "I'll review this code now.",
			expect: false,
		},
		{
			name:   "empty string",
			text:   "",
			expect: false,
		},
		{
			name:   "partial match not auth",
			text:   "The API key rotation feature looks good.",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, isAuthError(tt.text))
		})
	}
}

// TestHandleSubActorResult_AuthError tests that auth errors auto-cancel
// the review instead of leaving it stuck in under_review.
func TestHandleSubActorResult_AuthError(t *testing.T) {
	ctx := context.Background()
	st, cleanup := testDB(t)
	defer cleanup()

	agent := createTestAgent(t, st, "AuthErrorTest")
	svc := NewService(ServiceConfig{
		Store:       st,
		ActorSystem: newTestActorSystem(t),
	})

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

	// Simulate an auth error result.
	svc.handleSubActorResult(ctx, &SubActorResult{
		ReviewID: reviewID,
		Error: fmt.Errorf(
			"auth error: Invalid API key · " +
				"Please run /login",
		),
	})

	// The review should have been auto-cancelled.
	require.Equal(t, "cancelled", fsm.CurrentState())
}

// fmt is used in TestHandleSubActorResult_Error.
var _ = fmt.Errorf
