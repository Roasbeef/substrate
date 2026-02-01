package review

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/roasbeef/subtrate/internal/store"
)

// Service handles review orchestration and can spawn structured analysis.
type Service struct {
	store store.Storage
	log   *slog.Logger

	// Registered reviewer configurations (for validation/routing)
	reviewers map[string]*ReviewerConfig

	// Active reviews being tracked (in-memory FSMs)
	mu            sync.RWMutex
	activeReviews map[string]*ReviewFSM

	// Default multi-reviewer config
	defaultConfig *MultiReviewConfig
}

// NewService creates a new review service.
func NewService(s store.Storage, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		store:         s,
		log:           log,
		reviewers:     SpecializedReviewers(),
		activeReviews: make(map[string]*ReviewFSM),
		defaultConfig: DefaultMultiReviewConfig(),
	}
}

// RequestReview creates a new review request and returns the review ID.
func (s *Service) RequestReview(ctx context.Context,
	req ReviewRequest) (string, error) {

	reviewID := uuid.New().String()
	threadID := req.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
	}

	// Convert PRNumber to pointer
	var prNum *int64
	if req.PRNumber > 0 {
		n := int64(req.PRNumber)
		prNum = &n
	}

	// Create review in database
	_, err := s.store.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    reviewID,
		ThreadID:    threadID,
		RequesterID: req.RequesterID,
		PRNumber:    prNum,
		Branch:      req.Branch,
		BaseBranch:  req.BaseBranch,
		CommitSHA:   req.CommitSHA,
		RepoPath:    req.RepoPath,
		ReviewType:  string(req.ReviewType),
		Priority:    string(req.Priority),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create review: %w", err)
	}

	// Create in-memory FSM
	s.mu.Lock()
	s.activeReviews[reviewID] = NewReviewFSM(reviewID, threadID, s.defaultConfig)
	s.mu.Unlock()

	// Transition to pending_review
	if err := s.transitionState(ctx, reviewID, SubmitForReviewEvent{
		RequesterID: req.RequesterID,
	}); err != nil {
		return "", fmt.Errorf("failed to transition state: %w", err)
	}

	s.log.Info("Review requested",
		"review_id", reviewID,
		"branch", req.Branch,
		"requester_id", req.RequesterID,
	)

	return reviewID, nil
}

// GetReview retrieves a review by ID.
func (s *Service) GetReview(ctx context.Context,
	reviewID string) (store.Review, error) {

	return s.store.GetReview(ctx, reviewID)
}

// ListPendingReviews returns reviews waiting for review.
func (s *Service) ListPendingReviews(ctx context.Context,
	limit int) ([]store.Review, error) {

	return s.store.ListPendingReviews(ctx, limit)
}

// ListActiveReviews returns reviews that are in progress.
func (s *Service) ListActiveReviews(ctx context.Context,
	limit int) ([]store.Review, error) {

	return s.store.ListActiveReviews(ctx, limit)
}

// StartReview marks a review as being actively reviewed.
func (s *Service) StartReview(ctx context.Context, reviewID,
	reviewerID string) error {

	return s.transitionState(ctx, reviewID, StartReviewEvent{
		ReviewerID: reviewerID,
	})
}

// SubmitReviewIteration records a review iteration from a reviewer.
func (s *Service) SubmitReviewIteration(ctx context.Context,
	resp ReviewResponse) error {

	// Get current iteration count
	iterCount, err := s.store.GetIterationCount(ctx, resp.ReviewID)
	if err != nil {
		return fmt.Errorf("failed to get iteration count: %w", err)
	}
	iterNum := iterCount + 1

	// Create iteration record
	_, err = s.store.CreateReviewIteration(ctx, store.CreateReviewIterationParams{
		ReviewID:      resp.ReviewID,
		IterationNum:  iterNum,
		ReviewerID:    resp.ReviewerName,
		Decision:      string(resp.Decision),
		Summary:       resp.Summary,
		FilesReviewed: resp.FilesReviewed,
		LinesAnalyzed: resp.LinesAnalyzed,
		DurationMS:    resp.DurationMS,
		CostUSD:       resp.CostUSD,
	})
	if err != nil {
		return fmt.Errorf("failed to create iteration: %w", err)
	}

	// Create issue records
	for _, issue := range resp.Issues {
		var lineEnd *int
		if issue.LineEnd > 0 {
			lineEnd = &issue.LineEnd
		}

		_, err := s.store.CreateReviewIssue(ctx, store.CreateReviewIssueParams{
			ReviewID:     resp.ReviewID,
			IterationNum: iterNum,
			IssueType:    string(issue.Type),
			Severity:     string(issue.Severity),
			FilePath:     issue.File,
			LineStart:    issue.LineStart,
			LineEnd:      lineEnd,
			Title:        issue.Title,
			Description:  issue.Description,
			CodeSnippet:  issue.CodeSnippet,
			Suggestion:   issue.Suggestion,
			ClaudeMDRef:  issue.ClaudeMDRef,
		})
		if err != nil {
			s.log.Warn("Failed to create issue record",
				"error", err,
				"review_id", resp.ReviewID,
				"issue_title", issue.Title,
			)
		}
	}

	// Transition state based on decision
	switch resp.Decision {
	case DecisionApprove:
		return s.transitionState(ctx, resp.ReviewID, ApproveEvent{
			ReviewerID: resp.ReviewerName,
		})
	case DecisionRequestChanges:
		return s.transitionState(ctx, resp.ReviewID, RequestChangesEvent{
			ReviewerID: resp.ReviewerName,
			Issues:     resp.Issues,
		})
	case DecisionComment:
		// Comments don't change state
		return nil
	default:
		return fmt.Errorf("unknown decision: %s", resp.Decision)
	}
}

