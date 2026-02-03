package review

import "time"

// ReviewerConfig defines a specialized reviewer persona. This is used to
// configure reviewer agents when they start.
type ReviewerConfig struct {
	// Name is the reviewer persona identifier (e.g., "SecurityReviewer").
	Name string

	// SystemPrompt is the base system prompt (CLAUDE.md content).
	SystemPrompt string

	// FocusAreas defines what the reviewer should look for.
	FocusAreas []string

	// IgnorePatterns defines files/patterns to skip.
	IgnorePatterns []string

	// Model is the Claude model to use (e.g., "claude-opus-4-5-20251101").
	Model string

	// Timeout is the maximum time for a single review round.
	Timeout time.Duration
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
		Model:   "claude-sonnet-4-20250514",
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
			Timeout: 15 * time.Minute,
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
			Timeout: 10 * time.Minute,
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
			Timeout: 15 * time.Minute,
		},
	}
}
