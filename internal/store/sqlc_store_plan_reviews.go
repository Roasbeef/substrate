package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// =============================================================================
// PlanReviewStore implementation for SqlcStore
// =============================================================================

// CreatePlanReview creates a new plan review record.
func (s *SqlcStore) CreatePlanReview(ctx context.Context,
	params CreatePlanReviewParams,
) (PlanReview, error) {

	now := time.Now().Unix()

	var msgID sql.NullInt64
	if params.MessageID != nil {
		msgID = sql.NullInt64{Int64: *params.MessageID, Valid: true}
	}

	row, err := s.db.CreatePlanReview(ctx, sqlc.CreatePlanReviewParams{
		PlanReviewID: params.PlanReviewID,
		MessageID:    msgID,
		ThreadID:     params.ThreadID,
		RequesterID:  params.RequesterID,
		ReviewerName: params.ReviewerName,
		PlanPath:     params.PlanPath,
		PlanTitle:    params.PlanTitle,
		PlanSummary:  ToSqlcNullString(params.PlanSummary),
		State:        "pending",
		SessionID:    ToSqlcNullString(params.SessionID),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReview retrieves a plan review by its UUID.
func (s *SqlcStore) GetPlanReview(ctx context.Context,
	planReviewID string,
) (PlanReview, error) {

	row, err := s.db.GetPlanReview(ctx, planReviewID)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewByMessage retrieves a plan review by message ID.
func (s *SqlcStore) GetPlanReviewByMessage(ctx context.Context,
	messageID int64,
) (PlanReview, error) {

	row, err := s.db.GetPlanReviewByMessage(
		ctx, sql.NullInt64{Int64: messageID, Valid: true},
	)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewByThread retrieves the latest plan review for a thread.
func (s *SqlcStore) GetPlanReviewByThread(ctx context.Context,
	threadID string,
) (PlanReview, error) {

	row, err := s.db.GetPlanReviewByThread(ctx, threadID)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewBySession retrieves the pending plan review for a session.
func (s *SqlcStore) GetPlanReviewBySession(ctx context.Context,
	sessionID string,
) (PlanReview, error) {

	row, err := s.db.GetPlanReviewBySession(
		ctx, ToSqlcNullString(sessionID),
	)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// ListPlanReviews lists plan reviews ordered by creation time.
func (s *SqlcStore) ListPlanReviews(ctx context.Context,
	limit, offset int,
) ([]PlanReview, error) {

	rows, err := s.db.ListPlanReviews(ctx, sqlc.ListPlanReviewsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPlanReviewsByState lists plan reviews matching the given state.
func (s *SqlcStore) ListPlanReviewsByState(ctx context.Context,
	state string, limit int,
) ([]PlanReview, error) {

	rows, err := s.db.ListPlanReviewsByState(
		ctx, sqlc.ListPlanReviewsByStateParams{
			State: state,
			Limit: int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPlanReviewsByRequester lists plan reviews by the requesting agent.
func (s *SqlcStore) ListPlanReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]PlanReview, error) {

	rows, err := s.db.ListPlanReviewsByRequester(
		ctx, sqlc.ListPlanReviewsByRequesterParams{
			RequesterID: requesterID,
			Limit:       int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdatePlanReviewState updates the state and reviewer info of a plan review.
func (s *SqlcStore) UpdatePlanReviewState(ctx context.Context,
	params UpdatePlanReviewStateParams,
) error {

	now := time.Now().Unix()

	var reviewedBy sql.NullInt64
	if params.ReviewedBy != nil {
		reviewedBy = sql.NullInt64{
			Int64: *params.ReviewedBy, Valid: true,
		}
	}

	return s.db.UpdatePlanReviewState(ctx, sqlc.UpdatePlanReviewStateParams{
		State:           params.State,
		ReviewerComment: ToSqlcNullString(params.ReviewerComment),
		ReviewedBy:      reviewedBy,
		UpdatedAt:       now,
		ReviewedAt:      sql.NullInt64{Int64: now, Valid: true},
		PlanReviewID:    params.PlanReviewID,
	})
}

// DeletePlanReview deletes a plan review by its UUID.
func (s *SqlcStore) DeletePlanReview(ctx context.Context,
	planReviewID string,
) error {

	return s.db.DeletePlanReview(ctx, planReviewID)
}

// =============================================================================
// PlanReviewStore implementation for txSqlcStore
// =============================================================================

// CreatePlanReview creates a new plan review record.
func (s *txSqlcStore) CreatePlanReview(ctx context.Context,
	params CreatePlanReviewParams,
) (PlanReview, error) {

	now := time.Now().Unix()

	var msgID sql.NullInt64
	if params.MessageID != nil {
		msgID = sql.NullInt64{Int64: *params.MessageID, Valid: true}
	}

	row, err := s.queries.CreatePlanReview(
		ctx, sqlc.CreatePlanReviewParams{
			PlanReviewID: params.PlanReviewID,
			MessageID:    msgID,
			ThreadID:     params.ThreadID,
			RequesterID:  params.RequesterID,
			ReviewerName: params.ReviewerName,
			PlanPath:     params.PlanPath,
			PlanTitle:    params.PlanTitle,
			PlanSummary:  ToSqlcNullString(params.PlanSummary),
			State:        "pending",
			SessionID:    ToSqlcNullString(params.SessionID),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReview retrieves a plan review by its UUID.
func (s *txSqlcStore) GetPlanReview(ctx context.Context,
	planReviewID string,
) (PlanReview, error) {

	row, err := s.queries.GetPlanReview(ctx, planReviewID)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewByMessage retrieves a plan review by message ID.
func (s *txSqlcStore) GetPlanReviewByMessage(ctx context.Context,
	messageID int64,
) (PlanReview, error) {

	row, err := s.queries.GetPlanReviewByMessage(
		ctx, sql.NullInt64{Int64: messageID, Valid: true},
	)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewByThread retrieves the latest plan review for a thread.
func (s *txSqlcStore) GetPlanReviewByThread(ctx context.Context,
	threadID string,
) (PlanReview, error) {

	row, err := s.queries.GetPlanReviewByThread(ctx, threadID)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// GetPlanReviewBySession retrieves the pending plan review for a session.
func (s *txSqlcStore) GetPlanReviewBySession(ctx context.Context,
	sessionID string,
) (PlanReview, error) {

	row, err := s.queries.GetPlanReviewBySession(
		ctx, ToSqlcNullString(sessionID),
	)
	if err != nil {
		return PlanReview{}, err
	}
	return PlanReviewFromSqlc(row), nil
}

// ListPlanReviews lists plan reviews ordered by creation time.
func (s *txSqlcStore) ListPlanReviews(ctx context.Context,
	limit, offset int,
) ([]PlanReview, error) {

	rows, err := s.queries.ListPlanReviews(
		ctx, sqlc.ListPlanReviewsParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		},
	)
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPlanReviewsByState lists plan reviews matching the given state.
func (s *txSqlcStore) ListPlanReviewsByState(ctx context.Context,
	state string, limit int,
) ([]PlanReview, error) {

	rows, err := s.queries.ListPlanReviewsByState(
		ctx, sqlc.ListPlanReviewsByStateParams{
			State: state,
			Limit: int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// ListPlanReviewsByRequester lists plan reviews by the requesting agent.
func (s *txSqlcStore) ListPlanReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]PlanReview, error) {

	rows, err := s.queries.ListPlanReviewsByRequester(
		ctx, sqlc.ListPlanReviewsByRequesterParams{
			RequesterID: requesterID,
			Limit:       int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	reviews := make([]PlanReview, len(rows))
	for i, row := range rows {
		reviews[i] = PlanReviewFromSqlc(row)
	}
	return reviews, nil
}

// UpdatePlanReviewState updates the state and reviewer info of a plan review.
func (s *txSqlcStore) UpdatePlanReviewState(ctx context.Context,
	params UpdatePlanReviewStateParams,
) error {

	now := time.Now().Unix()

	var reviewedBy sql.NullInt64
	if params.ReviewedBy != nil {
		reviewedBy = sql.NullInt64{
			Int64: *params.ReviewedBy, Valid: true,
		}
	}

	return s.queries.UpdatePlanReviewState(
		ctx, sqlc.UpdatePlanReviewStateParams{
			State:           params.State,
			ReviewerComment: ToSqlcNullString(params.ReviewerComment),
			ReviewedBy:      reviewedBy,
			UpdatedAt:       now,
			ReviewedAt:      sql.NullInt64{Int64: now, Valid: true},
			PlanReviewID:    params.PlanReviewID,
		},
	)
}

// DeletePlanReview deletes a plan review by its UUID.
func (s *txSqlcStore) DeletePlanReview(ctx context.Context,
	planReviewID string,
) error {

	return s.queries.DeletePlanReview(ctx, planReviewID)
}
