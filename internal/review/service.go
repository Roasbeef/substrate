package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/store"
)

// Service handles review orchestration and can spawn structured analysis.
type Service struct {
	store store.Storage
	log   *slog.Logger

	// Spawner for one-shot structured analysis
	spawner *agent.Spawner

	// Registered reviewer configurations (for validation/routing)
	reviewers map[string]*ReviewerConfig

	// Active reviews being tracked (in-memory FSMs)
	mu            sync.RWMutex
	activeReviews map[string]*ReviewFSM

	// Default multi-reviewer config
	defaultConfig *MultiReviewConfig

	// Structured review prompt template (parsed once)
	promptTemplate *template.Template
}

// ServiceOption is a functional option for configuring the review service.
type ServiceOption func(*Service)

// WithSpawner sets a custom spawner for the service.
func WithSpawner(spawner *agent.Spawner) ServiceOption {
	return func(s *Service) {
		s.spawner = spawner
	}
}

// WithLogger sets a custom logger for the service.
func WithLogger(log *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.log = log
	}
}

// WithMultiReviewConfig sets a custom multi-review configuration.
func WithMultiReviewConfig(config *MultiReviewConfig) ServiceOption {
	return func(s *Service) {
		s.defaultConfig = config
	}
}

// NewService creates a new review service.
func NewService(s store.Storage, opts ...ServiceOption) *Service {
	// Parse the prompt template once
	tmpl, err := template.New("structured_review").Parse(StructuredReviewPromptTemplate)
	if err != nil {
		// This should never happen since the template is a constant
		panic(fmt.Sprintf("failed to parse review prompt template: %v", err))
	}

	svc := &Service{
		store:          s,
		log:            slog.Default(),
		reviewers:      SpecializedReviewers(),
		activeReviews:  make(map[string]*ReviewFSM),
		defaultConfig:  DefaultMultiReviewConfig(),
		promptTemplate: tmpl,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
}

// RequestReview creates a new review request and returns the review ID.
func (s *Service) RequestReview(ctx context.Context,
	req ReviewRequest) (string, error) {

	reviewID := uuid.New().String()
	threadID := req.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
	}

	// Convert PRNumber to pointer
	var prNum *int64
	if req.PRNumber > 0 {
		n := int64(req.PRNumber)
		prNum = &n
	}

	// Create review in database
	_, err := s.store.CreateReview(ctx, store.CreateReviewParams{
		ReviewID:    reviewID,
		ThreadID:    threadID,
		RequesterID: req.RequesterID,
		PRNumber:    prNum,
		Branch:      req.Branch,
		BaseBranch:  req.BaseBranch,
		CommitSHA:   req.CommitSHA,
		RepoPath:    req.RepoPath,
		ReviewType:  string(req.ReviewType),
		Priority:    string(req.Priority),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create review: %w", err)
	}

	// Create in-memory FSM
	s.mu.Lock()
	s.activeReviews[reviewID] = NewReviewFSM(reviewID, threadID, s.defaultConfig)
	s.mu.Unlock()

	// Transition to pending_review
	if err := s.transitionState(ctx, reviewID, SubmitForReviewEvent{
		RequesterID: req.RequesterID,
	}); err != nil {
		return "", fmt.Errorf("failed to transition state: %w", err)
	}

	s.log.Info("Review requested",
		"review_id", reviewID,
		"branch", req.Branch,
		"requester_id", req.RequesterID,
	)

	return reviewID, nil
}

// GetReview retrieves a review by ID.
func (s *Service) GetReview(ctx context.Context,
	reviewID string) (store.Review, error) {

	return s.store.GetReview(ctx, reviewID)
}

// ListPendingReviews returns reviews waiting for review.
func (s *Service) ListPendingReviews(ctx context.Context,
	limit int) ([]store.Review, error) {

	return s.store.ListPendingReviews(ctx, limit)
}

// ListActiveReviews returns reviews that are in progress.
func (s *Service) ListActiveReviews(ctx context.Context,
	limit int) ([]store.Review, error) {

	return s.store.ListActiveReviews(ctx, limit)
}

// StartReview marks a review as being actively reviewed.
func (s *Service) StartReview(ctx context.Context, reviewID,
	reviewerID string) error {

	return s.transitionState(ctx, reviewID, StartReviewEvent{
		ReviewerID: reviewerID,
	})
}

// SubmitReviewIteration records a review iteration from a reviewer.
func (s *Service) SubmitReviewIteration(ctx context.Context,
	resp ReviewResponse) error {

	// Get current iteration count
	iterCount, err := s.store.GetIterationCount(ctx, resp.ReviewID)
	if err != nil {
		return fmt.Errorf("failed to get iteration count: %w", err)
	}
	iterNum := iterCount + 1

	// Create iteration record
	_, err = s.store.CreateReviewIteration(ctx, store.CreateReviewIterationParams{
		ReviewID:      resp.ReviewID,
		IterationNum:  iterNum,
		ReviewerID:    resp.ReviewerName,
		Decision:      string(resp.Decision),
		Summary:       resp.Summary,
		FilesReviewed: resp.FilesReviewed,
		LinesAnalyzed: resp.LinesAnalyzed,
		DurationMS:    resp.DurationMS,
		CostUSD:       resp.CostUSD,
	})
	if err != nil {
		return fmt.Errorf("failed to create iteration: %w", err)
	}

	// Create issue records
	for _, issue := range resp.Issues {
		var lineEnd *int
		if issue.LineEnd > 0 {
			lineEnd = &issue.LineEnd
		}

		_, err := s.store.CreateReviewIssue(ctx, store.CreateReviewIssueParams{
			ReviewID:     resp.ReviewID,
			IterationNum: iterNum,
			IssueType:    string(issue.Type),
			Severity:     string(issue.Severity),
			FilePath:     issue.File,
			LineStart:    issue.LineStart,
			LineEnd:      lineEnd,
			Title:        issue.Title,
			Description:  issue.Description,
			CodeSnippet:  issue.CodeSnippet,
			Suggestion:   issue.Suggestion,
			ClaudeMDRef:  issue.ClaudeMDRef,
		})
		if err != nil {
			s.log.Warn("Failed to create issue record",
				"error", err,
				"review_id", resp.ReviewID,
				"issue_title", issue.Title,
			)
		}
	}

	// Transition state based on decision
	switch resp.Decision {
	case DecisionApprove:
		return s.transitionState(ctx, resp.ReviewID, ApproveEvent{
			ReviewerID: resp.ReviewerName,
		})
	case DecisionRequestChanges:
		return s.transitionState(ctx, resp.ReviewID, RequestChangesEvent{
			ReviewerID: resp.ReviewerName,
			Issues:     resp.Issues,
		})
	case DecisionComment:
		// Comments don't change state
		return nil
	default:
		return fmt.Errorf("unknown decision: %s", resp.Decision)
	}
}

// ResubmitForReview marks a review as resubmitted after changes.
func (s *Service) ResubmitForReview(ctx context.Context, reviewID,
	newCommitSHA string) error {

	// Update commit SHA
	if err := s.store.UpdateReviewCommit(ctx, reviewID, newCommitSHA); err != nil {
		return fmt.Errorf("failed to update commit: %w", err)
	}

	return s.transitionState(ctx, reviewID, ResubmitEvent{
		NewCommitSHA: newCommitSHA,
	})
}

// CancelReview cancels an active review.
func (s *Service) CancelReview(ctx context.Context, reviewID,
	reason string) error {

	return s.transitionState(ctx, reviewID, CancelEvent{
		Reason: reason,
	})
}

// GetReviewState returns the current state of a review.
func (s *Service) GetReviewState(ctx context.Context,
	reviewID string) (ReviewState, error) {

	review, err := s.store.GetReview(ctx, reviewID)
	if err != nil {
		return "", err
	}
	return ReviewState(review.State), nil
}

// GetReviewIterations returns all iterations for a review.
func (s *Service) GetReviewIterations(ctx context.Context,
	reviewID string) ([]store.ReviewIteration, error) {

	return s.store.ListReviewIterations(ctx, reviewID)
}

// GetOpenIssues returns open issues for a review.
func (s *Service) GetOpenIssues(ctx context.Context,
	reviewID string) ([]store.ReviewIssue, error) {

	return s.store.ListOpenReviewIssues(ctx, reviewID)
}

// ResolveIssue marks an issue as resolved.
func (s *Service) ResolveIssue(ctx context.Context, issueID int64) error {
	// Get current iteration
	// For now, use 0 as we'd need the review ID to get the actual iteration
	return s.store.ResolveIssue(ctx, issueID, 0)
}

// GetReviewStats returns aggregate review statistics.
func (s *Service) GetReviewStats(ctx context.Context) (store.ReviewStats, error) {
	return s.store.GetReviewStats(ctx)
}

// transitionState applies a state transition to a review.
func (s *Service) transitionState(ctx context.Context, reviewID string,
	event ReviewEvent) error {

	// Get or create FSM
	s.mu.Lock()
	fsm, exists := s.activeReviews[reviewID]
	if !exists {
		// Load from database
		review, err := s.store.GetReview(ctx, reviewID)
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("review not found: %w", err)
		}
		fsm = NewReviewFSM(reviewID, review.ThreadID, s.defaultConfig)
		fsm.CurrentState = ReviewState(review.State)
		s.activeReviews[reviewID] = fsm
	}
	s.mu.Unlock()

	// Process event
	newState, err := fsm.ProcessEvent(event)
	if err != nil {
		return err
	}

	// Persist state change
	if newState == StateApproved || newState == StateRejected ||
		newState == StateCancelled {
		return s.store.CompleteReview(ctx, reviewID, string(newState))
	}
	return s.store.UpdateReviewState(ctx, reviewID, string(newState))
}

