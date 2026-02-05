package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Review data stores for MockStore.
// These are added as package-level maps keyed by the MockStore pointer to avoid
// modifying the MockStore struct (which would require updating NewMockStore and
// all test files). This is a pragmatic approach for mock data.

// mockReviewData holds review data for a MockStore instance.
type mockReviewData struct {
	reviews         map[string]Review        // keyed by review_id
	reviewIssues    map[string][]ReviewIssue // keyed by review_id
	reviewIters     map[string][]ReviewIteration
	nextReviewID    int64
	nextIssueID     int64
	nextIterationID int64
}

// reviewDataMap stores per-MockStore review data.
var reviewDataMap = make(map[*MockStore]*mockReviewData)

// getReviewData returns or initializes review data for a MockStore.
func getReviewData(m *MockStore) *mockReviewData {
	// We use m.mu for synchronization (already locked by callers).
	data, ok := reviewDataMap[m]
	if !ok {
		data = &mockReviewData{
			reviews:         make(map[string]Review),
			reviewIssues:    make(map[string][]ReviewIssue),
			reviewIters:     make(map[string][]ReviewIteration),
			nextReviewID:    1,
			nextIssueID:     1,
			nextIterationID: 1,
		}
		reviewDataMap[m] = data
	}
	return data
}

// =============================================================================
// ReviewStore implementation for MockStore
// =============================================================================

// CreateReview creates a new review record.
func (m *MockStore) CreateReview(ctx context.Context,
	params CreateReviewParams,
) (Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)

	now := time.Now()
	review := Review{
		ID:          data.nextReviewID,
		ReviewID:    params.ReviewID,
		ThreadID:    params.ThreadID,
		RequesterID: params.RequesterID,
		PRNumber:    params.PRNumber,
		Branch:      params.Branch,
		BaseBranch:  params.BaseBranch,
		CommitSHA:   params.CommitSHA,
		RepoPath:    params.RepoPath,
		RemoteURL:   params.RemoteURL,
		ReviewType:  params.ReviewType,
		Priority:    params.Priority,
		State:       "new",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	data.nextReviewID++
	data.reviews[params.ReviewID] = review

	return review, nil
}

// GetReview retrieves a review by its UUID.
func (m *MockStore) GetReview(ctx context.Context,
	reviewID string,
) (Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	review, ok := data.reviews[reviewID]
	if !ok {
		return Review{}, fmt.Errorf("review not found: %s", reviewID)
	}

	return review, nil
}

// ListReviews lists reviews ordered by creation time.
func (m *MockStore) ListReviews(ctx context.Context,
	limit, offset int,
) ([]Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)

	// Collect all reviews and sort by created_at DESC.
	reviews := make([]Review, 0, len(data.reviews))
	for _, r := range data.reviews {
		reviews = append(reviews, r)
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})

	// Apply offset and limit.
	if offset >= len(reviews) {
		return nil, nil
	}
	reviews = reviews[offset:]
	if limit > 0 && limit < len(reviews) {
		reviews = reviews[:limit]
	}

	return reviews, nil
}

