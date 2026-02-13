package review

import (
	"context"
	"testing"
)

// newTestFSM creates a ReviewFSM for testing with standard test values.
func newTestFSM() *ReviewFSM {
	return NewReviewFSM(
		"test-review-123", "thread-456", "/tmp/repo", 1,
	)
}

// TestFSM_HappyPath tests the full lifecycle: new → pending → under_review → approved.
func TestFSM_HappyPath(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Initial state should be new.
	if fsm.CurrentState() != "new" {
		t.Fatalf("expected state 'new', got %q", fsm.CurrentState())
	}
	if fsm.IsTerminal() {
		t.Fatal("new state should not be terminal")
	}

	// Submit for review: new → pending_review.
	outbox, err := fsm.ProcessEvent(ctx, SubmitForReviewEvent{
		RequesterID: 1,
	})
	if err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}
	if fsm.CurrentState() != "pending_review" {
		t.Fatalf("expected 'pending_review', got %q", fsm.CurrentState())
	}

	// Verify outbox events.
	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[SpawnReviewerAgent](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)

	// Start review: pending_review → under_review.
	_, err = fsm.ProcessEvent(ctx, StartReviewEvent{
		ReviewerID: "CodeReviewer",
	})
	if err != nil {
		t.Fatalf("StartReview failed: %v", err)
	}
	if fsm.CurrentState() != "under_review" {
		t.Fatalf("expected 'under_review', got %q", fsm.CurrentState())
	}

	// Approve: under_review → approved.
	outbox, err = fsm.ProcessEvent(ctx, ApproveEvent{
		ReviewerID: "CodeReviewer",
	})
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if fsm.CurrentState() != "approved" {
		t.Fatalf("expected 'approved', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("approved state should be terminal")
	}

	// Verify we got activity event for approval.
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ChangesRequested tests the iteration cycle: review → changes_requested → re_review → approved.
func TestFSM_ChangesRequested(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Get to under_review state.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "SecurityReviewer"})

	if fsm.CurrentState() != "under_review" {
		t.Fatalf("expected 'under_review', got %q", fsm.CurrentState())
	}

	// Request changes: under_review → changes_requested.
	issues := []ReviewIssueSummary{
		{Title: "SQL injection", Severity: "critical"},
		{Title: "Missing auth check", Severity: "high"},
	}
	outbox, err := fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "SecurityReviewer",
		Issues:     issues,
	})
	if err != nil {
		t.Fatalf("RequestChanges failed: %v", err)
	}
	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}
	assertHasOutboxEvent[CreateReviewIssues](t, outbox)

	// Resubmit: changes_requested → re_review.
	outbox, err = fsm.ProcessEvent(ctx, ResubmitEvent{
		NewCommitSHA: "abc123",
	})
	if err != nil {
		t.Fatalf("Resubmit failed: %v", err)
	}
	if fsm.CurrentState() != "re_review" {
		t.Fatalf("expected 're_review', got %q", fsm.CurrentState())
	}
	assertHasOutboxEvent[SendMailToReviewer](t, outbox)

	// Re-review starts: re_review → under_review.
	_, err = fsm.ProcessEvent(ctx, StartReviewEvent{
		ReviewerID: "SecurityReviewer",
	})
	if err != nil {
		t.Fatalf("StartReview (re-review) failed: %v", err)
	}
	if fsm.CurrentState() != "under_review" {
		t.Fatalf("expected 'under_review', got %q", fsm.CurrentState())
	}

	// Approve after fixes: under_review → approved.
	_, err = fsm.ProcessEvent(ctx, ApproveEvent{
		ReviewerID: "SecurityReviewer",
	})
	if err != nil {
		t.Fatalf("Approve (after fix) failed: %v", err)
	}
	if fsm.CurrentState() != "approved" {
		t.Fatalf("expected 'approved', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("approved should be terminal")
	}
}

