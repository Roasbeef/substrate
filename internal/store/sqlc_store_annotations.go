package store

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// =============================================================================
// AnnotationStore implementation for SqlcStore
// =============================================================================

// CreatePlanAnnotation creates a new plan annotation record.
func (s *SqlcStore) CreatePlanAnnotation(ctx context.Context,
	params CreatePlanAnnotationParams,
) (PlanAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.db.CreatePlanAnnotation(
		ctx, sqlc.CreatePlanAnnotationParams{
			PlanReviewID:   params.PlanReviewID,
			AnnotationID:   params.AnnotationID,
			BlockID:        params.BlockID,
			AnnotationType: params.AnnotationType,
			Text:           ToSqlcNullString(params.Text),
			OriginalText:   params.OriginalText,
			StartOffset:    int64(params.StartOffset),
			EndOffset:      int64(params.EndOffset),
			DiffContext:    ToSqlcNullString(params.DiffContext),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	)
	if err != nil {
		return PlanAnnotation{}, err
	}

	return PlanAnnotationFromSqlc(row), nil
}

// GetPlanAnnotation retrieves a plan annotation by its UUID.
func (s *SqlcStore) GetPlanAnnotation(ctx context.Context,
	annotationID string,
) (PlanAnnotation, error) {
	row, err := s.db.GetPlanAnnotation(ctx, annotationID)
	if err != nil {
		return PlanAnnotation{}, err
	}

	return PlanAnnotationFromSqlc(row), nil
}

// ListPlanAnnotationsByReview retrieves all annotations for a plan review.
func (s *SqlcStore) ListPlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) ([]PlanAnnotation, error) {
	rows, err := s.db.ListPlanAnnotationsByReview(ctx, planReviewID)
	if err != nil {
		return nil, err
	}

	result := make([]PlanAnnotation, 0, len(rows))
	for _, row := range rows {
		result = append(result, PlanAnnotationFromSqlc(row))
	}

	return result, nil
}

// UpdatePlanAnnotation updates a plan annotation's content.
func (s *SqlcStore) UpdatePlanAnnotation(ctx context.Context,
	params UpdatePlanAnnotationParams,
) (PlanAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.db.UpdatePlanAnnotation(
		ctx, sqlc.UpdatePlanAnnotationParams{
			Text:         ToSqlcNullString(params.Text),
			OriginalText: params.OriginalText,
			StartOffset:  int64(params.StartOffset),
			EndOffset:    int64(params.EndOffset),
			DiffContext:  ToSqlcNullString(params.DiffContext),
			UpdatedAt:    now,
			AnnotationID: params.AnnotationID,
		},
	)
	if err != nil {
		return PlanAnnotation{}, err
	}
	return PlanAnnotationFromSqlc(row), nil
}

// DeletePlanAnnotation deletes a plan annotation by its UUID.
func (s *SqlcStore) DeletePlanAnnotation(ctx context.Context,
	annotationID string,
) error {
	return s.db.DeletePlanAnnotation(ctx, annotationID)
}

// DeletePlanAnnotationsByReview deletes all annotations for a plan review.
func (s *SqlcStore) DeletePlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) error {
	return s.db.DeletePlanAnnotationsByReview(ctx, planReviewID)
}

// CreateDiffAnnotation creates a new diff annotation record.
func (s *SqlcStore) CreateDiffAnnotation(ctx context.Context,
	params CreateDiffAnnotationParams,
) (DiffAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.db.CreateDiffAnnotation(
		ctx, sqlc.CreateDiffAnnotationParams{
			AnnotationID:   params.AnnotationID,
			MessageID:      params.MessageID,
			AnnotationType: params.AnnotationType,
			Scope:          params.Scope,
			FilePath:       params.FilePath,
			LineStart:      int64(params.LineStart),
			LineEnd:        int64(params.LineEnd),
			Side:           params.Side,
			Text:           ToSqlcNullString(params.Text),
			SuggestedCode:  ToSqlcNullString(params.SuggestedCode),
			OriginalCode:   ToSqlcNullString(params.OriginalCode),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	)
	if err != nil {
		return DiffAnnotation{}, err
	}

	return DiffAnnotationFromSqlc(row), nil
}

// GetDiffAnnotation retrieves a diff annotation by its UUID.
func (s *SqlcStore) GetDiffAnnotation(ctx context.Context,
	annotationID string,
) (DiffAnnotation, error) {
	row, err := s.db.GetDiffAnnotation(ctx, annotationID)
	if err != nil {
		return DiffAnnotation{}, err
	}

	return DiffAnnotationFromSqlc(row), nil
}

// ListDiffAnnotationsByMessage retrieves all diff annotations for a
// message.
func (s *SqlcStore) ListDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) ([]DiffAnnotation, error) {
	rows, err := s.db.ListDiffAnnotationsByMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}

	result := make([]DiffAnnotation, 0, len(rows))
	for _, row := range rows {
		result = append(result, DiffAnnotationFromSqlc(row))
	}

	return result, nil
}