// ResubmitForReview marks a review as resubmitted after changes.
func (s *Service) ResubmitForReview(ctx context.Context, reviewID,
	newCommitSHA string) error {

	// Update commit SHA
	if err := s.store.UpdateReviewCommit(ctx, reviewID, newCommitSHA); err != nil {
		return fmt.Errorf("failed to update commit: %w", err)
	}

	return s.transitionState(ctx, reviewID, ResubmitEvent{
		NewCommitSHA: newCommitSHA,
	})
}

// CancelReview cancels an active review.
func (s *Service) CancelReview(ctx context.Context, reviewID,
	reason string) error {

	return s.transitionState(ctx, reviewID, CancelEvent{
		Reason: reason,
	})
}

// GetReviewState returns the current state of a review.
func (s *Service) GetReviewState(ctx context.Context,
	reviewID string) (ReviewState, error) {

	review, err := s.store.GetReview(ctx, reviewID)
	if err != nil {
		return "", err
	}
	return ReviewState(review.State), nil
}

// GetReviewIterations returns all iterations for a review.
func (s *Service) GetReviewIterations(ctx context.Context,
	reviewID string) ([]store.ReviewIteration, error) {

	return s.store.ListReviewIterations(ctx, reviewID)
}

// GetOpenIssues returns open issues for a review.
func (s *Service) GetOpenIssues(ctx context.Context,
	reviewID string) ([]store.ReviewIssue, error) {

	return s.store.ListOpenReviewIssues(ctx, reviewID)
}

// ResolveIssue marks an issue as resolved.
func (s *Service) ResolveIssue(ctx context.Context, issueID int64) error {
	// Get current iteration
	// For now, use 0 as we'd need the review ID to get the actual iteration
	return s.store.ResolveIssue(ctx, issueID, 0)
}

// GetReviewStats returns aggregate review statistics.
func (s *Service) GetReviewStats(ctx context.Context) (store.ReviewStats, error) {
	return s.store.GetReviewStats(ctx)
}

// transitionState applies a state transition to a review.
func (s *Service) transitionState(ctx context.Context, reviewID string,
	event ReviewEvent) error {

	// Get or create FSM
	s.mu.Lock()
	fsm, exists := s.activeReviews[reviewID]
	if !exists {
		// Load from database
		review, err := s.store.GetReview(ctx, reviewID)
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("review not found: %w", err)
		}
		fsm = NewReviewFSM(reviewID, review.ThreadID, s.defaultConfig)
		fsm.CurrentState = ReviewState(review.State)
		s.activeReviews[reviewID] = fsm
	}
	s.mu.Unlock()

	// Process event
	newState, err := fsm.ProcessEvent(event)
	if err != nil {
		return err
	}

	// Persist state change
	if newState == StateApproved || newState == StateRejected ||
		newState == StateCancelled {
		return s.store.CompleteReview(ctx, reviewID, string(newState))
	}
	return s.store.UpdateReviewState(ctx, reviewID, string(newState))
}

// GetFSM returns the FSM for a review (for testing/debugging).
func (s *Service) GetFSM(reviewID string) *ReviewFSM {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeReviews[reviewID]
}

// RegisterReviewer registers a reviewer configuration.
func (s *Service) RegisterReviewer(config *ReviewerConfig) {
	s.reviewers[config.Name] = config
}

// GetReviewer returns a reviewer configuration by name.
func (s *Service) GetReviewer(name string) (*ReviewerConfig, bool) {
	config, ok := s.reviewers[name]
	return config, ok
}

// ListReviewers returns all registered reviewer names.
func (s *Service) ListReviewers() []string {
	names := make([]string, 0, len(s.reviewers))
	for name := range s.reviewers {
		names = append(names, name)
	}
	return names
}

// CleanupOldFSMs removes FSMs for completed reviews from memory.
func (s *Service) CleanupOldFSMs(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, fsm := range s.activeReviews {
		// Check if review is in a terminal state and old
		if fsm.CurrentState == StateApproved ||
			fsm.CurrentState == StateRejected ||
			fsm.CurrentState == StateCancelled {
			// Check last transition time
			if len(fsm.Transitions) > 0 {
				lastTransition := fsm.Transitions[len(fsm.Transitions)-1]
				if lastTransition.Timestamp.Before(cutoff) {
					delete(s.activeReviews, id)
				}
			}
		}
	}
}