// TestFSM_Rejection tests the rejection path: under_review → rejected.
func TestFSM_Rejection(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "Reviewer"})

	// Reject: under_review → rejected.
	outbox, err := fsm.ProcessEvent(ctx, RejectEvent{
		ReviewerID: "Reviewer",
		Reason:     "fundamental design flaw",
	})
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if fsm.CurrentState() != "rejected" {
		t.Fatalf("expected 'rejected', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("rejected should be terminal")
	}
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_CancelFromAnyState tests cancellation from every non-terminal state.
func TestFSM_CancelFromAnyState(t *testing.T) {
	ctx := context.Background()

	states := []struct {
		name  string
		setup func(*ReviewFSM)
	}{
		{
			name:  "new",
			setup: func(f *ReviewFSM) {},
		},
		{
			name: "pending_review",
			setup: func(f *ReviewFSM) {
				f.ProcessEvent(ctx, SubmitForReviewEvent{
					RequesterID: 1,
				})
			},
		},
		{
			name: "under_review",
			setup: func(f *ReviewFSM) {
				f.ProcessEvent(ctx, SubmitForReviewEvent{
					RequesterID: 1,
				})
				f.ProcessEvent(ctx, StartReviewEvent{
					ReviewerID: "R",
				})
			},
		},
		{
			name: "changes_requested",
			setup: func(f *ReviewFSM) {
				f.ProcessEvent(ctx, SubmitForReviewEvent{
					RequesterID: 1,
				})
				f.ProcessEvent(ctx, StartReviewEvent{
					ReviewerID: "R",
				})
				f.ProcessEvent(ctx, RequestChangesEvent{
					ReviewerID: "R",
				})
			},
		},
		{
			name: "re_review",
			setup: func(f *ReviewFSM) {
				f.ProcessEvent(ctx, SubmitForReviewEvent{
					RequesterID: 1,
				})
				f.ProcessEvent(ctx, StartReviewEvent{
					ReviewerID: "R",
				})
				f.ProcessEvent(ctx, RequestChangesEvent{
					ReviewerID: "R",
				})
				f.ProcessEvent(ctx, ResubmitEvent{
					NewCommitSHA: "abc",
				})
			},
		},
	}

	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			fsm := newTestFSM()
			tc.setup(fsm)

			if fsm.CurrentState() != tc.name {
				t.Fatalf(
					"setup failed: expected %q, got %q",
					tc.name, fsm.CurrentState(),
				)
			}

			outbox, err := fsm.ProcessEvent(ctx, CancelEvent{
				Reason: "no longer needed",
			})
			if err != nil {
				t.Fatalf("Cancel from %s failed: %v", tc.name, err)
			}
			if fsm.CurrentState() != "cancelled" {
				t.Fatalf(
					"expected 'cancelled', got %q",
					fsm.CurrentState(),
				)
			}
			if !fsm.IsTerminal() {
				t.Fatal("cancelled should be terminal")
			}
			assertHasOutboxEvent[PersistReviewState](t, outbox)
		})
	}
}

// TestFSM_TerminalStatesRejectEvents tests that terminal states reject all events.
func TestFSM_TerminalStatesRejectEvents(t *testing.T) {
	ctx := context.Background()

	terminalStates := []struct {
		name  string
		state ReviewState
	}{
		{"approved", &StateApproved{ReviewerID: "R"}},
		{"rejected", &StateRejected{ReviewerID: "R", Reason: "bad"}},
		{"cancelled", &StateCancelled{}},
	}

	events := []ReviewEvent{
		SubmitForReviewEvent{RequesterID: 1},
		StartReviewEvent{ReviewerID: "R"},
		RequestChangesEvent{ReviewerID: "R"},
		ResubmitEvent{NewCommitSHA: "abc"},
		ApproveEvent{ReviewerID: "R"},
		RejectEvent{ReviewerID: "R"},
		CancelEvent{Reason: "test"},
	}

	for _, ts := range terminalStates {
		for _, evt := range events {
			t.Run(ts.name, func(t *testing.T) {
				_, err := ts.state.ProcessEvent(
					ctx, evt, &ReviewEnvironment{
						ReviewID: "test",
					},
				)
				if err == nil {
					t.Fatalf(
						"expected error for %T in terminal "+
							"state %s",
						evt, ts.name,
					)
				}
			})
		}
	}
}

