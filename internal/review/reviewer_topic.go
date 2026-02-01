package review

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// ReviewerTopicName is the default topic name for review requests.
const ReviewerTopicName = "reviewers"

// ReviewerRole defines the specialization of a reviewer.
type ReviewerRole string

const (
	RoleGeneral      ReviewerRole = "general"
	RoleSecurity     ReviewerRole = "security"
	RolePerformance  ReviewerRole = "performance"
	RoleArchitecture ReviewerRole = "architecture"
)

// RegisteredReviewer represents a reviewer agent subscribed to the reviewers topic.
type RegisteredReviewer struct {
	AgentID     int64
	AgentName   string
	Role        ReviewerRole
	Active      bool
	LastSeen    time.Time
	ReviewCount int
}

// ReviewerTopicManager manages the reviewers topic and subscriber registration.
type ReviewerTopicManager struct {
	store       store.Storage
	mailService *mail.Service
	log         *slog.Logger

	mu        sync.RWMutex
	reviewers map[int64]*RegisteredReviewer
	topicID   int64
}

// NewReviewerTopicManager creates a new reviewer topic manager.
func NewReviewerTopicManager(
	s store.Storage,
	mailSvc *mail.Service,
	log *slog.Logger,
) *ReviewerTopicManager {

	if log == nil {
		log = slog.Default()
	}

	return &ReviewerTopicManager{
		store:       s,
		mailService: mailSvc,
		log:         log,
		reviewers:   make(map[int64]*RegisteredReviewer),
	}
}

// Initialize ensures the reviewers topic exists and loads registered reviewers.
func (m *ReviewerTopicManager) Initialize(ctx context.Context) error {
	// Get or create the reviewers topic.
	topic, err := m.store.GetOrCreateTopic(ctx, ReviewerTopicName, "broadcast")
	if err != nil {
		return fmt.Errorf("failed to create reviewers topic: %w", err)
	}
	m.topicID = topic.ID

	// Load existing subscribers.
	subscribers, err := m.store.GetTopicSubscribers(ctx, m.topicID)
	if err != nil {
		m.log.Warn("Failed to load topic subscribers", "error", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range subscribers {
		m.reviewers[sub.AgentID] = &RegisteredReviewer{
			AgentID:   sub.AgentID,
			AgentName: sub.AgentName,
			Role:      RoleGeneral, // Default role
			Active:    true,
			LastSeen:  sub.SubscribedAt,
		}
	}

	m.log.Info("Reviewer topic initialized",
		"topic_id", m.topicID,
		"subscriber_count", len(m.reviewers),
	)

	return nil
}

// RegisterReviewer registers an agent as a reviewer with a specific role.
func (m *ReviewerTopicManager) RegisterReviewer(
	ctx context.Context,
	agentID int64,
	agentName string,
	role ReviewerRole,
) error {

	// Subscribe to the topic.
	err := m.store.SubscribeToTopic(ctx, m.topicID, agentID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	m.mu.Lock()
	m.reviewers[agentID] = &RegisteredReviewer{
		AgentID:   agentID,
		AgentName: agentName,
		Role:      role,
		Active:    true,
		LastSeen:  time.Now(),
	}
	m.mu.Unlock()

	m.log.Info("Reviewer registered",
		"agent_id", agentID,
		"agent_name", agentName,
		"role", role,
	)

	return nil
}

// UnregisterReviewer removes an agent from the reviewers list.
func (m *ReviewerTopicManager) UnregisterReviewer(ctx context.Context, agentID int64) error {
	err := m.store.UnsubscribeFromTopic(ctx, m.topicID, agentID)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	m.mu.Lock()
	delete(m.reviewers, agentID)
	m.mu.Unlock()

	m.log.Info("Reviewer unregistered", "agent_id", agentID)

	return nil
}

// ListReviewers returns all registered reviewers.
func (m *ReviewerTopicManager) ListReviewers() []*RegisteredReviewer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reviewers := make([]*RegisteredReviewer, 0, len(m.reviewers))
	for _, r := range m.reviewers {
		reviewers = append(reviewers, r)
	}
	return reviewers
}

// ListReviewersByRole returns reviewers with a specific role.
func (m *ReviewerTopicManager) ListReviewersByRole(role ReviewerRole) []*RegisteredReviewer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reviewers []*RegisteredReviewer
	for _, r := range m.reviewers {
		if r.Role == role || role == RoleGeneral {
			reviewers = append(reviewers, r)
		}
	}
	return reviewers
}

// GetActiveReviewers returns only active reviewers.
func (m *ReviewerTopicManager) GetActiveReviewers() []*RegisteredReviewer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reviewers []*RegisteredReviewer
	for _, r := range m.reviewers {
		if r.Active {
			reviewers = append(reviewers, r)
		}
	}
	return reviewers
}

// UpdateReviewerStatus updates the active status and last seen time for a reviewer.
func (m *ReviewerTopicManager) UpdateReviewerStatus(agentID int64, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.reviewers[agentID]; ok {
		r.Active = active
		r.LastSeen = time.Now()
	}
}

// IncrementReviewCount increments the review count for a reviewer.
func (m *ReviewerTopicManager) IncrementReviewCount(agentID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.reviewers[agentID]; ok {
		r.ReviewCount++
	}
}

// ReviewRequestMessage is the message format for review requests.
type ReviewRequestMessage struct {
	Type        string `json:"type"`
	ReviewID    string `json:"review_id"`
	ThreadID    string `json:"thread_id"`
	RequesterID int64  `json:"requester_id"`
	Branch      string `json:"branch"`
	BaseBranch  string `json:"base_branch"`
	CommitSHA   string `json:"commit_sha"`
	RepoPath    string `json:"repo_path"`
	ReviewType  string `json:"review_type"`
	Priority    string `json:"priority"`
	Iteration   int    `json:"iteration,omitempty"`
}