// GetFSM returns the FSM for a review (for testing/debugging).
func (s *Service) GetFSM(reviewID string) *ReviewFSM {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeReviews[reviewID]
}

// RegisterReviewer registers a reviewer configuration.
func (s *Service) RegisterReviewer(config *ReviewerConfig) {
	s.reviewers[config.Name] = config
}

// GetReviewer returns a reviewer configuration by name.
func (s *Service) GetReviewer(name string) (*ReviewerConfig, bool) {
	config, ok := s.reviewers[name]
	return config, ok
}

// ListReviewers returns all registered reviewer names.
func (s *Service) ListReviewers() []string {
	names := make([]string, 0, len(s.reviewers))
	for name := range s.reviewers {
		names = append(names, name)
	}
	return names
}

// CleanupOldFSMs removes FSMs for completed reviews from memory.
func (s *Service) CleanupOldFSMs(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, fsm := range s.activeReviews {
		// Check if review is in a terminal state and old
		if fsm.CurrentState == StateApproved ||
			fsm.CurrentState == StateRejected ||
			fsm.CurrentState == StateCancelled {
			// Check last transition time
			if len(fsm.Transitions) > 0 {
				lastTransition := fsm.Transitions[len(fsm.Transitions)-1]
				if lastTransition.Timestamp.Before(cutoff) {
					delete(s.activeReviews, id)
				}
			}
		}
	}
}

