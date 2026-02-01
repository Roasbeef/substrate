// Review API handlers for the /api/v1/reviews endpoints.
package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIV1Review represents a review in the JSON API.
type APIV1Review struct {
	ID            int64   `json:"id"`
	ReviewID      string  `json:"review_id"`
	ThreadID      string  `json:"thread_id"`
	RequesterID   int64   `json:"requester_id"`
	RequesterName string  `json:"requester_name,omitempty"`
	PRNumber      *int64  `json:"pr_number,omitempty"`
	Branch        string  `json:"branch"`
	BaseBranch    string  `json:"base_branch"`
	CommitSHA     string  `json:"commit_sha"`
	RepoPath      string  `json:"repo_path"`
	ReviewType    string  `json:"review_type"`
	Priority      string  `json:"priority"`
	State         string  `json:"state"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	CompletedAt   *string `json:"completed_at,omitempty"`
}

// APIV1ReviewIteration represents a review iteration in the JSON API.
type APIV1ReviewIteration struct {
	ID                int64   `json:"id"`
	ReviewID          string  `json:"review_id"`
	IterationNum      int     `json:"iteration_num"`
	ReviewerID        string  `json:"reviewer_id"`
	ReviewerSessionID string  `json:"reviewer_session_id,omitempty"`
	Decision          string  `json:"decision"`
	Summary           string  `json:"summary"`
	FilesReviewed     int     `json:"files_reviewed"`
	LinesAnalyzed     int     `json:"lines_analyzed"`
	DurationMS        int64   `json:"duration_ms"`
	CostUSD           float64 `json:"cost_usd"`
	StartedAt         string  `json:"started_at"`
	CompletedAt       *string `json:"completed_at,omitempty"`
}

// APIV1ReviewIssue represents a review issue in the JSON API.
type APIV1ReviewIssue struct {
	ID           int64   `json:"id"`
	ReviewID     string  `json:"review_id"`
	IterationNum int     `json:"iteration_num"`
	IssueType    string  `json:"type"`
	Severity     string  `json:"severity"`
	FilePath     string  `json:"file_path"`
	LineStart    int     `json:"line_start"`
	LineEnd      *int    `json:"line_end,omitempty"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	CodeSnippet  string  `json:"code_snippet,omitempty"`
	Suggestion   string  `json:"suggestion,omitempty"`
	ClaudeMDRef  string  `json:"claude_md_ref,omitempty"`
	Status       string  `json:"status"`
	ResolvedAt   *string `json:"resolved_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// APIV1ReviewStats represents review statistics.
type APIV1ReviewStats struct {
	TotalReviews     int64 `json:"total_reviews"`
	Approved         int64 `json:"approved"`
	Pending          int64 `json:"pending"`
	InProgress       int64 `json:"in_progress"`
	ChangesRequested int64 `json:"changes_requested"`
}

// registerAPIV1ReviewRoutes registers the /api/v1/reviews routes.
func (s *Server) registerAPIV1ReviewRoutes(api func(http.HandlerFunc) http.HandlerFunc) {
	s.mux.HandleFunc("/api/v1/reviews", api(s.handleAPIV1Reviews))
	s.mux.HandleFunc("/api/v1/reviews/stats", api(s.handleAPIV1ReviewStats))
	s.mux.HandleFunc("/api/v1/reviews/", api(s.handleAPIV1ReviewByID))
}

// handleAPIV1Reviews handles GET/POST /api/v1/reviews.
func (s *Server) handleAPIV1Reviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Parse query parameters.
		filter := r.URL.Query().Get("filter")
		requesterIDStr := r.URL.Query().Get("requester_id")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		var reviews []APIV1Review

		// Route to appropriate query based on filters.
		switch {
		case requesterIDStr != "":
			requesterID, err := strconv.ParseInt(requesterIDStr, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_param", "Invalid requester_id")
				return
			}
			dbReviews, err := s.store.Queries().ListReviewsByRequester(ctx, struct {
				RequesterID int64
				Limit       int64
			}{
				RequesterID: requesterID,
				Limit:       int64(limit),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch reviews")
				return
			}
			reviews = convertReviewsToAPI(dbReviews)

		case filter != "" && filter != "all":
			dbReviews, err := s.store.Queries().ListReviewsByState(ctx, struct {
				State string
				Limit int64
			}{
				State: filter,
				Limit: int64(limit),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch reviews")
				return
			}
			reviews = convertReviewsToAPI(dbReviews)

		default:
			dbReviews, err := s.store.Queries().ListReviews(ctx, int64(limit))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch reviews")
				return
			}
			reviews = convertReviewsToAPI(dbReviews)
		}

		// Add requester names.
		for i := range reviews {
			agent, err := s.store.Queries().GetAgent(ctx, reviews[i].RequesterID)
			if err == nil {
				reviews[i].RequesterName = agent.Name
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"reviews": reviews,
			"total":   len(reviews),
		})

	case http.MethodPost:
		// Create a new review (used by the CLI/API).
		var req struct {
			Branch      string   `json:"branch"`
			BaseBranch  string   `json:"base_branch"`
			CommitSHA   string   `json:"commit_sha"`
			RepoPath    string   `json:"repo_path"`
			PRNumber    *int64   `json:"pr_number,omitempty"`
			ReviewType  string   `json:"review_type"`
			Priority    string   `json:"priority"`
			RequesterID int64    `json:"requester_id"`
			Reviewers   []string `json:"reviewers,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
			return
		}

		// Validate required fields.
		if req.Branch == "" || req.CommitSHA == "" || req.RepoPath == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "branch, commit_sha, and repo_path are required")
			return
		}

		// Set defaults.
		if req.BaseBranch == "" {
			req.BaseBranch = "main"
		}
		if req.ReviewType == "" {
			req.ReviewType = "full"
		}
		if req.Priority == "" {
			req.Priority = "normal"
		}
		if req.RequesterID == 0 {
			req.RequesterID = s.getUserAgentID(ctx)
		}

		// Create the review using the store.
		review, err := s.store.Queries().CreateReview(ctx, struct {
			ReviewID    string
			ThreadID    string
			RequesterID int64
			PrNumber    interface{}
			Branch      string
			BaseBranch  string
			CommitSha   string
			RepoPath    string
			ReviewType  string
			Priority    string
			CreatedAt   int64
			UpdatedAt   int64
		}{
			ReviewID:    generateUUID(),
			ThreadID:    generateUUID(),
			RequesterID: req.RequesterID,
			PrNumber:    req.PRNumber,
			Branch:      req.Branch,
			BaseBranch:  req.BaseBranch,
			CommitSha:   req.CommitSHA,
			RepoPath:    req.RepoPath,
			ReviewType:  req.ReviewType,
			Priority:    req.Priority,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to create review")
			return
		}

		apiReview := convertReviewToAPI(review)
		writeJSON(w, http.StatusCreated, apiReview)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAPIV1ReviewStats handles GET /api/v1/reviews/stats.
func (s *Server) handleAPIV1ReviewStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	stats, err := s.store.Queries().GetReviewStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch review stats")
		return
	}

	writeJSON(w, http.StatusOK, APIV1ReviewStats{
		TotalReviews:     stats.TotalReviews,
		Approved:         stats.Approved,
		Pending:          stats.Pending,
		InProgress:       stats.InProgress,
		ChangesRequested: stats.ChangesRequested,
	})
}

