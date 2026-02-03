package review

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
)

// ReviewServiceKey is the service key for the review service actor.
var ReviewServiceKey = actor.NewServiceKey[ReviewRequest, ReviewResponse](
	"review-service",
)

// Ensure Service implements ActorBehavior.
var _ actor.ActorBehavior[ReviewRequest, ReviewResponse] = (*Service)(nil)

// ServiceConfig holds configuration for the review service.
type ServiceConfig struct {
	// Store is the storage backend for persisting reviews.
	Store store.Storage
}

// Service handles review orchestration as an actor. It creates DB records,
// manages the review FSM, and tracks active reviews. The Claude Agent SDK
// integration (sub-actor spawning) is handled separately in Task #8.
type Service struct {
	store store.Storage

	// Registered reviewer configurations keyed by review type.
	reviewers map[string]*ReviewerConfig

	// Active review FSMs, keyed by review ID. Protected by mu.
	mu            sync.RWMutex
	activeReviews map[string]*ReviewFSM
}

// NewService creates a new review service with the given configuration.
func NewService(cfg ServiceConfig) *Service {
	// Build reviewer config map with default + specialized.
	reviewers := map[string]*ReviewerConfig{
		"full": DefaultReviewerConfig(),
	}
	for name, cfg := range SpecializedReviewers() {
		reviewers[name] = cfg
	}

	return &Service{
		store:         cfg.Store,
		reviewers:     reviewers,
		activeReviews: make(map[string]*ReviewFSM),
	}
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (s *Service) Receive(ctx context.Context,
	msg ReviewRequest,
) fn.Result[ReviewResponse] {
	switch m := msg.(type) {
	case CreateReviewMsg:
		resp := s.handleCreateReview(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case GetReviewMsg:
		resp := s.handleGetReview(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case ListReviewsMsg:
		resp := s.handleListReviews(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case ResubmitMsg:
		resp := s.handleResubmit(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case CancelReviewMsg:
		resp := s.handleCancel(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case GetIssuesMsg:
		resp := s.handleGetIssues(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	case UpdateIssueMsg:
		resp := s.handleUpdateIssue(ctx, m)
		return fn.Ok[ReviewResponse](resp)

	default:
		return fn.Err[ReviewResponse](fmt.Errorf(
			"unknown message type: %T", msg,
		))
	}
}

// handleCreateReview creates a new review record and initializes the FSM.
func (s *Service) handleCreateReview(ctx context.Context,
	msg CreateReviewMsg,
) CreateReviewResp {
	reviewID := uuid.New().String()
	threadID := fmt.Sprintf("review-%s", reviewID[:8])

	// Determine review type, defaulting to "full".
	reviewType := msg.ReviewType
	if reviewType == "" {
		reviewType = "full"
	}
	priority := msg.Priority
	if priority == "" {
		priority = "normal"
	}

	// Create the review record in the database.
	review, err := s.store.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    reviewID,
		ThreadID:    threadID,
		RequesterID: msg.RequesterID,
		PRNumber:    msg.PRNumber,
		Branch:      msg.Branch,
		BaseBranch:  msg.BaseBranch,
		CommitSHA:   msg.CommitSHA,
		RepoPath:    msg.RepoPath,
		RemoteURL:   msg.RemoteURL,
		ReviewType:  reviewType,
		Priority:    priority,
	})
	if err != nil {
		return CreateReviewResp{Error: err}
	}

	// Create and initialize the FSM.
	fsm := NewReviewFSM(
		reviewID, threadID, msg.RepoPath, msg.RequesterID,
	)

	// Process the submit event to transition from new → pending_review.
	outbox, err := fsm.ProcessEvent(ctx, SubmitForReviewEvent{
		RequesterID: msg.RequesterID,
	})
	if err != nil {
		return CreateReviewResp{Error: err}
	}

	// Process outbox events (persist state, etc.).
	s.processOutbox(ctx, outbox)

	// Track the active review.
	s.mu.Lock()
	s.activeReviews[reviewID] = fsm
	s.mu.Unlock()

	return CreateReviewResp{
		ReviewID: review.ReviewID,
		ThreadID: review.ThreadID,
		State:    fsm.CurrentState(),
	}
}

// handleGetReview retrieves details for a specific review.
func (s *Service) handleGetReview(ctx context.Context,
	msg GetReviewMsg,
) GetReviewResp {
	review, err := s.store.GetReview(ctx, msg.ReviewID)
	if err != nil {
		return GetReviewResp{Error: err}
	}

	iters, err := s.store.GetReviewIterations(ctx, msg.ReviewID)
	if err != nil {
		return GetReviewResp{Error: err}
	}

	openIssues, err := s.store.CountOpenIssues(ctx, msg.ReviewID)
	if err != nil {
		return GetReviewResp{Error: err}
	}

	return GetReviewResp{
		ReviewID:   review.ReviewID,
		ThreadID:   review.ThreadID,
		State:      review.State,
		Branch:     review.Branch,
		BaseBranch: review.BaseBranch,
		ReviewType: review.ReviewType,
		Iterations: len(iters),
		OpenIssues: openIssues,
	}
}

// handleListReviews returns a list of reviews matching the given filters.
func (s *Service) handleListReviews(ctx context.Context,
	msg ListReviewsMsg,
) ListReviewsResp {
	limit := msg.Limit
	if limit == 0 {
		limit = 50
	}

	var (
		reviews []store.Review
		err     error
	)

	switch {
	case msg.State != "":
		reviews, err = s.store.ListReviewsByState(
			ctx, msg.State, limit,
		)
	case msg.RequesterID != 0:
		reviews, err = s.store.ListReviewsByRequester(
			ctx, msg.RequesterID, limit,
		)
	default:
		reviews, err = s.store.ListReviews(
			ctx, limit, msg.Offset,
		)
	}
	if err != nil {
		return ListReviewsResp{Error: err}
	}

	summaries := make([]ReviewSummary, len(reviews))
	for i, r := range reviews {
		summaries[i] = ReviewSummary{
			ReviewID:    r.ReviewID,
			ThreadID:    r.ThreadID,
			RequesterID: r.RequesterID,
			Branch:      r.Branch,
			State:       r.State,
			ReviewType:  r.ReviewType,
			CreatedAt:   r.CreatedAt.Unix(),
		}
	}

	return ListReviewsResp{Reviews: summaries}
}

// handleResubmit processes a review resubmission after author changes.
func (s *Service) handleResubmit(ctx context.Context,
	msg ResubmitMsg,
) ResubmitResp {
	s.mu.RLock()
	fsm, ok := s.activeReviews[msg.ReviewID]
	s.mu.RUnlock()

	if !ok {
		// Try to recover from DB.
		review, err := s.store.GetReview(ctx, msg.ReviewID)
		if err != nil {
			return ResubmitResp{Error: err}
		}
		fsm = NewReviewFSMFromDB(
			review.ReviewID, review.ThreadID,
			review.RepoPath, review.RequesterID, review.State,
		)
		s.mu.Lock()
		s.activeReviews[msg.ReviewID] = fsm
		s.mu.Unlock()
	}

	outbox, err := fsm.ProcessEvent(ctx, ResubmitEvent{
		NewCommitSHA: msg.CommitSHA,
	})
	if err != nil {
		return ResubmitResp{Error: err}
	}

	s.processOutbox(ctx, outbox)

	return ResubmitResp{
		ReviewID: msg.ReviewID,
		NewState: fsm.CurrentState(),
	}
}

// handleCancel cancels an active review.
func (s *Service) handleCancel(ctx context.Context,
	msg CancelReviewMsg,
) CancelReviewResp {
	s.mu.RLock()
	fsm, ok := s.activeReviews[msg.ReviewID]
	s.mu.RUnlock()

	if !ok {
		// Try to recover from DB.
		review, err := s.store.GetReview(ctx, msg.ReviewID)
		if err != nil {
			return CancelReviewResp{Error: err}
		}
		fsm = NewReviewFSMFromDB(
			review.ReviewID, review.ThreadID,
			review.RepoPath, review.RequesterID, review.State,
		)
	}

	outbox, err := fsm.ProcessEvent(ctx, CancelEvent{
		Reason: msg.Reason,
	})
	if err != nil {
		return CancelReviewResp{Error: err}
	}

	s.processOutbox(ctx, outbox)

	// Remove from active reviews since cancelled is terminal.
	s.mu.Lock()
	delete(s.activeReviews, msg.ReviewID)
	s.mu.Unlock()

	return CancelReviewResp{}
}

// handleGetIssues retrieves issues for a review.
func (s *Service) handleGetIssues(ctx context.Context,
	msg GetIssuesMsg,
) GetIssuesResp {
	issues, err := s.store.GetReviewIssues(ctx, msg.ReviewID)
	if err != nil {
		return GetIssuesResp{Error: err}
	}

	summaries := make([]IssueSummary, len(issues))
	for i, issue := range issues {
		summaries[i] = IssueSummary{
			ID:           issue.ID,
			ReviewID:     issue.ReviewID,
			IterationNum: issue.IterationNum,
			IssueType:    issue.IssueType,
			Severity:     issue.Severity,
			FilePath:     issue.FilePath,
			LineStart:    issue.LineStart,
			Title:        issue.Title,
			Status:       issue.Status,
		}
	}

	return GetIssuesResp{Issues: summaries}
}

// handleUpdateIssue updates the status of a review issue.
func (s *Service) handleUpdateIssue(ctx context.Context,
	msg UpdateIssueMsg,
) UpdateIssueResp {
	err := s.store.UpdateReviewIssueStatus(
		ctx, msg.IssueID, msg.Status, nil,
	)
	if err != nil {
		return UpdateIssueResp{Error: err}
	}

	return UpdateIssueResp{}
}

// processOutbox dispatches outbox events from the FSM to external systems.
func (s *Service) processOutbox(ctx context.Context,
	events []ReviewOutboxEvent,
) {
	for _, event := range events {
		switch e := event.(type) {
		case PersistReviewState:
			// Persist the new state to the database.
			if e.NewState == "approved" || e.NewState == "rejected" ||
				e.NewState == "cancelled" {

				_ = s.store.UpdateReviewCompleted(
					ctx, e.ReviewID, e.NewState,
				)
			} else {
				_ = s.store.UpdateReviewState(
					ctx, e.ReviewID, e.NewState,
				)
			}

		case NotifyReviewStateChange:
			// TODO(review): Send via notification hub for
			// WebSocket broadcast (Task #9).

		case SpawnReviewerAgent:
			// TODO(review): Spawn Claude Agent SDK client via
			// sub-actor (Task #8).

		case CreateReviewIteration:
			// TODO(review): Create iteration record from parsed
			// YAML frontmatter (Task #8).

		case CreateReviewIssues:
			// TODO(review): Create issue records from parsed
			// YAML frontmatter (Task #8).

		case RecordActivity:
			_ = s.store.CreateActivity(
				ctx, store.CreateActivityParams{
					AgentID:      e.AgentID,
					ActivityType: e.ActivityType,
					Description:  e.Description,
					Metadata: fmt.Sprintf(
						`{"review_id":"%s"}`,
						e.ReviewID,
					),
				},
			)
		}
	}
}

// RecoverActiveReviews loads active reviews from the database and restores
// their FSMs. Called on server startup for restart recovery.
func (s *Service) RecoverActiveReviews(ctx context.Context) error {
	reviews, err := s.store.ListActiveReviews(ctx)
	if err != nil {
		return fmt.Errorf("list active reviews: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, review := range reviews {
		fsm := NewReviewFSMFromDB(
			review.ReviewID, review.ThreadID,
			review.RepoPath, review.RequesterID, review.State,
		)
		s.activeReviews[review.ReviewID] = fsm
	}

	return nil
}

// ActiveReviewCount returns the number of active (non-terminal) reviews.
func (s *Service) ActiveReviewCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeReviews)
}
