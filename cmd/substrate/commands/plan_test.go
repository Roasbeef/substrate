package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadPlanContext verifies JSON lines parsing from a plan context file.
func TestLoadPlanContext(t *testing.T) {
	dir := t.TempDir()

	// Create .claude directory.
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))

	// Write context file with multiple entries.
	ctxPath := filepath.Join(claudeDir, ".substrate-plan-context")
	entries := []planContext{
		{
			SessionID: "sess-1",
			PlanPath:  "/tmp/plan1.md",
			Timestamp: 1000,
		},
		{
			SessionID: "sess-2",
			PlanPath:  "/tmp/plan2.md",
			Timestamp: 2000,
		},
		{
			SessionID: "sess-1",
			PlanPath:  "/tmp/plan3.md",
			Timestamp: 3000,
		},
	}

	var lines []string
	for _, e := range entries {
		data, err := json.Marshal(e)
		require.NoError(t, err)
		lines = append(lines, string(data))
	}
	err := os.WriteFile(
		ctxPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644,
	)
	require.NoError(t, err)

	// Load and verify.
	loaded, err := loadPlanContext(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 3)
	require.Equal(t, "sess-1", loaded[0].SessionID)
	require.Equal(t, "/tmp/plan2.md", loaded[1].PlanPath)
	require.Equal(t, int64(3000), loaded[2].Timestamp)
}

// TestLoadPlanContextMissing verifies empty result for missing file.
func TestLoadPlanContextMissing(t *testing.T) {
	dir := t.TempDir()

	loaded, err := loadPlanContext(dir)
	require.NoError(t, err)
	require.Nil(t, loaded)
}

// TestLoadPlanContextSkipsInvalidLines verifies bad JSON lines are skipped.
func TestLoadPlanContextSkipsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))

	ctxPath := filepath.Join(claudeDir, ".substrate-plan-context")
	content := `{"session_id":"s1","plan_path":"/tmp/a.md","timestamp":1}
not-json-at-all
{"session_id":"s2","plan_path":"/tmp/b.md","timestamp":2}
`
	err := os.WriteFile(ctxPath, []byte(content), 0o644)
	require.NoError(t, err)

	loaded, err := loadPlanContext(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	require.Equal(t, "s1", loaded[0].SessionID)
	require.Equal(t, "s2", loaded[1].SessionID)
}