// TestFSM_InvalidTransitions tests that invalid events produce errors.
func TestFSM_InvalidTransitions(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name  string
		state ReviewState
		event ReviewEvent
	}{
		{
			name:  "approve in new",
			state: &StateNew{},
			event: ApproveEvent{ReviewerID: "R"},
		},
		{
			name:  "request_changes in new",
			state: &StateNew{},
			event: RequestChangesEvent{ReviewerID: "R"},
		},
		{
			name:  "resubmit in pending",
			state: &StatePendingReview{},
			event: ResubmitEvent{NewCommitSHA: "abc"},
		},
		{
			name:  "approve in pending",
			state: &StatePendingReview{},
			event: ApproveEvent{ReviewerID: "R"},
		},
		{
			name:  "resubmit in under_review",
			state: &StateUnderReview{},
			event: ResubmitEvent{NewCommitSHA: "abc"},
		},
		{
			name:  "resubmit in re_review",
			state: &StateReReview{},
			event: ResubmitEvent{NewCommitSHA: "abc"},
		},
	}

	env := &ReviewEnvironment{ReviewID: "test"}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.state.ProcessEvent(ctx, tc.event, env)
			if err == nil {
				t.Fatalf(
					"expected error for %T in state %s",
					tc.event, tc.state.String(),
				)
			}
		})
	}
}

// TestFSM_FromDB tests creating an FSM from a persisted state string.
func TestFSM_FromDB(t *testing.T) {
	states := []string{
		"new", "pending_review", "under_review",
		"changes_requested", "re_review",
		"approved", "rejected", "cancelled",
	}

	for _, s := range states {
		t.Run(s, func(t *testing.T) {
			fsm := NewReviewFSMFromDB(
				"review-1", "thread-1", "/repo", 1, s,
			)
			if fsm.CurrentState() != s {
				t.Fatalf(
					"expected state %q, got %q",
					s, fsm.CurrentState(),
				)
			}
		})
	}
}

