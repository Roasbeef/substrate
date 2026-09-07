package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSessionIDFromEnv(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	require.Empty(t, getSessionIDFromEnv())

	t.Setenv("CODEX_THREAD_ID", "codex-thread")
	require.Equal(t, "codex-thread", getSessionIDFromEnv())

	t.Setenv("CODEX_SESSION_ID", "codex-session")
	require.Equal(t, "codex-session", getSessionIDFromEnv())

	t.Setenv("CLAUDE_SESSION_ID", "claude-session")
	require.Equal(t, "claude-session", getSessionIDFromEnv())
}

func TestGetProjectDirFromEnv(t *testing.T) {
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("CODEX_PROJECT_DIR", "/codex/project")
	require.Equal(t, "/codex/project", getProjectDirFromEnv())

	t.Setenv("CLAUDE_PROJECT_DIR", "/claude/project")
	require.Equal(t, "/claude/project", getProjectDirFromEnv())
}
