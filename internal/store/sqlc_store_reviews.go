package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// =============================================================================
// ReviewStore implementation for SqlcStore
// =============================================================================

// CreateReview creates a new review record.
func (s *SqlcStore) CreateReview(ctx context.Context,
	params CreateReviewParams,
) (Review, error) {
	now := time.Now().Unix()

	row, err := s.db.CreateReview(ctx, sqlc.CreateReviewParams{
		ReviewID:    params.ReviewID,
		ThreadID:    params.ThreadID,
		RequesterID: params.RequesterID,
		PrNumber:    ToSqlcNullInt64Val(params.PRNumber),
		Branch:      params.Branch,
		BaseBranch:  params.BaseBranch,
		CommitSha:   params.CommitSHA,
		RepoPath:    params.RepoPath,
		RemoteUrl:   ToSqlcNullString(params.RemoteURL),
		ReviewType:  params.ReviewType,
		Priority:    params.Priority,
		State:       "new",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Review{}, err
	}

	return ReviewFromSqlc(row), nil
}

// GetReview retrieves a review by its UUID.
func (s *SqlcStore) GetReview(ctx context.Context,
	reviewID string,
) (Review, error) {
	row, err := s.db.GetReview(ctx, reviewID)
	if err != nil {
		return Review{}, err
	}

	return ReviewFromSqlc(row), nil
}

// ListReviews lists reviews ordered by creation time.
func (s *SqlcStore) ListReviews(ctx context.Context,
	limit, offset int,
) ([]Review, error) {
	rows, err := s.db.ListReviews(ctx, sqlc.ListReviewsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
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

// ListReviewsByState lists reviews matching the given state.
func (s *SqlcStore) ListReviewsByState(ctx context.Context,
	state string, limit int,
) ([]Review, error) {
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

// ListReviewsByRequester lists reviews by the requesting agent.
func (s *SqlcStore) ListReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]Review, error) {
	rows, err := s.db.ListReviewsByRequester(
		ctx, sqlc.ListReviewsByRequesterParams{
			RequesterID: requesterID,
			Limit:       int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}

	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListActiveReviews returns reviews in non-terminal states.
func (s *SqlcStore) ListActiveReviews(
	ctx context.Context,
) ([]Review, error) {
	rows, err := s.db.ListActiveReviews(ctx)
	if err != nil {
		return nil, err
	}

	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdateReviewState updates the FSM state of a review.
func (s *SqlcStore) UpdateReviewState(ctx context.Context,
	reviewID, state string,
) error {
	return s.db.UpdateReviewState(ctx, sqlc.UpdateReviewStateParams{
		ReviewID:  reviewID,
		State:     state,
		UpdatedAt: time.Now().Unix(),
	})
}

// UpdateReviewCompleted marks a review as completed with a terminal state.
func (s *SqlcStore) UpdateReviewCompleted(ctx context.Context,
	reviewID, state string,
) error {
	now := time.Now().Unix()
	return s.db.UpdateReviewCompleted(ctx, sqlc.UpdateReviewCompletedParams{
		ReviewID:    reviewID,
		State:       state,
		UpdatedAt:   now,
		CompletedAt: sql.NullInt64{Int64: now, Valid: true},
	})
}

// CreateReviewIteration records a review iteration result.
func (s *SqlcStore) CreateReviewIteration(ctx context.Context,
	params CreateReviewIterationParams,
) (ReviewIteration, error) {
	row, err := s.db.CreateReviewIteration(
		ctx, sqlc.CreateReviewIterationParams{
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
			StartedAt:         params.StartedAt.Unix(),
			CompletedAt:       ToSqlcNullInt64(params.CompletedAt),
		},
	)
	if err != nil {
		return ReviewIteration{}, err
	}

	return ReviewIterationFromSqlc(row), nil
}

// GetReviewIterations gets all iterations for a review.
func (s *SqlcStore) GetReviewIterations(ctx context.Context,
	reviewID string,
) ([]ReviewIteration, error) {
	rows, err := s.db.GetReviewIterations(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	iters := make([]ReviewIteration, len(rows))
	for i, row := range rows {
		iters[i] = ReviewIterationFromSqlc(row)
	}
	return iters, nil
}

// CreateReviewIssue records a specific issue found during review.
func (s *SqlcStore) CreateReviewIssue(ctx context.Context,
	params CreateReviewIssueParams,
) (ReviewIssue, error) {
	row, err := s.db.CreateReviewIssue(ctx, sqlc.CreateReviewIssueParams{
		ReviewID:     params.ReviewID,
		IterationNum: int64(params.IterationNum),
		IssueType:    params.IssueType,
		Severity:     params.Severity,
		FilePath:     params.FilePath,
		LineStart:    int64(params.LineStart),
		LineEnd:      ToSqlcNullInt64Val(params.LineEnd),
		Title:        params.Title,
		Description:  params.Description,
		CodeSnippet:  ToSqlcNullString(params.CodeSnippet),
		Suggestion:   ToSqlcNullString(params.Suggestion),
		ClaudeMdRef:  ToSqlcNullString(params.ClaudeMDRef),
		Status:       "open",
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return ReviewIssue{}, err
	}

	return ReviewIssueFromSqlc(row), nil
}

// GetReviewIssues gets all issues for a review.
func (s *SqlcStore) GetReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	rows, err := s.db.GetReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// GetOpenReviewIssues gets open issues for a review.
func (s *SqlcStore) GetOpenReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	rows, err := s.db.GetOpenReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// UpdateReviewIssueStatus updates an issue's resolution status.
func (s *SqlcStore) UpdateReviewIssueStatus(ctx context.Context,
	issueID int64, status string, resolvedInIteration *int,
) error {
	var resolvedAt sql.NullInt64
	if status == "fixed" || status == "wont_fix" || status == "duplicate" {
		resolvedAt = sql.NullInt64{
			Int64: time.Now().Unix(), Valid: true,
		}
	}

	return s.db.UpdateReviewIssueStatus(
		ctx, sqlc.UpdateReviewIssueStatusParams{
			ID:                  issueID,
			Status:              status,
			ResolvedAt:          resolvedAt,
			ResolvedInIteration: ToSqlcNullInt64FromInt(resolvedInIteration),
		},
	)
}

// CountOpenIssues counts open issues for a review.
func (s *SqlcStore) CountOpenIssues(ctx context.Context,
	reviewID string,
) (int64, error) {
	return s.db.CountOpenIssues(ctx, reviewID)
}

// =============================================================================
// ReviewStore implementation for txSqlcStore
// =============================================================================

// CreateReview creates a new review record within a transaction.
func (s *txSqlcStore) CreateReview(ctx context.Context,
	params CreateReviewParams,
) (Review, error) {
	now := time.Now().Unix()

	row, err := s.queries.CreateReview(ctx, sqlc.CreateReviewParams{
		ReviewID:    params.ReviewID,
		ThreadID:    params.ThreadID,
		RequesterID: params.RequesterID,
		PrNumber:    ToSqlcNullInt64Val(params.PRNumber),
		Branch:      params.Branch,
		BaseBranch:  params.BaseBranch,
		CommitSha:   params.CommitSHA,
		RepoPath:    params.RepoPath,
		RemoteUrl:   ToSqlcNullString(params.RemoteURL),
		ReviewType:  params.ReviewType,
		Priority:    params.Priority,
		State:       "new",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Review{}, err
	}

	return ReviewFromSqlc(row), nil
}

// GetReview retrieves a review by its UUID within a transaction.
func (s *txSqlcStore) GetReview(ctx context.Context,
	reviewID string,
) (Review, error) {
	row, err := s.queries.GetReview(ctx, reviewID)
	if err != nil {
		return Review{}, err
	}

	return ReviewFromSqlc(row), nil
}

// ListReviews lists reviews ordered by creation time within a transaction.
func (s *txSqlcStore) ListReviews(ctx context.Context,
	limit, offset int,
) ([]Review, error) {
	rows, err := s.queries.ListReviews(ctx, sqlc.ListReviewsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
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

// ListReviewsByState lists reviews matching the given state within a
// transaction.
func (s *txSqlcStore) ListReviewsByState(ctx context.Context,
	state string, limit int,
) ([]Review, error) {
	rows, err := s.queries.ListReviewsByState(
		ctx, sqlc.ListReviewsByStateParams{
			State: state,
			Limit: int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}

	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListReviewsByRequester lists reviews by the requesting agent within a
// transaction.
func (s *txSqlcStore) ListReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]Review, error) {
	rows, err := s.queries.ListReviewsByRequester(
		ctx, sqlc.ListReviewsByRequesterParams{
			RequesterID: requesterID,
			Limit:       int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}

	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListActiveReviews returns reviews in non-terminal states within a
// transaction.
func (s *txSqlcStore) ListActiveReviews(
	ctx context.Context,
) ([]Review, error) {
	rows, err := s.queries.ListActiveReviews(ctx)
	if err != nil {
		return nil, err
	}

	reviews := make([]Review, len(rows))
	for i, row := range rows {
		reviews[i] = ReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdateReviewState updates the FSM state of a review within a transaction.
func (s *txSqlcStore) UpdateReviewState(ctx context.Context,
	reviewID, state string,
) error {
	return s.queries.UpdateReviewState(ctx, sqlc.UpdateReviewStateParams{
		ReviewID:  reviewID,
		State:     state,
		UpdatedAt: time.Now().Unix(),
	})
}

// UpdateReviewCompleted marks a review as completed within a transaction.
func (s *txSqlcStore) UpdateReviewCompleted(ctx context.Context,
	reviewID, state string,
) error {
	now := time.Now().Unix()
	return s.queries.UpdateReviewCompleted(
		ctx, sqlc.UpdateReviewCompletedParams{
			ReviewID:    reviewID,
			State:       state,
			UpdatedAt:   now,
			CompletedAt: sql.NullInt64{Int64: now, Valid: true},
		},
	)
}

// CreateReviewIteration records a review iteration result within a
// transaction.
func (s *txSqlcStore) CreateReviewIteration(ctx context.Context,
	params CreateReviewIterationParams,
) (ReviewIteration, error) {
	row, err := s.queries.CreateReviewIteration(
		ctx, sqlc.CreateReviewIterationParams{
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
			StartedAt:         params.StartedAt.Unix(),
			CompletedAt:       ToSqlcNullInt64(params.CompletedAt),
		},
	)
	if err != nil {
		return ReviewIteration{}, err
	}

	return ReviewIterationFromSqlc(row), nil
}

// GetReviewIterations gets all iterations for a review within a transaction.
func (s *txSqlcStore) GetReviewIterations(ctx context.Context,
	reviewID string,
) ([]ReviewIteration, error) {
	rows, err := s.queries.GetReviewIterations(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	iters := make([]ReviewIteration, len(rows))
	for i, row := range rows {
		iters[i] = ReviewIterationFromSqlc(row)
	}
	return iters, nil
}

// CreateReviewIssue records a specific issue found during review within a
// transaction.
func (s *txSqlcStore) CreateReviewIssue(ctx context.Context,
	params CreateReviewIssueParams,
) (ReviewIssue, error) {
	row, err := s.queries.CreateReviewIssue(
		ctx, sqlc.CreateReviewIssueParams{
			ReviewID:     params.ReviewID,
			IterationNum: int64(params.IterationNum),
			IssueType:    params.IssueType,
			Severity:     params.Severity,
			FilePath:     params.FilePath,
			LineStart:    int64(params.LineStart),
			LineEnd:      ToSqlcNullInt64Val(params.LineEnd),
			Title:        params.Title,
			Description:  params.Description,
			CodeSnippet:  ToSqlcNullString(params.CodeSnippet),
			Suggestion:   ToSqlcNullString(params.Suggestion),
			ClaudeMdRef:  ToSqlcNullString(params.ClaudeMDRef),
			Status:       "open",
			CreatedAt:    time.Now().Unix(),
		},
	)
	if err != nil {
		return ReviewIssue{}, err
	}

	return ReviewIssueFromSqlc(row), nil
}

// GetReviewIssues gets all issues for a review within a transaction.
func (s *txSqlcStore) GetReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	rows, err := s.queries.GetReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// GetOpenReviewIssues gets open issues for a review within a transaction.
func (s *txSqlcStore) GetOpenReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	rows, err := s.queries.GetOpenReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	issues := make([]ReviewIssue, len(rows))
	for i, row := range rows {
		issues[i] = ReviewIssueFromSqlc(row)
	}
	return issues, nil
}

// UpdateReviewIssueStatus updates an issue's resolution status within a
// transaction.
func (s *txSqlcStore) UpdateReviewIssueStatus(ctx context.Context,
	issueID int64, status string, resolvedInIteration *int,
) error {
	var resolvedAt sql.NullInt64
	if status == "fixed" || status == "wont_fix" || status == "duplicate" {
		resolvedAt = sql.NullInt64{
			Int64: time.Now().Unix(), Valid: true,
		}
	}

	return s.queries.UpdateReviewIssueStatus(
		ctx, sqlc.UpdateReviewIssueStatusParams{
			ID:                  issueID,
			Status:              status,
			ResolvedAt:          resolvedAt,
			ResolvedInIteration: ToSqlcNullInt64FromInt(resolvedInIteration),
		},
	)
}

// CountOpenIssues counts open issues for a review within a transaction.
func (s *txSqlcStore) CountOpenIssues(ctx context.Context,
	reviewID string,
) (int64, error) {
	return s.queries.CountOpenIssues(ctx, reviewID)
}

// =============================================================================
// DeleteReview implementations
// =============================================================================

// DeleteReview deletes a review and its associated iterations and issues.
func (s *SqlcStore) DeleteReview(ctx context.Context,
	reviewID string,
) error {
	return s.WithTx(ctx, func(
		ctx context.Context, txStore Storage,
	) error {
		// Delete child records first (issues, then iterations), then
		// the parent review.
		if err := s.db.DeleteReviewIssues(
			ctx, reviewID,
		); err != nil {
			return err
		}
		if err := s.db.DeleteReviewIterations(
			ctx, reviewID,
		); err != nil {
			return err
		}

		return s.db.DeleteReview(ctx, reviewID)
	})
}

// DeleteReview deletes a review within a transaction.
func (s *txSqlcStore) DeleteReview(ctx context.Context,
	reviewID string,
) error {
	if err := s.queries.DeleteReviewIssues(
		ctx, reviewID,
	); err != nil {
		return err
	}
	if err := s.queries.DeleteReviewIterations(
		ctx, reviewID,
	); err != nil {
		return err
	}

	return s.queries.DeleteReview(ctx, reviewID)
}