// UpdateDiffAnnotation updates a diff annotation's content.
func (s *SqlcStore) UpdateDiffAnnotation(ctx context.Context,
	params UpdateDiffAnnotationParams,
) (DiffAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.db.UpdateDiffAnnotation(
		ctx, sqlc.UpdateDiffAnnotationParams{
			Text:          ToSqlcNullString(params.Text),
			SuggestedCode: ToSqlcNullString(params.SuggestedCode),
			OriginalCode:  ToSqlcNullString(params.OriginalCode),
			UpdatedAt:     now,
			AnnotationID:  params.AnnotationID,
		},
	)
	if err != nil {
		return DiffAnnotation{}, err
	}
	return DiffAnnotationFromSqlc(row), nil
}

// DeleteDiffAnnotation deletes a diff annotation by its UUID.
func (s *SqlcStore) DeleteDiffAnnotation(ctx context.Context,
	annotationID string,
) error {
	return s.db.DeleteDiffAnnotation(ctx, annotationID)
}

// DeleteDiffAnnotationsByMessage deletes all diff annotations for a
// message.
func (s *SqlcStore) DeleteDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) error {
	return s.db.DeleteDiffAnnotationsByMessage(ctx, messageID)
}

// =============================================================================
// AnnotationStore implementation for txSqlcStore
// =============================================================================

// CreatePlanAnnotation creates a new plan annotation record.
func (s *txSqlcStore) CreatePlanAnnotation(ctx context.Context,
	params CreatePlanAnnotationParams,
) (PlanAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.queries.CreatePlanAnnotation(
		ctx, sqlc.CreatePlanAnnotationParams{
			PlanReviewID:   params.PlanReviewID,
			AnnotationID:   params.AnnotationID,
			BlockID:        params.BlockID,
			AnnotationType: params.AnnotationType,
			Text:           ToSqlcNullString(params.Text),
			OriginalText:   params.OriginalText,
			StartOffset:    int64(params.StartOffset),
			EndOffset:      int64(params.EndOffset),
			DiffContext:    ToSqlcNullString(params.DiffContext),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	)
	if err != nil {
		return PlanAnnotation{}, err
	}

	return PlanAnnotationFromSqlc(row), nil
}

// GetPlanAnnotation retrieves a plan annotation by its UUID.
func (s *txSqlcStore) GetPlanAnnotation(ctx context.Context,
	annotationID string,
) (PlanAnnotation, error) {
	row, err := s.queries.GetPlanAnnotation(ctx, annotationID)
	if err != nil {
		return PlanAnnotation{}, err
	}

	return PlanAnnotationFromSqlc(row), nil
}

// ListPlanAnnotationsByReview retrieves all annotations for a plan review.
func (s *txSqlcStore) ListPlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) ([]PlanAnnotation, error) {
	rows, err := s.queries.ListPlanAnnotationsByReview(ctx, planReviewID)
	if err != nil {
		return nil, err
	}

	result := make([]PlanAnnotation, 0, len(rows))
	for _, row := range rows {
		result = append(result, PlanAnnotationFromSqlc(row))
	}

	return result, nil
}

