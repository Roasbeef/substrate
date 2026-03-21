package store

import (
	"context"
	"fmt"
	"time"
)

// =============================================================================
// Plan annotation mock implementation
// =============================================================================

// CreatePlanAnnotation creates a new plan annotation record.
func (m *MockStore) CreatePlanAnnotation(ctx context.Context,
	params CreatePlanAnnotationParams,
) (PlanAnnotation, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	ann := PlanAnnotation{
		ID:             m.nextPlanAnnotationID,
		PlanReviewID:   params.PlanReviewID,
		AnnotationID:   params.AnnotationID,
		BlockID:        params.BlockID,
		AnnotationType: params.AnnotationType,
		Text:           params.Text,
		OriginalText:   params.OriginalText,
		StartOffset:    params.StartOffset,
		EndOffset:      params.EndOffset,
		DiffContext:    params.DiffContext,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.planAnnotations[params.AnnotationID] = ann
	m.nextPlanAnnotationID++

	return ann, nil
}

// GetPlanAnnotation retrieves a plan annotation by its UUID.
func (m *MockStore) GetPlanAnnotation(ctx context.Context,
	annotationID string,
) (PlanAnnotation, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	ann, ok := m.planAnnotations[annotationID]
	if !ok {
		return PlanAnnotation{}, fmt.Errorf(
			"plan annotation not found: %s", annotationID,
		)
	}

	return ann, nil
}

// ListPlanAnnotationsByReview retrieves all annotations for a plan
// review.
func (m *MockStore) ListPlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) ([]PlanAnnotation, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []PlanAnnotation
	for _, ann := range m.planAnnotations {
		if ann.PlanReviewID == planReviewID {
			result = append(result, ann)
		}
	}

	return result, nil
}

// UpdatePlanAnnotation updates a plan annotation's content.
func (m *MockStore) UpdatePlanAnnotation(ctx context.Context,
	params UpdatePlanAnnotationParams,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	ann, ok := m.planAnnotations[params.AnnotationID]
	if !ok {
		return fmt.Errorf(
			"plan annotation not found: %s", params.AnnotationID,
		)
	}

	ann.Text = params.Text
	ann.OriginalText = params.OriginalText
	ann.StartOffset = params.StartOffset
	ann.EndOffset = params.EndOffset
	ann.DiffContext = params.DiffContext
	ann.UpdatedAt = time.Now()
	m.planAnnotations[params.AnnotationID] = ann

	return nil
}

// DeletePlanAnnotation deletes a plan annotation by its UUID.
func (m *MockStore) DeletePlanAnnotation(ctx context.Context,
	annotationID string,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.planAnnotations, annotationID)

	return nil
}

// DeletePlanAnnotationsByReview deletes all annotations for a plan
// review.
func (m *MockStore) DeletePlanAnnotationsByReview(ctx context.Context,
	planReviewID string,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ann := range m.planAnnotations {
		if ann.PlanReviewID == planReviewID {
			delete(m.planAnnotations, id)
		}
	}

	return nil
}

// =============================================================================
// Diff annotation mock implementation
// =============================================================================

// CreateDiffAnnotation creates a new diff annotation record.
func (m *MockStore) CreateDiffAnnotation(ctx context.Context,
	params CreateDiffAnnotationParams,
) (DiffAnnotation, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	ann := DiffAnnotation{
		ID:             m.nextDiffAnnotationID,
		AnnotationID:   params.AnnotationID,
		MessageID:      params.MessageID,
		AnnotationType: params.AnnotationType,
		Scope:          params.Scope,
		FilePath:       params.FilePath,
		LineStart:      params.LineStart,
		LineEnd:        params.LineEnd,
		Side:           params.Side,
		Text:           params.Text,
		SuggestedCode:  params.SuggestedCode,
		OriginalCode:   params.OriginalCode,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.diffAnnotations[params.AnnotationID] = ann
	m.nextDiffAnnotationID++

	return ann, nil
}

// GetDiffAnnotation retrieves a diff annotation by its UUID.
func (m *MockStore) GetDiffAnnotation(ctx context.Context,
	annotationID string,
) (DiffAnnotation, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	ann, ok := m.diffAnnotations[annotationID]
	if !ok {
		return DiffAnnotation{}, fmt.Errorf(
			"diff annotation not found: %s", annotationID,
		)
	}

	return ann, nil
}

// ListDiffAnnotationsByMessage retrieves all diff annotations for a
// message.
func (m *MockStore) ListDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) ([]DiffAnnotation, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []DiffAnnotation
	for _, ann := range m.diffAnnotations {
		if ann.MessageID == messageID {
			result = append(result, ann)
		}
	}

	return result, nil
}

// UpdateDiffAnnotation updates a diff annotation's content.
func (m *MockStore) UpdateDiffAnnotation(ctx context.Context,
	params UpdateDiffAnnotationParams,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	ann, ok := m.diffAnnotations[params.AnnotationID]
	if !ok {
		return fmt.Errorf(
			"diff annotation not found: %s", params.AnnotationID,
		)
	}

	ann.Text = params.Text
	ann.SuggestedCode = params.SuggestedCode
	ann.OriginalCode = params.OriginalCode
	ann.UpdatedAt = time.Now()
	m.diffAnnotations[params.AnnotationID] = ann

	return nil
}

// DeleteDiffAnnotation deletes a diff annotation by its UUID.
func (m *MockStore) DeleteDiffAnnotation(ctx context.Context,
	annotationID string,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.diffAnnotations, annotationID)

	return nil
}

// DeleteDiffAnnotationsByMessage deletes all diff annotations for a
// message.
func (m *MockStore) DeleteDiffAnnotationsByMessage(ctx context.Context,
	messageID int64,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ann := range m.diffAnnotations {
		if ann.MessageID == messageID {
			delete(m.diffAnnotations, id)
		}
	}

	return nil
}