// TestFSM_OutboxPersistStateContent verifies outbox event field values.
func TestFSM_OutboxPersistStateContent(t *testing.T) {
	ctx := context.Background()
	fsm := NewReviewFSM("r-42", "t-99", "/tmp/code", 7)

	outbox, err := fsm.ProcessEvent(ctx, SubmitForReviewEvent{
		RequesterID: 7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check PersistReviewState has correct fields.
	for _, evt := range outbox {
		if persist, ok := evt.(PersistReviewState); ok {
			if persist.ReviewID != "r-42" {
				t.Fatalf(
					"expected ReviewID 'r-42', got %q",
					persist.ReviewID,
				)
			}
			if persist.NewState != "pending_review" {
				t.Fatalf(
					"expected NewState 'pending_review', got %q",
					persist.NewState,
				)
			}
			return
		}
	}
	t.Fatal("PersistReviewState not found in outbox")
}

// TestFSM_ChangesRequestedDirectApprove tests the back-and-forth path where
// the reviewer approves directly from the changes_requested state without a
// formal resubmit cycle.
func TestFSM_ChangesRequestedDirectApprove(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to changes_requested.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Bug", Severity: "high"},
		},
	})

	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}

	// Approve directly from changes_requested.
	outbox, err := fsm.ProcessEvent(ctx, ApproveEvent{
		ReviewerID: "R",
	})
	if err != nil {
		t.Fatalf("Approve from changes_requested failed: %v", err)
	}
	if fsm.CurrentState() != "approved" {
		t.Fatalf("expected 'approved', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("approved state should be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ChangesRequestedDirectReject tests the path where the reviewer
// rejects directly from the changes_requested state.
func TestFSM_ChangesRequestedDirectReject(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to changes_requested.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Fatal flaw", Severity: "critical"},
		},
	})

	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}

	// Reject directly from changes_requested.
	outbox, err := fsm.ProcessEvent(ctx, RejectEvent{
		ReviewerID: "R",
		Reason:     "unfixable design issue",
	})
	if err != nil {
		t.Fatalf("Reject from changes_requested failed: %v", err)
	}
	if fsm.CurrentState() != "rejected" {
		t.Fatalf("expected 'rejected', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("rejected state should be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ChangesRequestedSecondRoundChanges tests that a reviewer can issue
// a second round of change requests while already in changes_requested state.
func TestFSM_ChangesRequestedSecondRoundChanges(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to changes_requested.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "First issue", Severity: "high"},
		},
	})

	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}

	// Issue another round of changes from changes_requested.
	newIssues := []ReviewIssueSummary{
		{Title: "Second issue", Severity: "medium"},
		{Title: "Third issue", Severity: "low"},
	}
	outbox, err := fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues:     newIssues,
	})
	if err != nil {
		t.Fatalf(
			"RequestChanges from changes_requested failed: %v",
			err,
		)
	}
	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}
	if fsm.IsTerminal() {
		t.Fatal("changes_requested should not be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[CreateReviewIssues](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ChangesRequestedOutboxContent verifies outbox event field values
// for the changes_requested → approved transition.
func TestFSM_ChangesRequestedOutboxContent(t *testing.T) {
	ctx := context.Background()
	fsm := NewReviewFSM("r-99", "t-88", "/tmp/code", 5)

	// Drive to changes_requested.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 5})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Bug", Severity: "high"},
		},
	})

	// Approve from changes_requested.
	outbox, err := fsm.ProcessEvent(ctx, ApproveEvent{
		ReviewerID: "R",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify PersistReviewState fields.
	var foundPersist bool
	for _, evt := range outbox {
		if persist, ok := evt.(PersistReviewState); ok {
			foundPersist = true
			if persist.ReviewID != "r-99" {
				t.Fatalf(
					"expected ReviewID 'r-99', got %q",
					persist.ReviewID,
				)
			}
			if persist.NewState != "approved" {
				t.Fatalf(
					"expected NewState 'approved', got %q",
					persist.NewState,
				)
			}
		}
	}
	if !foundPersist {
		t.Fatal("PersistReviewState not found in outbox")
	}

	// Verify NotifyReviewStateChange fields.
	var foundNotify bool
	for _, evt := range outbox {
		if notify, ok := evt.(NotifyReviewStateChange); ok {
			foundNotify = true
			if notify.OldState != "changes_requested" {
				t.Fatalf(
					"expected OldState "+
						"'changes_requested', got %q",
					notify.OldState,
				)
			}
			if notify.NewState != "approved" {
				t.Fatalf(
					"expected NewState 'approved', got %q",
					notify.NewState,
				)
			}
		}
	}
	if !foundNotify {
		t.Fatal("NotifyReviewStateChange not found in outbox")
	}
}

// TestStateFromString_Unknown tests that an unknown state falls back to New.
func TestStateFromString_Unknown(t *testing.T) {
	state := StateFromString("totally_unknown")
	if state.String() != "new" {
		t.Fatalf(
			"expected fallback to 'new', got %q", state.String(),
		)
	}
}

// TestFSM_ReReviewDirectApprove tests that a reviewer can approve directly
// from the re_review state during a back-and-forth session (via stop hook).
func TestFSM_ReReviewDirectApprove(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to re_review.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Bug", Severity: "high"},
		},
	})
	_, _ = fsm.ProcessEvent(ctx, ResubmitEvent{NewCommitSHA: "def456"})

	if fsm.CurrentState() != "re_review" {
		t.Fatalf(
			"expected 're_review', got %q", fsm.CurrentState(),
		)
	}

	// Approve directly from re_review.
	outbox, err := fsm.ProcessEvent(ctx, ApproveEvent{
		ReviewerID: "R",
	})
	if err != nil {
		t.Fatalf("Approve from re_review failed: %v", err)
	}
	if fsm.CurrentState() != "approved" {
		t.Fatalf("expected 'approved', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("approved state should be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)

	// Verify the old state is re_review in the notification.
	for _, evt := range outbox {
		if notify, ok := evt.(NotifyReviewStateChange); ok {
			if notify.OldState != "re_review" {
				t.Fatalf(
					"expected OldState 're_review', got %q",
					notify.OldState,
				)
			}
		}
	}
}