// =============================================================================
// Spawner Integration for Structured Analysis
// =============================================================================

// SpawnStructuredReview runs a one-shot Claude Code analysis with JSON output.
// This is used for automated review passes that need structured data.
func (s *Service) SpawnStructuredReview(ctx context.Context,
	req StructuredReviewRequest) (*StructuredReviewResult, error) {

	if s.spawner == nil {
		return nil, fmt.Errorf("spawner not configured")
	}

	// Build the prompt from the template
	prompt, err := s.buildStructuredPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	s.log.Info("Spawning structured review",
		"review_id", req.ReviewID,
		"work_dir", req.WorkDir,
		"files", len(req.ChangedFiles),
	)

	startTime := time.Now()

	// Spawn Claude with the prompt
	resp, err := s.spawner.Spawn(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("spawn failed: %w", err)
	}

	if resp.IsError {
		return nil, fmt.Errorf("reviewer returned error: %s", resp.Error)
	}

	// Parse the structured JSON response
	result, err := s.parseStructuredResponse(resp.Result)
	if err != nil {
		// Log the raw response for debugging
		s.log.Warn("Failed to parse structured response",
			"error", err,
			"review_id", req.ReviewID,
			"raw_response_len", len(resp.Result),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Add metadata from the spawn response
	result.SessionID = resp.SessionID
	result.CostUSD = resp.CostUSD
	result.DurationMS = resp.DurationMS

	s.log.Info("Structured review completed",
		"review_id", req.ReviewID,
		"decision", result.Decision,
		"issues", len(result.Issues),
		"duration_ms", time.Since(startTime).Milliseconds(),
		"cost_usd", resp.CostUSD,
	)

	return result, nil
}

// buildStructuredPrompt builds the prompt for a structured review.
func (s *Service) buildStructuredPrompt(req StructuredReviewRequest) (string, error) {
	var buf bytes.Buffer
	if err := s.promptTemplate.Execute(&buf, req); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// parseStructuredResponse extracts the JSON from the response and parses it.
func (s *Service) parseStructuredResponse(response string) (*StructuredReviewResult, error) {
	// Try to find JSON in the response
	// The response might contain markdown code blocks around the JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var result StructuredReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &result, nil
}

// extractJSON attempts to extract JSON from a response that may contain
// markdown code blocks or other text.
func extractJSON(s string) string {
	// First, try to find JSON in a code block
	start := -1
	end := -1

	// Look for ```json ... ```
	if idx := findSubstring(s, "```json"); idx != -1 {
		start = idx + 7 // len("```json")
		// Skip any whitespace after ```json
		for start < len(s) && (s[start] == '\n' || s[start] == '\r') {
			start++
		}
		// Find closing ```
		if closeIdx := findSubstring(s[start:], "```"); closeIdx != -1 {
			end = start + closeIdx
		}
	}

	// Also try just ``` ... ```
	if start == -1 {
		if idx := findSubstring(s, "```"); idx != -1 {
			start = idx + 3
			// Skip whitespace
			for start < len(s) && (s[start] == '\n' || s[start] == '\r') {
				start++
			}
			// Find closing ```
			if closeIdx := findSubstring(s[start:], "```"); closeIdx != -1 {
				end = start + closeIdx
			}
		}
	}

	if start != -1 && end != -1 {
		return s[start:end]
	}

	// Try to find a JSON object directly
	// Look for first { and last }
	braceStart := findSubstring(s, "{")
	braceEnd := lastIndex(s, "}")
	if braceStart != -1 && braceEnd != -1 && braceEnd > braceStart {
		return s[braceStart : braceEnd+1]
	}

	return ""
}

// findSubstring finds the first occurrence of substr in s.
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// lastIndex finds the last occurrence of char in s.
func lastIndex(s string, char string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if string(s[i]) == char {
			return i
		}
	}
	return -1
}

// ProcessStructuredResult processes the result from a structured review
// and updates the database and FSM accordingly.
func (s *Service) ProcessStructuredResult(ctx context.Context,
	reviewID string, result *StructuredReviewResult) error {

	// Create a ReviewResponse from the structured result
	issues := make([]ReviewIssue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, ReviewIssue{
			Type:        IssueType(issue.Type),
			Severity:    Severity(issue.Severity),
			File:        issue.FilePath,
			LineStart:   issue.LineStart,
			LineEnd:     issue.LineEnd,
			Title:       issue.Title,
			Description: issue.Description,
			CodeSnippet: issue.CodeSnippet,
			Suggestion:  issue.Suggestion,
			ClaudeMDRef: issue.ClaudeMDRef,
		})
	}

	suggestions := make([]Suggestion, 0, len(result.Suggestions))
	for _, sugg := range result.Suggestions {
		suggestions = append(suggestions, Suggestion{
			Title:       sugg.Title,
			Description: sugg.Description,
		})
	}

	resp := ReviewResponse{
		ReviewID:      reviewID,
		ReviewerName:  "StructuredReviewer",
		Decision:      result.Decision,
		Summary:       result.Summary,
		Issues:        issues,
		Suggestions:   suggestions,
		FilesReviewed: result.FilesReviewed,
		LinesAnalyzed: result.LinesAnalyzed,
		DurationMS:    result.DurationMS,
		CostUSD:       result.CostUSD,
	}

	return s.SubmitReviewIteration(ctx, resp)
}

// RunAutomatedReview runs a complete automated review cycle:
// 1. Builds the structured review request
// 2. Spawns Claude for analysis
// 3. Processes the result
func (s *Service) RunAutomatedReview(ctx context.Context,
	reviewID string, opts AutomatedReviewOptions) (*StructuredReviewResult, error) {

	// Get the review from the database
	review, err := s.store.GetReview(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("failed to get review: %w", err)
	}

	// Get any previous issues for context
	previousIssues, _ := s.store.ListOpenReviewIssues(ctx, reviewID)

	// Build the request
	req := StructuredReviewRequest{
		ReviewID:     reviewID,
		WorkDir:      review.RepoPath,
		Diff:         opts.Diff,
		Context:      opts.Context,
		ChangedFiles: opts.ChangedFiles,
		FocusAreas:   opts.FocusAreas,
	}

	// Convert previous issues
	for _, issue := range previousIssues {
		var lineEnd int
		if issue.LineEnd != nil {
			lineEnd = *issue.LineEnd
		}
		req.PreviousIssues = append(req.PreviousIssues, ReviewIssue{
			Type:        IssueType(issue.IssueType),
			Severity:    Severity(issue.Severity),
			File:        issue.FilePath,
			LineStart:   issue.LineStart,
			LineEnd:     lineEnd,
			Title:       issue.Title,
			Description: issue.Description,
		})
	}

	// Mark review as under_review
	if err := s.StartReview(ctx, reviewID, "AutomatedReviewer"); err != nil {
		s.log.Warn("Failed to transition to under_review", "error", err)
	}

	// Spawn the structured review
	result, err := s.SpawnStructuredReview(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("structured review failed: %w", err)
	}

	// Process the result
	if err := s.ProcessStructuredResult(ctx, reviewID, result); err != nil {
		return nil, fmt.Errorf("failed to process result: %w", err)
	}

	return result, nil
}

// AutomatedReviewOptions contains options for running an automated review.
type AutomatedReviewOptions struct {
	// Diff is the git diff to review.
	Diff string

	// Context provides additional context (PR description, etc.)
	Context string

	// ChangedFiles lists the files that were changed.
	ChangedFiles []string

	// FocusAreas specifies areas to focus on (security, performance, etc.)
	FocusAreas []string

	// ReviewerType specifies which reviewer prompt to use.
	ReviewerType string
}

// CreateSpawnerForReviewer creates a spawner configured for a specific reviewer.
func (s *Service) CreateSpawnerForReviewer(reviewerType string,
	workDir string) *agent.Spawner {

	prompt := GetReviewerPrompt(reviewerType)

	cfg := agent.DefaultSpawnConfig()
	cfg.SystemPrompt = prompt
	cfg.WorkDir = workDir

	// Use the appropriate model for the reviewer type
	config, ok := s.reviewers[reviewerType]
	if ok && config.Model != "" {
		cfg.Model = config.Model
	}

	return agent.NewSpawner(cfg)
}