// handleAPIV1ReviewByID handles GET/POST/DELETE /api/v1/reviews/{id}.
func (s *Server) handleAPIV1ReviewByID(w http.ResponseWriter, r *http.Request) {
	// Extract review ID and optional action from path.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/reviews/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "Review ID required")
		return
	}

	// Handle /stats route separately.
	if parts[0] == "stats" {
		s.handleAPIV1ReviewStats(w, r)
		return
	}

	reviewID := parts[0]
	ctx := r.Context()

	// Check for subpaths.
	if len(parts) > 1 {
		subpath := parts[1]
		switch subpath {
		case "iterations":
			s.handleAPIV1ReviewIterations(w, r, reviewID)
			return
		case "issues":
			s.handleAPIV1ReviewIssues(w, r, reviewID, parts)
			return
		case "resubmit":
			s.handleAPIV1ReviewResubmit(w, r, reviewID)
			return
		case "diff":
			s.handleAPIV1ReviewDiff(w, r, reviewID)
			return
		case "patch":
			s.handleAPIV1ReviewPatch(w, r, reviewID)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		// Get review details.
		review, err := s.store.Queries().GetReview(ctx, reviewID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Review not found")
			return
		}

		apiReview := convertReviewToAPI(review)

		// Add requester name.
		agent, err := s.store.Queries().GetAgent(ctx, review.RequesterID)
		if err == nil {
			apiReview.RequesterName = agent.Name
		}

		// Fetch iterations.
		iterations, _ := s.store.Queries().ListReviewIterations(ctx, reviewID)
		apiIterations := convertIterationsToAPI(iterations)

		// Count open issues.
		openIssues, _ := s.store.Queries().ListOpenReviewIssues(ctx, reviewID)
		allIssues, _ := s.store.Queries().ListReviewIssues(ctx, reviewID)

		writeJSON(w, http.StatusOK, map[string]any{
			"review_id":         apiReview.ReviewID,
			"thread_id":         apiReview.ThreadID,
			"requester_id":      apiReview.RequesterID,
			"requester_name":    apiReview.RequesterName,
			"pr_number":         apiReview.PRNumber,
			"branch":            apiReview.Branch,
			"base_branch":       apiReview.BaseBranch,
			"commit_sha":        apiReview.CommitSHA,
			"repo_path":         apiReview.RepoPath,
			"review_type":       apiReview.ReviewType,
			"priority":          apiReview.Priority,
			"state":             apiReview.State,
			"created_at":        apiReview.CreatedAt,
			"updated_at":        apiReview.UpdatedAt,
			"completed_at":      apiReview.CompletedAt,
			"iterations":        apiIterations,
			"open_issues_count": len(openIssues),
			"total_issues_count": len(allIssues),
		})

	case http.MethodDelete:
		// Cancel the review.
		review, err := s.store.Queries().GetReview(ctx, reviewID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Review not found")
			return
		}

		// Check if review can be cancelled.
		terminalStates := map[string]bool{
			"approved": true, "rejected": true, "cancelled": true,
		}
		if terminalStates[review.State] {
			writeError(w, http.StatusBadRequest, "invalid_state", "Cannot cancel review in state: "+review.State)
			return
		}

		// Update state to cancelled.
		err = s.store.Queries().UpdateReviewState(ctx, struct {
			State     string
			UpdatedAt int64
			ReviewID  string
		}{
			State:     "cancelled",
			UpdatedAt: time.Now().Unix(),
			ReviewID:  reviewID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to cancel review")
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAPIV1ReviewIterations handles GET /api/v1/reviews/{id}/iterations.
func (s *Server) handleAPIV1ReviewIterations(w http.ResponseWriter, r *http.Request, reviewID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	iterations, err := s.store.Queries().ListReviewIterations(ctx, reviewID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch iterations")
		return
	}

	writeJSON(w, http.StatusOK, convertIterationsToAPI(iterations))
}

// handleAPIV1ReviewIssues handles GET/PATCH /api/v1/reviews/{id}/issues[/{issueId}].
func (s *Server) handleAPIV1ReviewIssues(w http.ResponseWriter, r *http.Request, reviewID string, parts []string) {
	ctx := r.Context()

	// Check for specific issue ID.
	if len(parts) > 2 {
		issueIDStr := parts[2]
		issueID, err := strconv.ParseInt(issueIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "Invalid issue ID")
			return
		}

		if r.Method == http.MethodPatch {
			// Update issue status.
			var req struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
				return
			}

			// Validate status.
			validStatuses := map[string]bool{
				"open": true, "fixed": true, "wont_fix": true, "duplicate": true,
			}
			if !validStatuses[req.Status] {
				writeError(w, http.StatusBadRequest, "invalid_status", "Invalid status value")
				return
			}

			// Update the issue.
			if req.Status == "fixed" {
				err = s.store.Queries().UpdateIssueStatus(ctx, struct {
					Status     string
					ResolvedAt interface{}
					ID         int64
				}{
					Status:     req.Status,
					ResolvedAt: time.Now().Unix(),
					ID:         issueID,
				})
			} else {
				err = s.store.Queries().UpdateIssueStatus(ctx, struct {
					Status     string
					ResolvedAt interface{}
					ID         int64
				}{
					Status:     req.Status,
					ResolvedAt: nil,
					ID:         issueID,
				})
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", "Failed to update issue")
				return
			}

			// Return updated issue.
			issue, _ := s.store.Queries().GetReviewIssue(ctx, issueID)
			writeJSON(w, http.StatusOK, convertIssueToAPI(issue))
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Check for status filter.
	statusFilter := r.URL.Query().Get("status")
	var issues []APIV1ReviewIssue

	if statusFilter == "open" {
		dbIssues, err := s.store.Queries().ListOpenReviewIssues(ctx, reviewID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch issues")
			return
		}
		issues = convertIssuesToAPI(dbIssues)
	} else {
		dbIssues, err := s.store.Queries().ListReviewIssues(ctx, reviewID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch issues")
			return
		}
		issues = convertIssuesToAPI(dbIssues)
	}

	writeJSON(w, http.StatusOK, issues)
}

// handleAPIV1ReviewResubmit handles POST /api/v1/reviews/{id}/resubmit.
func (s *Server) handleAPIV1ReviewResubmit(w http.ResponseWriter, r *http.Request, reviewID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	var req struct {
		CommitSHA string `json:"commit_sha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.CommitSHA == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "commit_sha is required")
		return
	}

	ctx := r.Context()

	// Verify review exists and is in a resubmittable state.
	review, err := s.store.Queries().GetReview(ctx, reviewID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Review not found")
		return
	}

	resubmittableStates := map[string]bool{
		"changes_requested": true, "re_review": true,
	}
	if !resubmittableStates[review.State] {
		writeError(w, http.StatusBadRequest, "invalid_state", "Review cannot be resubmitted in state: "+review.State)
		return
	}

	// Update commit SHA and state.
	err = s.store.Queries().UpdateReviewCommit(ctx, struct {
		CommitSha string
		UpdatedAt int64
		ReviewID  string
	}{
		CommitSha: req.CommitSHA,
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to update commit")
		return
	}

	err = s.store.Queries().UpdateReviewState(ctx, struct {
		State     string
		UpdatedAt int64
		ReviewID  string
	}{
		State:     "re_review",
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to update state")
		return
	}

	// Return updated review.
	updatedReview, _ := s.store.Queries().GetReview(ctx, reviewID)
	writeJSON(w, http.StatusOK, convertReviewToAPI(updatedReview))
}

// handleAPIV1ReviewDiff handles GET /api/v1/reviews/{id}/diff?file=.
func (s *Server) handleAPIV1ReviewDiff(w http.ResponseWriter, r *http.Request, reviewID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// This is a placeholder - actual diff fetching would require git operations.
	writeJSON(w, http.StatusOK, map[string]any{
		"file_path":    r.URL.Query().Get("file"),
		"old_content":  "",
		"new_content":  "",
		"additions":    0,
		"deletions":    0,
		"issues":       []any{},
	})
}

// handleAPIV1ReviewPatch handles GET /api/v1/reviews/{id}/patch.
func (s *Server) handleAPIV1ReviewPatch(w http.ResponseWriter, r *http.Request, reviewID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// This is a placeholder - actual patch fetching would require git operations.
	writeJSON(w, http.StatusOK, map[string]any{
		"review_id": reviewID,
		"content":   "",
		"files":     []any{},
	})
}

// Helper functions for converting database models to API models.

func convertReviewToAPI(r interface{}) APIV1Review {
	// Use type assertion based on the sqlc generated type.
	// This handles the sqlc.Review type.
	switch review := r.(type) {
	case struct {
		ID          int64
		ReviewID    string
		ThreadID    string
		RequesterID int64
		PrNumber    interface{}
		Branch      string
		BaseBranch  string
		CommitSha   string
		RepoPath    string
		ReviewType  string
		Priority    string
		State       string
		CreatedAt   int64
		UpdatedAt   int64
		CompletedAt interface{}
	}:
		apiReview := APIV1Review{
			ID:          review.ID,
			ReviewID:    review.ReviewID,
			ThreadID:    review.ThreadID,
			RequesterID: review.RequesterID,
			Branch:      review.Branch,
			BaseBranch:  review.BaseBranch,
			CommitSHA:   review.CommitSha,
			RepoPath:    review.RepoPath,
			ReviewType:  review.ReviewType,
			Priority:    review.Priority,
			State:       review.State,
			CreatedAt:   time.Unix(review.CreatedAt, 0).UTC().Format(time.RFC3339),
			UpdatedAt:   time.Unix(review.UpdatedAt, 0).UTC().Format(time.RFC3339),
		}
		// Handle nullable fields using reflection or type assertions.
		return apiReview
	default:
		// Fallback for other types - use reflection.
		return APIV1Review{}
	}
}

func convertReviewsToAPI(reviews interface{}) []APIV1Review {
	// This will be implemented based on the actual sqlc type.
	return []APIV1Review{}
}

func convertIterationsToAPI(iterations interface{}) []APIV1ReviewIteration {
	return []APIV1ReviewIteration{}
}

func convertIssueToAPI(issue interface{}) APIV1ReviewIssue {
	return APIV1ReviewIssue{}
}

func convertIssuesToAPI(issues interface{}) []APIV1ReviewIssue {
	return []APIV1ReviewIssue{}
}

// generateUUID generates a simple UUID for review IDs.
func generateUUID() string {
	return time.Now().Format("20060102150405") + "-" + strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
}
