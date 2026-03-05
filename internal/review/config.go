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

	// Model is the Claude model to use (e.g., "claude-opus-4-6").
	Model string

	// Timeout is the maximum time for a single review round.
	Timeout time.Duration
}

// reviewModel is the model used for all reviewer sub-agents. Using a
// consistent high-capability model across all agents ensures uniform
// quality for security-critical analysis.
const reviewModel = "claude-opus-4-6"

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
		Model:   reviewModel,
		Timeout: 20 * time.Minute,
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
		Model:   reviewModel,
		Timeout: 10 * time.Minute,
	}
}

// subAgentTools is the standard tool set available to all review
// sub-agents. Read, Grep, Glob, and LS provide code navigation while
// Bash enables git blame, ast-grep, and other analysis commands.
var subAgentTools = []string{
	"Read", "Grep", "Glob", "LS", "Bash",
}

// Tier1SubAgents returns the Tier 1 sub-agent definitions that always
// run during deep and full security depth reviews. These cover the
// core review dimensions: code quality, offensive security, differential
// analysis, performance, test coverage, and documentation compliance.
func Tier1SubAgents() map[string]claudeagent.AgentDefinition {
	return map[string]claudeagent.AgentDefinition{
		"code-reviewer": {
			Name:        "code-reviewer",
			Description: "Senior staff engineer review: 8-phase methodology with Bitcoin/Lightning expertise, Go patterns, and production readiness",
			Prompt:      codeReviewerPrompt,
			Tools:       subAgentTools,
		},
		"security-auditor": {
			Name:        "security-auditor",
			Description: "Offensive security audit: exploit development, Bitcoin attack patterns, CVSS classification, and proof-of-concept exploits",
			Prompt:      securityAuditorPrompt,
			Tools:       subAgentTools,
		},
		"differential-reviewer": {
			Name:        "differential-reviewer",
			Description: "Trail of Bits differential review: blast radius calculation, git blame regression detection, and adversarial analysis",
			Prompt:      differentialReviewPrompt,
			Tools:       subAgentTools,
		},
		"performance-reviewer": {
			Name:        "performance-reviewer",
			Description: "Performance analysis: resource leaks, algorithmic efficiency, Go-specific perf issues, and allocation optimization",
			Prompt:      performanceSubReviewerPrompt,
			Tools:       subAgentTools,
		},
		"test-coverage-reviewer": {
			Name:        "test-coverage-reviewer",
			Description: "Test quality: missing tests, untested edge cases, fuzz candidates, table-driven patterns, and race detector coverage",
			Prompt:      testCoveragePrompt,
			Tools:       subAgentTools,
		},
		"doc-compliance-reviewer": {
			Name:        "doc-compliance-reviewer",
			Description: "Documentation accuracy and CLAUDE.md compliance: comment correctness, API docs, and explicit rule violations",
			Prompt:      docCompliancePrompt,
			Tools:       subAgentTools,
		},
	}
}

// Tier2SubAgents returns the Tier 2 sub-agent definitions that run
// conditionally based on file classification or when security depth is
// set to "full". These provide deeper specialized analysis for
// high-risk code areas.
func Tier2SubAgents() map[string]claudeagent.AgentDefinition {
	return map[string]claudeagent.AgentDefinition{
		"function-analyzer": {
			Name:        "function-analyzer",
			Description: "Trail of Bits deep function analysis: ultra-granular line-by-line analysis with First Principles, 5 Whys, invariant mapping",
			Prompt:      functionAnalyzerPrompt,
			Tools:       subAgentTools,
		},
		"spec-compliance-checker": {
			Name:        "spec-compliance-checker",
			Description: "BIP/BOLT specification compliance: spec-to-code mapping, divergence classification, anti-hallucination verification",
			Prompt:      specCompliancePrompt,
			Tools:       subAgentTools,
		},
		"api-safety-reviewer": {
			Name:        "api-safety-reviewer",
			Description: "API safety and insecure defaults: sharp edges analysis, footgun detection, fail-open patterns, three adversary threat model",
			Prompt:      apiSafetyPrompt,
			Tools:       subAgentTools,
		},
		"variant-analyzer": {
			Name:        "variant-analyzer",
			Description: "Variant analysis: find similar bugs across the codebase using pattern-based search with ast-grep and ripgrep",
			Prompt:      variantAnalyzerPrompt,
			Tools:       subAgentTools,
		},
	}
}

// FullReviewSubAgents returns all sub-agent definitions (Tier 1 + Tier 2)
// used by the coordinator for multi-sub-reviewer mode. The coordinator
// spawns Tier 1 agents unconditionally and Tier 2 agents conditionally
// based on file classification and security depth.
func FullReviewSubAgents() map[string]claudeagent.AgentDefinition {
	agents := Tier1SubAgents()
	for name, def := range Tier2SubAgents() {
		agents[name] = def
	}

	return agents
}

// SpecializedReviewers returns additional persona configurations for
// type-specific reviews (security, performance, architecture). These
// are used when the user requests a targeted review via --type flag.
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
			Model:   reviewModel,
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
			Model:   reviewModel,
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
			Model:   reviewModel,
			Timeout: 15 * time.Minute,
		},
	}
}