// TestExtractPlanTitle verifies title extraction from plan content.
func TestExtractPlanTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		path     string
		expected string
	}{
		{
			name:     "h1 heading",
			content:  "# My Implementation Plan\n\nSome content.",
			path:     "/tmp/plan.md",
			expected: "My Implementation Plan",
		},
		{
			name:     "h1 after frontmatter",
			content:  "---\nid: abc\n---\n\n# Plan Title\n\nBody.",
			path:     "/tmp/plan.md",
			expected: "Plan Title",
		},
		{
			name:     "no h1 uses filename",
			content:  "## Only h2 heading\n\nContent.",
			path:     "/tmp/my-feature-plan.md",
			expected: "my-feature-plan",
		},
		{
			name:     "no h1 no path",
			content:  "Just some text.",
			path:     "",
			expected: "Implementation Plan",
		},
		{
			name:     "empty content with path",
			content:  "",
			path:     "/home/user/refactor.md",
			expected: "refactor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPlanTitle(tc.content, tc.path)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractRegexSummary verifies regex-based summary extraction.
func TestExtractRegexSummary(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "summary section",
			content: `# Plan
## Summary
This plan adds authentication.
It uses JWT tokens.

## Details
Implementation details here.`,
			expected: "This plan adds authentication.\nIt uses JWT tokens.",
		},
		{
			name: "overview section",
			content: `# Plan
## Overview
High-level overview of changes.

## Implementation
Step by step.`,
			expected: "High-level overview of changes.",
		},
		{
			name: "context section",
			content: `# Plan
## Context
The system needs caching.

## Approach
Use Redis.`,
			expected: "The system needs caching.",
		},
		{
			name: "tldr section",
			content: `# Plan
## TL;DR
Quick summary here.

## Phase 1
First phase.`,
			expected: "Quick summary here.",
		},
		{
			name: "no summary section fallback",
			content: `# My Plan
This is the first paragraph.

## Phase 1
Details.`,
			expected: "This is the first paragraph.",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractRegexSummary(tc.content)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractFilesSection verifies files section extraction.
func TestExtractFilesSection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "files to modify section",
			content: `# Plan
## Files to Modify
- src/auth.go
- src/handler.go

## Details
More info.`,
			expected: "- src/auth.go\n- src/handler.go",
		},
		{
			name: "files section",
			content: `# Plan
## Files
- config.yaml
- main.go

## Testing
Tests here.`,
			expected: "- config.yaml\n- main.go",
		},
		{
			name:     "no files section",
			content:  "# Plan\n## Overview\nJust text.",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractFilesSection(tc.content)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestCleanSubject verifies subject line sanitization.
func TestCleanSubject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal subject",
			input:    "Add authentication",
			expected: "Add authentication",
		},
		{
			name:     "trims whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "replaces newlines",
			input:    "Line 1\nLine 2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "truncates long subjects",
			input:    strings.Repeat("a", 250),
			expected: strings.Repeat("a", 200) + "...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanSubject(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

// TestFormatPlanMessage verifies plan message body construction.
func TestFormatPlanMessage(t *testing.T) {
	content := `# My Plan
## Summary
A brief summary.
## Files to Modify
- file1.go
- file2.go
## Phase 1
Do things.`

	result := formatPlanMessage(content, "/tmp/plan.md", "AI summary")

	require.Contains(t, result, "## Summary")
	require.Contains(t, result, "AI summary")
	require.Contains(t, result, "## Key Files")
	require.Contains(t, result, "- file1.go")
	require.Contains(t, result, "## Plan Details")
	require.Contains(t, result, "`/tmp/plan.md`")
	require.Contains(t, result, "# My Plan")
}

// TestFormatPlanMessageNoSummary verifies message without summary.
func TestFormatPlanMessageNoSummary(t *testing.T) {
	content := "# Simple Plan\nJust do it."
	result := formatPlanMessage(content, "/tmp/plan.md", "")

	require.NotContains(t, result, "## Summary")
	require.Contains(t, result, "## Plan Details")
	require.Contains(t, result, "Just do it.")
}

// TestKeywordPatterns verifies the approval/rejection regex patterns.
func TestKeywordPatterns(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		approve bool
		reject  bool
		changes bool
	}{
		{
			name: "approve", text: "I approve this plan",
			approve: true,
		},
		{
			name: "approved", text: "This is approved",
			approve: true,
		},
		{
			name: "lgtm", text: "LGTM!",
			approve: true,
		},
		{
			name: "looks good", text: "looks good to me",
			approve: true,
		},
		{
			name: "ship it", text: "ship it!",
			approve: true,
		},
		{
			name: "reject", text: "I reject this plan",
			reject: true,
		},
		{
			name: "rejected", text: "Plan rejected.",
			reject: true,
		},
		{
			name: "nack", text: "nack, not ready",
			reject: true,
		},
		{
			name:    "changes requested",
			text:    "Changes requested on this plan",
			changes: true,
		},
		{
			name:    "needs changes",
			text:    "This needs changes before proceeding",
			changes: true,
		},
		{
			name:    "neutral comment",
			text:    "Interesting approach, let me think about it.",
			approve: false, reject: false, changes: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.approve,
				approveKeywords.MatchString(tc.text),
				"approve mismatch",
			)
			require.Equal(t, tc.reject,
				rejectKeywords.MatchString(tc.text),
				"reject mismatch",
			)
			require.Equal(t, tc.changes,
				changesKeywords.MatchString(tc.text),
				"changes mismatch",
			)
		})
	}
}

// TestNewPlanReviewID verifies UUID generation.
func TestNewPlanReviewID(t *testing.T) {
	id1, err := newPlanReviewID()
	require.NoError(t, err)
	require.NotEmpty(t, id1)

	id2, err := newPlanReviewID()
	require.NoError(t, err)
	require.NotEqual(t, id1, id2, "IDs should be unique")
}

// TestResolvePlanSessionID verifies session ID resolution precedence.
func TestResolvePlanSessionID(t *testing.T) {
	// Save and restore globals.
	origPlanSessID := planSessionID
	origSessID := sessionID
	defer func() {
		planSessionID = origPlanSessID
		sessionID = origSessID
	}()

	// planSessionID takes precedence.
	planSessionID = "plan-sess"
	sessionID = "global-sess"
	require.Equal(t, "plan-sess", resolvePlanSessionID())

	// Falls back to global sessionID.
	planSessionID = ""
	require.Equal(t, "global-sess", resolvePlanSessionID())

	// Falls back to env var.
	sessionID = ""
	t.Setenv("CLAUDE_SESSION_ID", "env-sess")
	require.Equal(t, "env-sess", resolvePlanSessionID())

	// Returns empty if nothing set.
	t.Setenv("CLAUDE_SESSION_ID", "")
	require.Equal(t, "", resolvePlanSessionID())
}

// TestExtractSectionContent verifies section content extraction limits.
func TestExtractSectionContent(t *testing.T) {
	lines := []string{
		"## Heading",
		"Line 1",
		"",
		"Line 2",
		"Line 3",
		"## Next Heading",
		"Line 4",
	}

	// Should extract up to maxLines non-empty lines, stopping at heading.
	result := extractSectionContent(lines, 1, 5)
	require.Equal(t, "Line 1\nLine 2\nLine 3", result)

	// Should respect maxLines.
	result = extractSectionContent(lines, 1, 2)
	require.Equal(t, "Line 1\nLine 2", result)

	// Should handle startIdx beyond range.
	result = extractSectionContent(lines, 100, 5)
	require.Equal(t, "", result)
}
