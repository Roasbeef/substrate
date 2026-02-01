package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// =============================================================================
// ReviewStore implementation for SqlcStore
// =============================================================================

// CreateReview creates a new review in the database.
func (s *SqlcStore) CreateReview(ctx context.Context,
	params CreateReviewParams) (Review, error) {

	now := time.Now().Unix()

	var prNumber sql.NullInt64
	if params.PRNumber != nil {
		prNumber = sql.NullInt64{Int64: *params.PRNumber, Valid: true}
	}

	review, err := s.db.CreateReview(ctx, sqlc.CreateReviewParams{
		ReviewID:    params.ReviewID,
		ThreadID:    params.ThreadID,
		RequesterID: params.RequesterID,
		PrNumber:    prNumber,
		Branch:      params.Branch,
		BaseBranch:  params.BaseBranch,
		CommitSha:   params.CommitSHA,
		RepoPath:    params.RepoPath,
		ReviewType:  params.ReviewType,
		Priority:    params.Priority,
		State:       "new",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// GetReview retrieves a review by its review ID.
func (s *SqlcStore) GetReview(ctx context.Context, reviewID string) (Review, error) {
	review, err := s.db.GetReview(ctx, reviewID)
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// GetReviewByThread retrieves a review by its thread ID.
func (s *SqlcStore) GetReviewByThread(ctx context.Context,
	threadID string) (Review, error) {

	review, err := s.db.GetReviewByThread(ctx, threadID)
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// ListReviews lists all reviews with a limit.
func (s *SqlcStore) ListReviews(ctx context.Context, limit int) ([]Review, error) {
	rows, err := s.db.ListReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListReviewsByRequester lists reviews by a specific requester.
func (s *SqlcStore) ListReviewsByRequester(ctx context.Context, requesterID int64,
	limit int) ([]Review, error) {

	rows, err := s.db.ListReviewsByRequester(ctx, sqlc.ListReviewsByRequesterParams{
		RequesterID: requesterID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListReviewsByState lists reviews by state.
func (s *SqlcStore) ListReviewsByState(ctx context.Context, state string,
	limit int) ([]Review, error) {

	rows, err := s.db.ListReviewsByState(ctx, sqlc.ListReviewsByStateParams{
		State: state,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPendingReviews lists reviews pending review, ordered by priority.
func (s *SqlcStore) ListPendingReviews(ctx context.Context,
	limit int) ([]Review, error) {

	rows, err := s.db.ListPendingReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListActiveReviews lists active (not completed) reviews.
func (s *SqlcStore) ListActiveReviews(ctx context.Context,
	limit int) ([]Review, error) {

	rows, err := s.db.ListActiveReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdateReviewState updates the state of a review.
func (s *SqlcStore) UpdateReviewState(ctx context.Context, reviewID,
	state string) error {

	return s.db.UpdateReviewState(ctx, sqlc.UpdateReviewStateParams{
		State:     state,
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
}

// UpdateReviewCommit updates the commit SHA for a review.
func (s *SqlcStore) UpdateReviewCommit(ctx context.Context, reviewID,
	commitSHA string) error {

	return s.db.UpdateReviewCommit(ctx, sqlc.UpdateReviewCommitParams{
		CommitSha: commitSHA,
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
}

// CompleteReview marks a review as completed with a final state.
func (s *SqlcStore) CompleteReview(ctx context.Context, reviewID,
	state string) error {

	now := time.Now().Unix()
	return s.db.CompleteReview(ctx, sqlc.CompleteReviewParams{
		State:       state,
		CompletedAt: sql.NullInt64{Int64: now, Valid: true},
		UpdatedAt:   now,
		ReviewID:    reviewID,
	})
}

// DeleteReview deletes a review and its related data.
func (s *SqlcStore) DeleteReview(ctx context.Context, reviewID string) error {
	// Issues and iterations are cascade deleted via FK constraints.
	return s.db.DeleteReview(ctx, reviewID)
}

// GetReviewStats retrieves aggregate review statistics.
func (s *SqlcStore) GetReviewStats(ctx context.Context) (ReviewStats, error) {
	row, err := s.db.GetReviewStats(ctx)
	if err != nil {
		return ReviewStats{}, err
	}
	return ReviewStats{
		TotalReviews:     row.TotalReviews,
		Approved:         toInt64(row.Approved),
		Pending:          toInt64(row.Pending),
		InProgress:       toInt64(row.InProgress),
		ChangesRequested: toInt64(row.ChangesRequested),
	}, nil
}

// CreateReviewIteration creates a new review iteration.
func (s *SqlcStore) CreateReviewIteration(ctx context.Context,
	params CreateReviewIterationParams) (ReviewIteration, error) {

	now := time.Now().Unix()
	iter, err := s.db.CreateReviewIteration(ctx, sqlc.CreateReviewIterationParams{
		ReviewID:          params.ReviewID,
		IterationNum:      int64(params.IterationNum),
		ReviewerID:        params.ReviewerID,
		ReviewerSessionID: ToSqlcNullString(params.ReviewerSessionID),
		Decision:          params.Decision,
		Summary:           params.Summary,
		IssuesJson:        ToSqlcNullString(params.IssuesJSON),
		SuggestionsJson:   ToSqlcNullString(params.SuggestionsJSON),
		FilesReviewed:     int64(params.FilesReviewed),
		LinesAnalyzed:     int64(params.LinesAnalyzed),
		DurationMs:        params.DurationMS,
		CostUsd:           params.CostUSD,
		StartedAt:         now,
		CompletedAt:       sql.NullInt64{Int64: now, Valid: true},
	})
	if err != nil {
		return ReviewIteration{}, err
	}
	return ReviewIterationFromSqlc(iter), nil
}

// GetLatestReviewIteration retrieves the latest iteration for a review.
func (s *SqlcStore) GetLatestReviewIteration(ctx context.Context,
	reviewID string) (ReviewIteration, error) {

	iter, err := s.db.GetLatestReviewIteration(ctx, reviewID)
	if err != nil {
		return ReviewIteration{}, err
	}
	return ReviewIterationFromSqlc(iter), nil
}

// ListReviewIterations lists all iterations for a review.
func (s *SqlcStore) ListReviewIterations(ctx context.Context,
	reviewID string) ([]ReviewIteration, error) {

	rows, err := s.db.ListReviewIterations(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	iters := make([]ReviewIteration, len(rows))
	for i, row := range rows {
		iters[i] = ReviewIterationFromSqlc(row)
	}
	return iters, nil
}

// GetIterationCount returns the current iteration number for a review.
func (s *SqlcStore) GetIterationCount(ctx context.Context,
	reviewID string) (int, error) {

	result, err := s.db.GetIterationCount(ctx, reviewID)
	if err != nil {
		return 0, err
	}
	return int(toInt64(result)), nil
}

// CreateReviewIssue creates a new review issue.
func (s *SqlcStore) CreateReviewIssue(ctx context.Context,
	params CreateReviewIssueParams) (ReviewIssue, error) {

	now := time.Now().Unix()

	var lineEnd sql.NullInt64
	if params.LineEnd != nil {
		lineEnd = sql.NullInt64{Int64: int64(*params.LineEnd), Valid: true}
	}

	issue, err := s.db.CreateReviewIssue(ctx, sqlc.CreateReviewIssueParams{
		ReviewID:     params.ReviewID,
		IterationNum: int64(params.IterationNum),
		IssueType:    params.IssueType,
		Severity:     params.Severity,
		FilePath:     params.FilePath,
		LineStart:    int64(params.LineStart),
		LineEnd:      lineEnd,
		Title:        params.Title,
		Description:  params.Description,
		CodeSnippet:  ToSqlcNullString(params.CodeSnippet),
		Suggestion:   ToSqlcNullString(params.Suggestion),
		ClaudeMdRef:  ToSqlcNullString(params.ClaudeMDRef),
		Status:       "open",
		CreatedAt:    now,
	})
	if err != nil {
		return ReviewIssue{}, err
	}
	return ReviewIssueFromSqlc(issue), nil
}

// ListReviewIssues lists all issues for a review.
func (s *SqlcStore) ListReviewIssues(ctx context.Context,
	reviewID string) ([]ReviewIssue, error) {

	rows, err := s.db.ListReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// ListOpenReviewIssues lists open issues for a review.
func (s *SqlcStore) ListOpenReviewIssues(ctx context.Context,
	reviewID string) ([]ReviewIssue, error) {

	rows, err := s.db.ListOpenReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// ResolveIssue marks an issue as resolved.
func (s *SqlcStore) ResolveIssue(ctx context.Context, issueID int64,
	iterationNum int) error {

	now := time.Now().Unix()
	return s.db.ResolveIssue(ctx, sqlc.ResolveIssueParams{
		ResolvedAt:          sql.NullInt64{Int64: now, Valid: true},
		ResolvedInIteration: sql.NullInt64{Int64: int64(iterationNum), Valid: true},
		ID:                  issueID,
	})
}

// CountOpenIssues counts open issues for a review.
func (s *SqlcStore) CountOpenIssues(ctx context.Context,
	reviewID string) (int64, error) {

	return s.db.CountOpenIssues(ctx, reviewID)
}

// GetReviewerDecisions retrieves reviewer decisions for an iteration.
func (s *SqlcStore) GetReviewerDecisions(ctx context.Context, reviewID string,
	iterationNum int) ([]ReviewerDecision, error) {

	rows, err := s.db.GetReviewerDecisions(ctx, sqlc.GetReviewerDecisionsParams{
		ReviewID:     reviewID,
		IterationNum: int64(iterationNum),
	})
	if err != nil {
		return nil, err
	}
	decisions := make([]ReviewerDecision, len(rows))
	for i, row := range rows {
		decisions[i] = ReviewerDecision{
			ReviewerID: row.ReviewerID,
			Decision:   row.Decision,
		}
	}
	return decisions, nil
}

// =============================================================================
// ReviewStore implementation for txSqlcStore (transaction context)
// =============================================================================

// CreateReview creates a new review in the database.
func (s *txSqlcStore) CreateReview(ctx context.Context,
	params CreateReviewParams) (Review, error) {

	now := time.Now().Unix()

	var prNumber sql.NullInt64
	if params.PRNumber != nil {
		prNumber = sql.NullInt64{Int64: *params.PRNumber, Valid: true}
	}

	review, err := s.queries.CreateReview(ctx, sqlc.CreateReviewParams{
		ReviewID:    params.ReviewID,
		ThreadID:    params.ThreadID,
		RequesterID: params.RequesterID,
		PrNumber:    prNumber,
		Branch:      params.Branch,
		BaseBranch:  params.BaseBranch,
		CommitSha:   params.CommitSHA,
		RepoPath:    params.RepoPath,
		ReviewType:  params.ReviewType,
		Priority:    params.Priority,
		State:       "new",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// GetReview retrieves a review by its review ID.
func (s *txSqlcStore) GetReview(ctx context.Context, reviewID string) (Review, error) {
	review, err := s.queries.GetReview(ctx, reviewID)
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// GetReviewByThread retrieves a review by its thread ID.
func (s *txSqlcStore) GetReviewByThread(ctx context.Context,
	threadID string) (Review, error) {

	review, err := s.queries.GetReviewByThread(ctx, threadID)
	if err != nil {
		return Review{}, err
	}
	return ReviewFromSqlc(review), nil
}

// ListReviews lists all reviews with a limit.
func (s *txSqlcStore) ListReviews(ctx context.Context, limit int) ([]Review, error) {
	rows, err := s.queries.ListReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListReviewsByRequester lists reviews by a specific requester.
func (s *txSqlcStore) ListReviewsByRequester(ctx context.Context, requesterID int64,
	limit int) ([]Review, error) {

	rows, err := s.queries.ListReviewsByRequester(ctx, sqlc.ListReviewsByRequesterParams{
		RequesterID: requesterID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListReviewsByState lists reviews by state.
func (s *txSqlcStore) ListReviewsByState(ctx context.Context, state string,
	limit int) ([]Review, error) {

	rows, err := s.queries.ListReviewsByState(ctx, sqlc.ListReviewsByStateParams{
		State: state,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPendingReviews lists reviews pending review, ordered by priority.
func (s *txSqlcStore) ListPendingReviews(ctx context.Context,
	limit int) ([]Review, error) {

	rows, err := s.queries.ListPendingReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListActiveReviews lists active (not completed) reviews.
func (s *txSqlcStore) ListActiveReviews(ctx context.Context,
	limit int) ([]Review, error) {

	rows, err := s.queries.ListActiveReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdateReviewState updates the state of a review.
func (s *txSqlcStore) UpdateReviewState(ctx context.Context, reviewID,
	state string) error {

	return s.queries.UpdateReviewState(ctx, sqlc.UpdateReviewStateParams{
		State:     state,
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
}

// UpdateReviewCommit updates the commit SHA for a review.
func (s *txSqlcStore) UpdateReviewCommit(ctx context.Context, reviewID,
	commitSHA string) error {

	return s.queries.UpdateReviewCommit(ctx, sqlc.UpdateReviewCommitParams{
		CommitSha: commitSHA,
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
}

// CompleteReview marks a review as completed with a final state.
func (s *txSqlcStore) CompleteReview(ctx context.Context, reviewID,
	state string) error {

	now := time.Now().Unix()
	return s.queries.CompleteReview(ctx, sqlc.CompleteReviewParams{
		State:       state,
		CompletedAt: sql.NullInt64{Int64: now, Valid: true},
		UpdatedAt:   now,
		ReviewID:    reviewID,
	})
}

// DeleteReview deletes a review and its related data.
func (s *txSqlcStore) DeleteReview(ctx context.Context, reviewID string) error {
	return s.queries.DeleteReview(ctx, reviewID)
}

// GetReviewStats retrieves aggregate review statistics.
func (s *txSqlcStore) GetReviewStats(ctx context.Context) (ReviewStats, error) {
	row, err := s.queries.GetReviewStats(ctx)
	if err != nil {
		return ReviewStats{}, err
	}
	return ReviewStats{
		TotalReviews:     row.TotalReviews,
		Approved:         toInt64(row.Approved),
		Pending:          toInt64(row.Pending),
		InProgress:       toInt64(row.InProgress),
		ChangesRequested: toInt64(row.ChangesRequested),
	}, nil
}

// CreateReviewIteration creates a new review iteration.
func (s *txSqlcStore) CreateReviewIteration(ctx context.Context,
	params CreateReviewIterationParams) (ReviewIteration, error) {

	now := time.Now().Unix()
	iter, err := s.queries.CreateReviewIteration(ctx, sqlc.CreateReviewIterationParams{
		ReviewID:          params.ReviewID,
		IterationNum:      int64(params.IterationNum),
		ReviewerID:        params.ReviewerID,
		ReviewerSessionID: ToSqlcNullString(params.ReviewerSessionID),
		Decision:          params.Decision,
		Summary:           params.Summary,
		IssuesJson:        ToSqlcNullString(params.IssuesJSON),
		SuggestionsJson:   ToSqlcNullString(params.SuggestionsJSON),
		FilesReviewed:     int64(params.FilesReviewed),
		LinesAnalyzed:     int64(params.LinesAnalyzed),
		DurationMs:        params.DurationMS,
		CostUsd:           params.CostUSD,
		StartedAt:         now,
		CompletedAt:       sql.NullInt64{Int64: now, Valid: true},
	})
	if err != nil {
		return ReviewIteration{}, err
	}
	return ReviewIterationFromSqlc(iter), nil
}

// GetLatestReviewIteration retrieves the latest iteration for a review.
func (s *txSqlcStore) GetLatestReviewIteration(ctx context.Context,
	reviewID string) (ReviewIteration, error) {

	iter, err := s.queries.GetLatestReviewIteration(ctx, reviewID)
	if err != nil {
		return ReviewIteration{}, err
	}
	return ReviewIterationFromSqlc(iter), nil
}

// ListReviewIterations lists all iterations for a review.
func (s *txSqlcStore) ListReviewIterations(ctx context.Context,
	reviewID string) ([]ReviewIteration, error) {

	rows, err := s.queries.ListReviewIterations(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	iters := make([]ReviewIteration, len(rows))
	for i, row := range rows {
		iters[i] = ReviewIterationFromSqlc(row)
	}
	return iters, nil
}

// GetIterationCount returns the current iteration number for a review.
func (s *txSqlcStore) GetIterationCount(ctx context.Context,
	reviewID string) (int, error) {

	result, err := s.queries.GetIterationCount(ctx, reviewID)
	if err != nil {
		return 0, err
	}
	return int(toInt64(result)), nil
}

// CreateReviewIssue creates a new review issue.
func (s *txSqlcStore) CreateReviewIssue(ctx context.Context,
	params CreateReviewIssueParams) (ReviewIssue, error) {

	now := time.Now().Unix()

	var lineEnd sql.NullInt64
	if params.LineEnd != nil {
		lineEnd = sql.NullInt64{Int64: int64(*params.LineEnd), Valid: true}
	}

	issue, err := s.queries.CreateReviewIssue(ctx, sqlc.CreateReviewIssueParams{
		ReviewID:     params.ReviewID,
		IterationNum: int64(params.IterationNum),
		IssueType:    params.IssueType,
		Severity:     params.Severity,
		FilePath:     params.FilePath,
		LineStart:    int64(params.LineStart),
		LineEnd:      lineEnd,
		Title:        params.Title,
		Description:  params.Description,
		CodeSnippet:  ToSqlcNullString(params.CodeSnippet),
		Suggestion:   ToSqlcNullString(params.Suggestion),
		ClaudeMdRef:  ToSqlcNullString(params.ClaudeMDRef),
		Status:       "open",
		CreatedAt:    now,
	})
	if err != nil {
		return ReviewIssue{}, err
	}
	return ReviewIssueFromSqlc(issue), nil
}

// ListReviewIssues lists all issues for a review.
func (s *txSqlcStore) ListReviewIssues(ctx context.Context,
	reviewID string) ([]ReviewIssue, error) {

	rows, err := s.queries.ListReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// ListOpenReviewIssues lists open issues for a review.
func (s *txSqlcStore) ListOpenReviewIssues(ctx context.Context,
	reviewID string) ([]ReviewIssue, error) {

	rows, err := s.queries.ListOpenReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// ResolveIssue marks an issue as resolved.
func (s *txSqlcStore) ResolveIssue(ctx context.Context, issueID int64,
	iterationNum int) error {

	now := time.Now().Unix()
	return s.queries.ResolveIssue(ctx, sqlc.ResolveIssueParams{
		ResolvedAt:          sql.NullInt64{Int64: now, Valid: true},
		ResolvedInIteration: sql.NullInt64{Int64: int64(iterationNum), Valid: true},
		ID:                  issueID,
	})
}

// CountOpenIssues counts open issues for a review.
func (s *txSqlcStore) CountOpenIssues(ctx context.Context,
	reviewID string) (int64, error) {

	return s.queries.CountOpenIssues(ctx, reviewID)
}

// GetReviewerDecisions retrieves reviewer decisions for an iteration.
func (s *txSqlcStore) GetReviewerDecisions(ctx context.Context, reviewID string,
	iterationNum int) ([]ReviewerDecision, error) {

	rows, err := s.queries.GetReviewerDecisions(ctx, sqlc.GetReviewerDecisionsParams{
		ReviewID:     reviewID,
		IterationNum: int64(iterationNum),
	})
	if err != nil {
		return nil, err
	}
	decisions := make([]ReviewerDecision, len(rows))
	for i, row := range rows {
		decisions[i] = ReviewerDecision{
			ReviewerID: row.ReviewerID,
			Decision:   row.Decision,
		}
	}
	return decisions, nil
}

// =============================================================================
// Helper functions
// =============================================================================

// toInt64 converts interface{} to int64, handling SQLite's various return
// types for aggregate functions.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case nil:
		return 0
	default:
		return 0
	}
}