// UpdatePlanAnnotation updates a plan annotation's content.
func (s *txSqlcStore) UpdatePlanAnnotation(ctx context.Context,
	params UpdatePlanAnnotationParams,
) (PlanAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.queries.UpdatePlanAnnotation(
		ctx, sqlc.UpdatePlanAnnotationParams{
			Text:         ToSqlcNullString(params.Text),
			OriginalText: params.OriginalText,
			StartOffset:  int64(params.StartOffset),
			EndOffset:    int64(params.EndOffset),
			DiffContext:  ToSqlcNullString(params.DiffContext),
			UpdatedAt:    now,
			AnnotationID: params.AnnotationID,
		},
	)
	if err != nil {
		return PlanAnnotation{}, err
	}
	return PlanAnnotationFromSqlc(row), nil
}

// DeletePlanAnnotation deletes a plan annotation by its UUID.
func (s *txSqlcStore) DeletePlanAnnotation(ctx context.Context,
	annotationID string,
) error {
	return s.queries.DeletePlanAnnotation(ctx, annotationID)
}

// DeletePlanAnnotationsByReview deletes all annotations for a plan review.
func (s *txSqlcStore) DeletePlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) error {
	return s.queries.DeletePlanAnnotationsByReview(ctx, planReviewID)
}

// CreateDiffAnnotation creates a new diff annotation record.
func (s *txSqlcStore) CreateDiffAnnotation(ctx context.Context,
	params CreateDiffAnnotationParams,
) (DiffAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.queries.CreateDiffAnnotation(
		ctx, sqlc.CreateDiffAnnotationParams{
			AnnotationID:   params.AnnotationID,
			MessageID:      params.MessageID,
			AnnotationType: params.AnnotationType,
			Scope:          params.Scope,
			FilePath:       params.FilePath,
			LineStart:      int64(params.LineStart),
			LineEnd:        int64(params.LineEnd),
			Side:           params.Side,
			Text:           ToSqlcNullString(params.Text),
			SuggestedCode:  ToSqlcNullString(params.SuggestedCode),
			OriginalCode:   ToSqlcNullString(params.OriginalCode),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	)
	if err != nil {
		return DiffAnnotation{}, err
	}

	return DiffAnnotationFromSqlc(row), nil
}

// GetDiffAnnotation retrieves a diff annotation by its UUID.
func (s *txSqlcStore) GetDiffAnnotation(ctx context.Context,
	annotationID string,
) (DiffAnnotation, error) {
	row, err := s.queries.GetDiffAnnotation(ctx, annotationID)
	if err != nil {
		return DiffAnnotation{}, err
	}

	return DiffAnnotationFromSqlc(row), nil
}

// ListDiffAnnotationsByMessage retrieves all diff annotations for a
// message.
func (s *txSqlcStore) ListDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) ([]DiffAnnotation, error) {
	rows, err := s.queries.ListDiffAnnotationsByMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}

	result := make([]DiffAnnotation, 0, len(rows))
	for _, row := range rows {
		result = append(result, DiffAnnotationFromSqlc(row))
	}

	return result, nil
}

// UpdateDiffAnnotation updates a diff annotation's content.
func (s *txSqlcStore) UpdateDiffAnnotation(ctx context.Context,
	params UpdateDiffAnnotationParams,
) (DiffAnnotation, error) {
	now := time.Now().Unix()

	row, err := s.queries.UpdateDiffAnnotation(
		ctx, sqlc.UpdateDiffAnnotationParams{
			Text:          ToSqlcNullString(params.Text),
			SuggestedCode: ToSqlcNullString(params.SuggestedCode),
			OriginalCode:  ToSqlcNullString(params.OriginalCode),
			UpdatedAt:     now,
			AnnotationID:  params.AnnotationID,
		},
	)
	if err != nil {
		return DiffAnnotation{}, err
	}
	return DiffAnnotationFromSqlc(row), nil
}

// DeleteDiffAnnotation deletes a diff annotation by its UUID.
func (s *txSqlcStore) DeleteDiffAnnotation(ctx context.Context,
	annotationID string,
) error {
	return s.queries.DeleteDiffAnnotation(ctx, annotationID)
}

// DeleteDiffAnnotationsByMessage deletes all diff annotations for a
// message.
func (s *txSqlcStore) DeleteDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) error {
	return s.queries.DeleteDiffAnnotationsByMessage(ctx, messageID)
}