// ListReviewsByState lists reviews matching the given state.
func (m *MockStore) ListReviewsByState(ctx context.Context,
	state string, limit int,
) ([]Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	var results []Review
	for _, r := range data.reviews {
		if r.State == state {
			results = append(results, r)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// ListReviewsByRequester lists reviews by the requesting agent.
func (m *MockStore) ListReviewsByRequester(ctx context.Context,
	requesterID int64, limit int,
) ([]Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	var results []Review
	for _, r := range data.reviews {
		if r.RequesterID == requesterID {
			results = append(results, r)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// ListActiveReviews returns reviews in non-terminal states.
func (m *MockStore) ListActiveReviews(
	ctx context.Context,
) ([]Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	terminalStates := map[string]bool{
		"approved":  true,
		"rejected":  true,
		"cancelled": true,
	}

	var results []Review
	for _, r := range data.reviews {
		if !terminalStates[r.State] {
			results = append(results, r)
		}
	}

	return results, nil
}

// UpdateReviewState updates the FSM state of a review.
func (m *MockStore) UpdateReviewState(ctx context.Context,
	reviewID, state string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)
	review, ok := data.reviews[reviewID]
	if !ok {
		return fmt.Errorf("review not found: %s", reviewID)
	}

	review.State = state
	review.UpdatedAt = time.Now()
	data.reviews[reviewID] = review

	return nil
}

// UpdateReviewCompleted marks a review as completed with a terminal state.
func (m *MockStore) UpdateReviewCompleted(ctx context.Context,
	reviewID, state string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)
	review, ok := data.reviews[reviewID]
	if !ok {
		return fmt.Errorf("review not found: %s", reviewID)
	}

	now := time.Now()
	review.State = state
	review.UpdatedAt = now
	review.CompletedAt = &now
	data.reviews[reviewID] = review

	return nil
}

// CreateReviewIteration records a review iteration result.
func (m *MockStore) CreateReviewIteration(ctx context.Context,
	params CreateReviewIterationParams,
) (ReviewIteration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)

	iter := ReviewIteration{
		ID:                data.nextIterationID,
		ReviewID:          params.ReviewID,
		IterationNum:      params.IterationNum,
		ReviewerID:        params.ReviewerID,
		ReviewerSessionID: params.ReviewerSessionID,
		Decision:          params.Decision,
		Summary:           params.Summary,
		IssuesJSON:        params.IssuesJSON,
		SuggestionsJSON:   params.SuggestionsJSON,
		FilesReviewed:     params.FilesReviewed,
		LinesAnalyzed:     params.LinesAnalyzed,
		DurationMS:        params.DurationMS,
		CostUSD:           params.CostUSD,
		StartedAt:         params.StartedAt,
		CompletedAt:       params.CompletedAt,
	}
	data.nextIterationID++
	data.reviewIters[params.ReviewID] = append(
		data.reviewIters[params.ReviewID], iter,
	)

	return iter, nil
}

// GetReviewIterations gets all iterations for a review.
func (m *MockStore) GetReviewIterations(ctx context.Context,
	reviewID string,
) ([]ReviewIteration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	return data.reviewIters[reviewID], nil
}

// CreateReviewIssue records a specific issue found during review.
func (m *MockStore) CreateReviewIssue(ctx context.Context,
	params CreateReviewIssueParams,
) (ReviewIssue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)

	issue := ReviewIssue{
		ID:           data.nextIssueID,
		ReviewID:     params.ReviewID,
		IterationNum: params.IterationNum,
		IssueType:    params.IssueType,
		Severity:     params.Severity,
		FilePath:     params.FilePath,
		LineStart:    params.LineStart,
		LineEnd:      params.LineEnd,
		Title:        params.Title,
		Description:  params.Description,
		CodeSnippet:  params.CodeSnippet,
		Suggestion:   params.Suggestion,
		ClaudeMDRef:  params.ClaudeMDRef,
		Status:       "open",
		CreatedAt:    time.Now(),
	}
	data.nextIssueID++
	data.reviewIssues[params.ReviewID] = append(
		data.reviewIssues[params.ReviewID], issue,
	)

	return issue, nil
}

// GetReviewIssues gets all issues for a review.
func (m *MockStore) GetReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	return data.reviewIssues[reviewID], nil
}

// GetOpenReviewIssues gets open issues for a review.
func (m *MockStore) GetOpenReviewIssues(ctx context.Context,
	reviewID string,
) ([]ReviewIssue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	var results []ReviewIssue
	for _, issue := range data.reviewIssues[reviewID] {
		if issue.Status == "open" {
			results = append(results, issue)
		}
	}

	return results, nil
}

// UpdateReviewIssueStatus updates an issue's resolution status.
func (m *MockStore) UpdateReviewIssueStatus(ctx context.Context,
	issueID int64, status string, resolvedInIteration *int,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)

	for reviewID, issues := range data.reviewIssues {
		for i, issue := range issues {
			if issue.ID == issueID {
				issues[i].Status = status
				if status == "fixed" || status == "wont_fix" ||
					status == "duplicate" {

					now := time.Now()
					issues[i].ResolvedAt = &now
				}
				issues[i].ResolvedInIteration = resolvedInIteration
				data.reviewIssues[reviewID] = issues

				return nil
			}
		}
	}

	return fmt.Errorf("review issue not found: %d", issueID)
}

// CountOpenIssues counts open issues for a review.
func (m *MockStore) CountOpenIssues(ctx context.Context,
	reviewID string,
) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := getReviewData(m)
	var count int64
	for _, issue := range data.reviewIssues[reviewID] {
		if issue.Status == "open" {
			count++
		}
	}

	return count, nil
}

// DeleteReview deletes a review and its associated data.
func (m *MockStore) DeleteReview(ctx context.Context,
	reviewID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := getReviewData(m)
	if _, ok := data.reviews[reviewID]; !ok {
		return fmt.Errorf("review not found: %s", reviewID)
	}

	delete(data.reviewIssues, reviewID)
	delete(data.reviewIters, reviewID)
	delete(data.reviews, reviewID)

	return nil
}
