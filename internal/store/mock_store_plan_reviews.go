package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Plan review data stores for MockStore.
// Uses the same package-level map pattern as mock_store_reviews.go to avoid
// modifying the MockStore struct.

// mockPlanReviewData holds plan review data for a MockStore instance.
type mockPlanReviewData struct {
	planReviews      map[string]PlanReview // Keyed by plan_review_id.
	nextPlanReviewID int64
}

// planReviewDataMap stores per-MockStore plan review data.
var planReviewDataMap = make(map[*MockStore]*mockPlanReviewData)

// getPlanReviewData returns or initializes plan review data for a MockStore.
func getPlanReviewData(m *MockStore) *mockPlanReviewData {
	data, ok := planReviewDataMap[m]
	if !ok {
		data = &mockPlanReviewData{
			planReviews:      make(map[string]PlanReview),
			nextPlanReviewID: 1,
		}
		planReviewDataMap[m] = data
	}
	return data
}

// =============================================================================
// PlanReviewStore implementation for MockStore
// =============================================================================

// CreatePlanReview creates a new plan review record.
func (m *MockStore) CreatePlanReview(ctx context.Context,
	params CreatePlanReviewParams,
) (PlanReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getPlanReviewData(m)

	now := time.Now()
	review := PlanReview{
		ID:           data.nextPlanReviewID,
		PlanReviewID: params.PlanReviewID,
		MessageID:    params.MessageID,
		ThreadID:     params.ThreadID,
		RequesterID:  params.RequesterID,
		ReviewerName: params.ReviewerName,
		PlanPath:     params.PlanPath,
		PlanTitle:    params.PlanTitle,
		PlanSummary:  params.PlanSummary,
		State:        "pending",
		SessionID:    params.SessionID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	data.nextPlanReviewID++
	data.planReviews[params.PlanReviewID] = review

	return review, nil
}

// GetPlanReview retrieves a plan review by its UUID.
func (m *MockStore) GetPlanReview(ctx context.Context,
	planReviewID string,
) (PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)
	review, ok := data.planReviews[planReviewID]
	if !ok {
		return PlanReview{}, fmt.Errorf(
			"plan review not found: %s", planReviewID,
		)
	}

	return review, nil
}

// GetPlanReviewByMessage retrieves a plan review by message ID.
func (m *MockStore) GetPlanReviewByMessage(ctx context.Context,
	messageID int64,
) (PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)
	for _, review := range data.planReviews {
		if review.MessageID != nil && *review.MessageID == messageID {
			return review, nil
		}
	}

	return PlanReview{}, fmt.Errorf(
		"plan review not found for message: %d", messageID,
	)
}

// GetPlanReviewByThread retrieves the latest plan review for a thread.
func (m *MockStore) GetPlanReviewByThread(ctx context.Context,
	threadID string,
) (PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)

	var latest PlanReview
	found := false
	for _, review := range data.planReviews {
		if review.ThreadID == threadID {
			if !found || review.CreatedAt.After(latest.CreatedAt) {
				latest = review
				found = true
			}
		}
	}

	if !found {
		return PlanReview{}, fmt.Errorf(
			"plan review not found for thread: %s", threadID,
		)
	}

	return latest, nil
}

// GetPlanReviewBySession retrieves the pending plan review for a session.
func (m *MockStore) GetPlanReviewBySession(ctx context.Context,
	sessionID string,
) (PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)

	var latest PlanReview
	found := false
	for _, review := range data.planReviews {
		if review.SessionID == sessionID && review.State == "pending" {
			if !found || review.CreatedAt.After(latest.CreatedAt) {
				latest = review
				found = true
			}
		}
	}

	if !found {
		return PlanReview{}, fmt.Errorf(
			"pending plan review not found for session: %s", sessionID,
		)
	}

	return latest, nil
}

// ListPlanReviews lists plan reviews ordered by creation time.
func (m *MockStore) ListPlanReviews(ctx context.Context,
	limit, offset int,
) ([]PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)

	reviews := make([]PlanReview, 0, len(data.planReviews))
	for _, r := range data.planReviews {
		reviews = append(reviews, r)
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})

	if offset >= len(reviews) {
		return nil, nil
	}
	reviews = reviews[offset:]
	if limit > 0 && limit < len(reviews) {
		reviews = reviews[:limit]
	}

	return reviews, nil
}

// ListPlanReviewsByState lists plan reviews matching the given state.
func (m *MockStore) ListPlanReviewsByState(ctx context.Context,
	state string, limit int,
) ([]PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)
	var results []PlanReview
	for _, r := range data.planReviews {
		if r.State == state {
			results = append(results, r)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// ListPlanReviewsByRequester lists plan reviews by the requesting agent.
func (m *MockStore) ListPlanReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]PlanReview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getPlanReviewData(m)
	var results []PlanReview
	for _, r := range data.planReviews {
		if r.RequesterID == requesterID {
			results = append(results, r)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// UpdatePlanReviewState updates the state and reviewer info of a plan review.
func (m *MockStore) UpdatePlanReviewState(ctx context.Context,
	params UpdatePlanReviewStateParams,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getPlanReviewData(m)
	review, ok := data.planReviews[params.PlanReviewID]
	if !ok {
		return fmt.Errorf(
			"plan review not found: %s", params.PlanReviewID,
		)
	}

	now := time.Now()
	review.State = params.State
	review.ReviewerComment = params.ReviewerComment
	review.ReviewedBy = params.ReviewedBy
	review.UpdatedAt = now
	review.ReviewedAt = &now
	data.planReviews[params.PlanReviewID] = review

	return nil
}

// DeletePlanReview deletes a plan review by its UUID.
func (m *MockStore) DeletePlanReview(ctx context.Context,
	planReviewID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getPlanReviewData(m)
	if _, ok := data.planReviews[planReviewID]; !ok {
		return fmt.Errorf(
			"plan review not found: %s", planReviewID,
		)
	}

	delete(data.planReviews, planReviewID)
	return nil
}
