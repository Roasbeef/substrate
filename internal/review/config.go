package review

import "time"

// ReviewerConfig defines a specialized reviewer persona.
type ReviewerConfig struct {
	// Name is the unique identifier for this reviewer persona.
	Name string

	// SystemPrompt is the base system prompt content.
	SystemPrompt string

	// FocusAreas defines what this reviewer should focus on.
	FocusAreas []string

	// IgnorePatterns are file patterns this reviewer should skip.
	IgnorePatterns []string

	// Model is the Claude model to use (e.g., claude-opus-4-5-20251101).
	Model string

	// WorkDir is the default working directory for code checkout.
	WorkDir string

	// Timeout is the maximum time for a review to complete.
	Timeout time.Duration

	// Hooks are custom hooks for this reviewer type.
	Hooks ReviewerHooks
}

// ReviewerHooks defines hook points for reviewer customization.
type ReviewerHooks struct {
	// OnStart is called when a review starts.
	OnStart string

	// OnComplete is called when a review completes.
	OnComplete string

	// OnIssueFound is called when an issue is found.
	OnIssueFound string
}

// MultiReviewConfig configures a multi-reviewer setup.
type MultiReviewConfig struct {
	// TopicName is where review requests are published.
	TopicName string

	// Reviewers are the personas subscribed to this topic.
	Reviewers []string // ["security", "performance", "architecture"]

	// Consensus rules
	RequireAll      bool // All must approve vs majority
	MinApprovals    int  // Minimum approvals needed
	BlockOnCritical bool // Any critical issue blocks approval
}

// DefaultReviewerConfig returns the standard code reviewer configuration.
func DefaultReviewerConfig() *ReviewerConfig {
	return &ReviewerConfig{
		Name: "CodeReviewer",
		FocusAreas: []string{
			"bugs",
			"logic_errors",
			"security_vulnerabilities",
			"claude_md_compliance",
		},
		Model:   "claude-opus-4-5-20251101",
		Timeout: 10 * time.Minute,
	}
}

// SpecializedReviewers returns additional persona configurations.
func SpecializedReviewers() map[string]*ReviewerConfig {
	return map[string]*ReviewerConfig{
		"security": {
			Name: "SecurityReviewer",
			FocusAreas: []string{
				"injection_vulnerabilities",
				"authentication_bypass",
				"authorization_flaws",
				"sensitive_data_exposure",
				"cryptographic_issues",
			},
			Model:   "claude-opus-4-5-20251101",
			Timeout: 10 * time.Minute,
		},
		"performance": {
			Name: "PerformanceReviewer",
			FocusAreas: []string{
				"n_plus_one_queries",
				"memory_leaks",
				"inefficient_algorithms",
				"unnecessary_allocations",
				"blocking_operations",
			},
			Model:   "claude-sonnet-4-20250514",
			Timeout: 5 * time.Minute,
		},
		"architecture": {
			Name: "ArchitectureReviewer",
			FocusAreas: []string{
				"separation_of_concerns",
				"interface_design",
				"dependency_management",
				"testability",
			},
			Model:   "claude-opus-4-5-20251101",
			Timeout: 10 * time.Minute,
		},
	}
}

// DefaultMultiReviewConfig returns a default multi-reviewer configuration.
func DefaultMultiReviewConfig() *MultiReviewConfig {
	return &MultiReviewConfig{
		TopicName:       "reviews",
		Reviewers:       []string{"security", "performance"},
		RequireAll:      false,
		MinApprovals:    1,
		BlockOnCritical: true,
	}
}