// TestFSM_ReReviewRequestChanges tests that a reviewer can request further
// changes from the re_review state.
func TestFSM_ReReviewRequestChanges(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to re_review.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "First issue", Severity: "high"},
		},
	})
	_, _ = fsm.ProcessEvent(ctx, ResubmitEvent{NewCommitSHA: "def456"})

	if fsm.CurrentState() != "re_review" {
		t.Fatalf(
			"expected 're_review', got %q", fsm.CurrentState(),
		)
	}

	// Request more changes from re_review.
	newIssues := []ReviewIssueSummary{
		{Title: "Still broken", Severity: "medium"},
	}
	outbox, err := fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues:     newIssues,
	})
	if err != nil {
		t.Fatalf(
			"RequestChanges from re_review failed: %v", err,
		)
	}
	if fsm.CurrentState() != "changes_requested" {
		t.Fatalf(
			"expected 'changes_requested', got %q",
			fsm.CurrentState(),
		)
	}
	if fsm.IsTerminal() {
		t.Fatal("changes_requested should not be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[CreateReviewIssues](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ReReviewReject tests that a reviewer can reject from the re_review
// state.
func TestFSM_ReReviewReject(t *testing.T) {
	ctx := context.Background()
	fsm := newTestFSM()

	// Drive to re_review.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Critical flaw", Severity: "critical"},
		},
	})
	_, _ = fsm.ProcessEvent(ctx, ResubmitEvent{NewCommitSHA: "def456"})

	if fsm.CurrentState() != "re_review" {
		t.Fatalf(
			"expected 're_review', got %q", fsm.CurrentState(),
		)
	}

	// Reject from re_review.
	outbox, err := fsm.ProcessEvent(ctx, RejectEvent{
		ReviewerID: "R",
		Reason:     "fundamental issue persists",
	})
	if err != nil {
		t.Fatalf("Reject from re_review failed: %v", err)
	}
	if fsm.CurrentState() != "rejected" {
		t.Fatalf("expected 'rejected', got %q", fsm.CurrentState())
	}
	if !fsm.IsTerminal() {
		t.Fatal("rejected state should be terminal")
	}

	assertHasOutboxEvent[PersistReviewState](t, outbox)
	assertHasOutboxEvent[NotifyReviewStateChange](t, outbox)
	assertHasOutboxEvent[RecordActivity](t, outbox)
}

// TestFSM_ResubmitEmitsSendMail tests that the resubmit transition emits
// SendMailToReviewer instead of SpawnReviewerAgent.
func TestFSM_ResubmitEmitsSendMail(t *testing.T) {
	ctx := context.Background()
	fsm := NewReviewFSM("r-mail", "t-mail", "/repo", 1)

	// Drive to changes_requested.
	_, _ = fsm.ProcessEvent(ctx, SubmitForReviewEvent{RequesterID: 1})
	_, _ = fsm.ProcessEvent(ctx, StartReviewEvent{ReviewerID: "R"})
	_, _ = fsm.ProcessEvent(ctx, RequestChangesEvent{
		ReviewerID: "R",
		Issues: []ReviewIssueSummary{
			{Title: "Issue", Severity: "high"},
		},
	})

	// Resubmit should emit SendMailToReviewer.
	outbox, err := fsm.ProcessEvent(ctx, ResubmitEvent{
		NewCommitSHA: "abc123",
	})
	if err != nil {
		t.Fatalf("Resubmit failed: %v", err)
	}

	assertHasOutboxEvent[SendMailToReviewer](t, outbox)

	// Verify it does NOT emit SpawnReviewerAgent.
	for _, evt := range outbox {
		if _, ok := evt.(SpawnReviewerAgent); ok {
			t.Fatal(
				"resubmit should not emit SpawnReviewerAgent",
			)
		}
	}

	// Verify SendMailToReviewer fields.
	for _, evt := range outbox {
		if mail, ok := evt.(SendMailToReviewer); ok {
			if mail.ReviewID != "r-mail" {
				t.Fatalf(
					"expected ReviewID 'r-mail', got %q",
					mail.ReviewID,
				)
			}
			if mail.ThreadID != "t-mail" {
				t.Fatalf(
					"expected ThreadID 't-mail', got %q",
					mail.ThreadID,
				)
			}
			if mail.RepoPath != "/repo" {
				t.Fatalf(
					"expected RepoPath '/repo', got %q",
					mail.RepoPath,
				)
			}
			if mail.Message == "" {
				t.Fatal("expected non-empty Message")
			}
			return
		}
	}
	t.Fatal("SendMailToReviewer not found in outbox")
}

// assertHasOutboxEvent checks that at least one outbox event matches the
// given type.
func assertHasOutboxEvent[T ReviewOutboxEvent](
	t *testing.T, events []ReviewOutboxEvent,
) {
	t.Helper()
	for _, evt := range events {
		if _, ok := evt.(T); ok {
			return
		}
	}
	t.Fatalf("expected outbox event of type %T not found", *new(T))
}
