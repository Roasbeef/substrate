package review

import (
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

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
// This is now the coordinator-based multi-sub-reviewer mode by default.
func DefaultReviewerConfig() *ReviewerConfig {
	return &ReviewerConfig{
		Name: "CoordinatorReviewer",
		FocusAreas: []string{
			"bugs",
			"logic_errors",
			"security_vulnerabilities",
			"claude_md_compliance",
		},
		Model:   "claude-sonnet-4-20250514",
		Timeout: 15 * time.Minute,
	}
}

// SingleReviewerConfig returns the legacy single-agent code reviewer
// configuration. Used as a fallback when multi-review is disabled.
func SingleReviewerConfig() *ReviewerConfig {
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

// FullReviewSubAgents returns the specialized sub-agent definitions used
// by the coordinator for multi-sub-reviewer mode. Each agent focuses on
// a narrow domain to maximize issue detection across orthogonal review
// dimensions. The coordinator spawns all of these via the SDK Task tool.
func FullReviewSubAgents() map[string]claudeagent.AgentDefinition {
	return map[string]claudeagent.AgentDefinition{
		"code-quality-reviewer": {
			Name:        "code-quality-reviewer",
			Description: "Review code for logic errors, error handling gaps, edge cases, and correctness issues",
			Prompt:      codeQualityPrompt,
			Tools: []string{
				"Read", "Grep", "Glob", "LS", "Bash",
			},
		},
		"security-reviewer": {
			Name:        "security-reviewer",
			Description: "Review code for security vulnerabilities including race conditions, injection, and auth issues",
			Prompt:      securitySubReviewerPrompt,
			Tools: []string{
				"Read", "Grep", "Glob", "LS", "Bash",
			},
		},
		"performance-reviewer": {
			Name:        "performance-reviewer",
			Description: "Review code for performance issues including resource leaks, unbounded growth, and inefficiency",
			Prompt:      performanceSubReviewerPrompt,
			Tools: []string{
				"Read", "Grep", "Glob", "LS", "Bash",
			},
		},
		"test-coverage-reviewer": {
			Name:        "test-coverage-reviewer",
			Description: "Review test coverage for missing test cases, untested edge cases, and test quality",
			Prompt:      testCoveragePrompt,
			Tools: []string{
				"Read", "Grep", "Glob", "LS", "Bash",
			},
		},
		"doc-compliance-reviewer": {
			Name:        "doc-compliance-reviewer",
			Description: "Review documentation accuracy, CLAUDE.md compliance, and comment correctness",
			Prompt:      docCompliancePrompt,
			Tools: []string{
				"Read", "Grep", "Glob", "LS", "Bash",
			},
		},
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