// PublishReviewRequest publishes a review request to the reviewers topic.
// Returns the message ID and thread ID.
func (m *ReviewerTopicManager) PublishReviewRequest(
	ctx context.Context,
	senderID int64,
	req ReviewRequestMessage,
) (int64, string, error) {

	// Serialize the review request.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return 0, "", fmt.Errorf("failed to marshal review request: %w", err)
	}

	// Build the message subject.
	subject := fmt.Sprintf("[Review Request] %s", req.Branch)
	if req.Iteration > 1 {
		subject = fmt.Sprintf("[Re-Review] %s (iteration %d)", req.Branch, req.Iteration)
	}

	// Build the message body.
	body := fmt.Sprintf(`# Review Request

**Branch:** %s
**Base:** %s
**Commit:** %s
**Type:** %s
**Priority:** %s

---
%s`, req.Branch, req.BaseBranch, req.CommitSHA[:8], req.ReviewType, req.Priority, string(reqJSON))

	// Publish to the topic.
	if m.mailService != nil {
		resp, err := m.mailService.Publish(ctx, mail.PublishRequest{
			SenderID:  senderID,
			TopicName: ReviewerTopicName,
			Subject:   subject,
			Body:      body,
			Priority:  mail.Priority(req.Priority),
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to publish to topic: %w", err)
		}
		return resp.MessageID, resp.ThreadID, nil
	}

	// Fallback: direct store access.
	topic, err := m.store.GetTopic(ctx, m.topicID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get topic: %w", err)
	}

	msg, err := m.store.CreateMessage(ctx, store.CreateMessageParams{
		SenderID: senderID,
		TopicID:  &topic.ID,
		Subject:  subject,
		Body:     body,
		Priority: req.Priority,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to create message: %w", err)
	}

	return msg.ID, msg.ThreadID, nil
}

// FanOutReviewRequest sends a review request to specific reviewers.
// Used for targeted multi-reviewer scenarios.
func (m *ReviewerTopicManager) FanOutReviewRequest(
	ctx context.Context,
	senderID int64,
	req ReviewRequestMessage,
	targetRoles []ReviewerRole,
) ([]int64, error) {

	// Get reviewers by role.
	var targetReviewers []*RegisteredReviewer
	for _, role := range targetRoles {
		reviewers := m.ListReviewersByRole(role)
		targetReviewers = append(targetReviewers, reviewers...)
	}

	if len(targetReviewers) == 0 {
		return nil, fmt.Errorf("no reviewers available for roles: %v", targetRoles)
	}

	// Deduplicate reviewers.
	seen := make(map[int64]bool)
	var uniqueReviewers []*RegisteredReviewer
	for _, r := range targetReviewers {
		if !seen[r.AgentID] {
			seen[r.AgentID] = true
			uniqueReviewers = append(uniqueReviewers, r)
		}
	}

	// Serialize the review request.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal review request: %w", err)
	}

	// Build the message.
	subject := fmt.Sprintf("[Review Request] %s", req.Branch)
	body := fmt.Sprintf(`# Review Request

**Branch:** %s
**Base:** %s
**Commit:** %s
**Type:** %s
**Priority:** %s

---
%s`, req.Branch, req.BaseBranch, req.CommitSHA[:8], req.ReviewType, req.Priority, string(reqJSON))

	// Send to each reviewer.
	var messageIDs []int64
	for _, reviewer := range uniqueReviewers {
		if m.mailService != nil {
			resp, err := m.mailService.SendMail(ctx, mail.SendMailRequest{
				SenderID:       senderID,
				RecipientNames: []string{reviewer.AgentName},
				Subject:        subject,
				Body:           body,
				Priority:       mail.Priority(req.Priority),
				ThreadID:       req.ThreadID,
			})
			if err != nil {
				m.log.Warn("Failed to send to reviewer",
					"reviewer", reviewer.AgentName,
					"error", err,
				)
				continue
			}
			messageIDs = append(messageIDs, resp.MessageID)
		}
	}

	m.log.Info("Fan-out review request sent",
		"review_id", req.ReviewID,
		"reviewer_count", len(messageIDs),
	)

	return messageIDs, nil
}

// SelectReviewersForRequest selects appropriate reviewers for a review request.
// Uses round-robin or least-loaded selection for load balancing.
func (m *ReviewerTopicManager) SelectReviewersForRequest(
	reviewType ReviewType,
	count int,
) []*RegisteredReviewer {

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Determine required role based on review type.
	var requiredRole ReviewerRole
	switch reviewType {
	case TypeSecurity:
		requiredRole = RoleSecurity
	case TypePerformance:
		requiredRole = RolePerformance
	default:
		requiredRole = RoleGeneral
	}

	// Collect eligible reviewers.
	var eligible []*RegisteredReviewer
	for _, r := range m.reviewers {
		if !r.Active {
			continue
		}
		if requiredRole == RoleGeneral || r.Role == requiredRole {
			eligible = append(eligible, r)
		}
	}

	if len(eligible) == 0 {
		return nil
	}

	// Sort by review count (least loaded first).
	for i := 0; i < len(eligible)-1; i++ {
		for j := i + 1; j < len(eligible); j++ {
			if eligible[j].ReviewCount < eligible[i].ReviewCount {
				eligible[i], eligible[j] = eligible[j], eligible[i]
			}
		}
	}

	// Select up to count reviewers.
	if count > len(eligible) {
		count = len(eligible)
	}

	return eligible[:count]
}
