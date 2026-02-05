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

	// SpawnConfig overrides the default reviewer spawn configuration.
	// If nil, defaults are used.
	SpawnConfig *SpawnConfig

	// ActorSystem is used to register reviewer sub-actors for lifecycle
	// management and graceful shutdown.
	ActorSystem *actor.ActorSystem
}

// Service handles review orchestration as an actor. It creates DB records,
// manages the review FSM, spawns reviewer sub-actors via the Claude Agent
// SDK, and tracks active reviews.
type Service struct {
	store store.Storage

	// Registered reviewer configurations keyed by review type.
	reviewers map[string]*ReviewerConfig

	// subActorMgr manages spawned reviewer sub-actors.
	subActorMgr *SubActorManager

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
		store:     cfg.Store,
		reviewers: reviewers,
		subActorMgr: NewSubActorManager(
			cfg.ActorSystem, cfg.Store, cfg.SpawnConfig,
		),
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

	case DeleteReviewMsg:
		resp := s.handleDelete(ctx, m)
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

	// Track the active review before processing outbox so that
	// SpawnReviewerAgent can find the FSM.
	s.mu.Lock()
	s.activeReviews[reviewID] = fsm
	s.mu.Unlock()

	// Process outbox events (persist state, spawn reviewer, etc.).
	s.processOutbox(ctx, outbox)

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

// handleDelete permanently removes a review and all associated data from
// the database and active tracking.
func (s *Service) handleDelete(ctx context.Context,
	msg DeleteReviewMsg,
) DeleteReviewResp {
	// Remove from active tracking first.
	s.mu.Lock()
	delete(s.activeReviews, msg.ReviewID)
	s.mu.Unlock()

	// Delete from the database (cascades to iterations and issues).
	if err := s.store.DeleteReview(ctx, msg.ReviewID); err != nil {
		return DeleteReviewResp{Error: err}
	}

	return DeleteReviewResp{}
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
			// WebSocket broadcast.

		case SpawnReviewerAgent:
			s.spawnReviewer(ctx, e)

		case CreateReviewIteration:
			// Iteration records are persisted by the sub-actor
			// directly in persistResults().

		case CreateReviewIssues:
			// Issue records are persisted by the sub-actor
			// directly in persistResults().

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

// spawnReviewer launches a reviewer sub-actor for the given review. The
// sub-actor is registered with the actor system and receives a RunReviewMsg
// to kick off the Claude Agent SDK review process.
func (s *Service) spawnReviewer(ctx context.Context,
	e SpawnReviewerAgent,
) {
	// Look up the reviewer config based on the review type.
	s.mu.RLock()
	fsm, ok := s.activeReviews[e.ReviewID]
	s.mu.RUnlock()

	if !ok {
		log.WarnS(ctx, "Review service: no active FSM, skipping "+
			"spawn", nil,
			"review_id", e.ReviewID,
		)
		return
	}

	// Determine reviewer config from the review type stored in the DB.
	review, err := s.store.GetReview(ctx, e.ReviewID)
	if err != nil {
		log.ErrorS(ctx, "Review service: failed to get review "+
			"for spawn", err,
			"review_id", e.ReviewID,
		)
		return
	}

	config, ok := s.reviewers[review.ReviewType]
	if !ok {
		config = s.reviewers["full"]
	}

	log.InfoS(ctx, "Review service spawning reviewer sub-actor",
		"review_id", e.ReviewID,
		"review_type", review.ReviewType,
		"reviewer", config.Name,
		"repo_path", e.RepoPath,
	)

	// Transition FSM to under_review.
	outbox, err := fsm.ProcessEvent(ctx, StartReviewEvent{
		ReviewerID: config.Name,
	})
	if err != nil {
		log.ErrorS(ctx, "Review service: start review event "+
			"failed", err,
			"review_id", e.ReviewID,
		)
		return
	}
	s.processOutbox(ctx, outbox)

	// Spawn the sub-actor with branch info for the diff command.
	s.subActorMgr.SpawnReviewer(
		ctx, e.ReviewID, e.ThreadID, e.RepoPath,
		e.Requester,
		review.Branch, review.BaseBranch, review.CommitSHA,
		config, s.handleSubActorResult,
	)
}

// handleSubActorResult processes the outcome of a reviewer sub-actor run. It
// feeds the appropriate event into the review FSM based on the reviewer's
// decision.
func (s *Service) handleSubActorResult(ctx context.Context,
	result *SubActorResult,
) {
	if result.Error != nil {
		log.ErrorS(ctx, "Review sub-actor error",
			result.Error,
			"review_id", result.ReviewID,
			"duration", result.Duration.String(),
		)
		// On error, we don't transition the FSM - the review stays
		// in under_review and can be retried or cancelled.
		return
	}

	s.mu.RLock()
	fsm, ok := s.activeReviews[result.ReviewID]
	s.mu.RUnlock()

	if !ok {
		log.WarnS(ctx, "Review service: no active FSM after "+
			"sub-actor completion", nil,
			"review_id", result.ReviewID,
		)
		return
	}

	log.InfoS(ctx, "Review service processing sub-actor result",
		"review_id", result.ReviewID,
		"decision", result.Result.Decision,
		"issues_found", len(result.Result.Issues),
		"session_id", result.SessionID,
		"cost_usd", result.CostUSD,
		"duration", result.Duration.String(),
	)

	var event ReviewEvent
	switch result.Result.Decision {
	case "approve":
		event = ApproveEvent{
			ReviewerID: "reviewer-agent",
		}

	case "request_changes":
		issues := make(
			[]ReviewIssueSummary, len(result.Result.Issues),
		)
		for i, issue := range result.Result.Issues {
			issues[i] = ReviewIssueSummary{
				Title:    issue.Title,
				Severity: issue.Severity,
			}
		}
		event = RequestChangesEvent{
			ReviewerID: "reviewer-agent",
			Issues:     issues,
		}

	case "reject":
		event = RejectEvent{
			ReviewerID: "reviewer-agent",
			Reason:     result.Result.Summary,
		}

	default:
		log.WarnS(ctx, "Review sub-actor returned unknown "+
			"decision", nil,
			"review_id", result.ReviewID,
			"decision", result.Result.Decision,
		)
		return
	}

	outbox, err := fsm.ProcessEvent(ctx, event)
	if err != nil {
		log.ErrorS(ctx, "Review service: FSM event processing "+
			"failed", err,
			"review_id", result.ReviewID,
			"decision", result.Result.Decision,
		)
		return
	}

	s.processOutbox(ctx, outbox)

	// If the review reached a terminal state, remove from active.
	if fsm.IsTerminal() {
		log.InfoS(ctx, "Review reached terminal state, removing "+
			"from active tracking",
			"review_id", result.ReviewID,
			"final_state", fsm.CurrentState(),
		)
		s.mu.Lock()
		delete(s.activeReviews, result.ReviewID)
		s.mu.Unlock()
	}
}

// OnStop implements actor.Stoppable for graceful shutdown of reviewer
// sub-actors when the daemon stops. The actor system calls this after the
// message processing loop exits.
func (s *Service) OnStop(ctx context.Context) error {
	activeCount := s.subActorMgr.ActiveCount()
	log.InfoS(ctx, "Review service OnStop called, cleaning up "+
		"sub-actors",
		"active_reviewers", activeCount,
	)
	s.subActorMgr.StopAll()
	log.InfoS(ctx, "Review service OnStop completed")
	return nil
}

// Ensure Service implements the Stoppable interface at compile time.
var _ actor.Stoppable = (*Service)(nil)

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
