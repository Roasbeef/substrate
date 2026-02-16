package summary

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProjectDir tests the project directory resolution for both
// mangled and unmangled project keys.
func TestProjectDir(t *testing.T) {
	reader := NewTranscriptReader("/home/user/.claude", 10)

	tests := []struct {
		name       string
		projectKey string
		want       string
	}{
		{
			name:       "already mangled",
			projectKey: "-Users-alice-code-myproject",
			want:       "/home/user/.claude/projects/-Users-alice-code-myproject",
		},
		{
			name:       "absolute path with slashes",
			projectKey: "/Users/alice/code/myproject",
			want:       "/home/user/.claude/projects/-Users-alice-code-myproject",
		},
		{
			name:       "path with dots",
			projectKey: "/Users/alice/github.com/repo",
			want:       "/home/user/.claude/projects/-Users-alice-github-com-repo",
		},
		{
			name:       "mangled with dots replaced",
			projectKey: "-Users-alice-github-com-repo",
			want:       "/home/user/.claude/projects/-Users-alice-github-com-repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reader.projectDir(tc.projectKey)
			if got != tc.want {
				t.Errorf("projectDir(%q) = %q, want %q",
					tc.projectKey, got, tc.want,
				)
			}
		})
	}
}

// TestProjectDirPathTraversal ensures that directory traversal
// attempts are sanitized.
func TestProjectDirPathTraversal(t *testing.T) {
	reader := NewTranscriptReader("/home/user/.claude", 10)

	tests := []struct {
		name       string
		projectKey string
	}{
		{
			name:       "dot-dot in path",
			projectKey: "/home/user/../../../etc/passwd",
		},
		{
			name:       "dot-dot in mangled key",
			projectKey: "-home-user-..-..-..-etc-passwd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := reader.projectDir(tc.projectKey)
			base := filepath.Join(
				"/home/user/.claude", "projects",
			)

			// The result must be under basePath/projects/.
			rel, err := filepath.Rel(base, dir)
			if err != nil {
				t.Fatalf("filepath.Rel failed: %v", err)
			}
			// Should not start with ".." meaning it escaped.
			if len(rel) >= 2 && rel[:2] == ".." {
				t.Errorf(
					"projectDir(%q) = %q escapes base %q",
					tc.projectKey, dir, base,
				)
			}
		})
	}
}

// TestReadRecentTranscript tests reading a transcript file with
// line tailing.
func TestReadRecentTranscript(t *testing.T) {
	// Create a temporary project directory structure.
	tmpDir := t.TempDir()
	projectDir := filepath.Join(
		tmpDir, "projects",
		"-Users-test-myproject",
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a transcript file with 10 lines.
	lines := ""
	for i := 1; i <= 10; i++ {
		lines += `{"type":"message","num":` +
			string(rune('0'+i)) + "}\n"
	}
	sessionID := "abc-123-def"
	transcriptPath := filepath.Join(
		projectDir, sessionID+".jsonl",
	)
	if err := os.WriteFile(
		transcriptPath, []byte(lines), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	reader := NewTranscriptReader(tmpDir, 3)

	data, err := reader.ReadRecentTranscript(
		"-Users-test-myproject", sessionID,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain only the last 3 lines.
	if data.Content == "" {
		t.Fatal("expected non-empty content")
	}
	if data.Hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

// TestReadRecentTranscriptNotFound tests the not-found sentinel.
func TestReadRecentTranscriptNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "projects", "-nonexistent")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	reader := NewTranscriptReader(tmpDir, 10)

	_, err := reader.ReadRecentTranscript(
		"-nonexistent", "no-such-session",
	)
	if err == nil {
		t.Fatal("expected error for missing transcript")
	}
}

// TestFindActiveSession tests session discovery from project dir.
func TestFindActiveSession(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(
		tmpDir, "projects", "-Users-test-proj",
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create two session files with different mod times.
	older := filepath.Join(projectDir, "old-session.jsonl")
	newer := filepath.Join(projectDir, "new-session.jsonl")

	if err := os.WriteFile(
		older, []byte("old"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	// Ensure the newer file has a later mod time.
	if err := os.WriteFile(
		newer, []byte("new"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	reader := NewTranscriptReader(tmpDir, 10)

	sessionID, err := reader.FindActiveSession(
		"-Users-test-proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "new-session" {
		t.Errorf(
			"session = %q, want %q",
			sessionID, "new-session",
		)
	}
}

// TestFindActiveSessionEmpty tests when no sessions exist.
func TestFindActiveSessionEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(
		tmpDir, "projects", "-empty-proj",
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	reader := NewTranscriptReader(tmpDir, 10)

	_, err := reader.FindActiveSession("-empty-proj")
	if err == nil {
		t.Fatal("expected error for empty project dir")
	}
}

// TestIsSessionFile tests the session file name detection.
func TestIsSessionFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"abc.jsonl", true},
		{"abc.json", true},
		{"abc.txt", false},
		{"abc", false},
		{".jsonl", true},
	}

	for _, tc := range tests {
		got := isSessionFile(tc.name)
		if got != tc.want {
			t.Errorf(
				"isSessionFile(%q) = %v, want %v",
				tc.name, got, tc.want,
			)
		}
	}
}
