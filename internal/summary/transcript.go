package summary

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrTranscriptNotFound is returned when no transcript file exists
// for the requested session.
var ErrTranscriptNotFound = errors.New("transcript not found")

// TranscriptReader reads Claude Code session transcripts from disk.
type TranscriptReader struct {
	basePath string
	maxLines int
}

// NewTranscriptReader creates a new TranscriptReader with the given base
// path and max lines to read.
func NewTranscriptReader(basePath string, maxLines int) *TranscriptReader {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			basePath = "/root"
		} else {
			basePath = home
		}
		basePath = filepath.Join(basePath, ".claude")
	}
	if maxLines <= 0 {
		maxLines = DefaultMaxTranscriptLines
	}
	return &TranscriptReader{basePath: basePath, maxLines: maxLines}
}

// TranscriptData holds the content and hash of a transcript.
type TranscriptData struct {
	Content string
	Hash    string
}

// maxTranscriptBytes is the maximum transcript file size we'll read
// into memory. Files larger than this are tail-read from the end.
const maxTranscriptBytes = 10 * 1024 * 1024 // 10 MB

// ReadRecentTranscript reads the tail of a session transcript file and
// returns both the content and a hash for cache invalidation.
func (r *TranscriptReader) ReadRecentTranscript(
	projectKey, sessionID string,
) (TranscriptData, error) {
	path, err := r.findTranscriptPath(projectKey, sessionID)
	if err != nil {
		return TranscriptData{}, fmt.Errorf(
			"find transcript: %w", err,
		)
	}

	// Check file size before reading to avoid unbounded memory
	// usage from very large transcript files.
	fi, err := os.Stat(path)
	if err != nil {
		return TranscriptData{}, fmt.Errorf(
			"stat transcript %s: %w", path, err,
		)
	}

	var data []byte
	if fi.Size() > maxTranscriptBytes {
		// For oversized files, read just the tail. Open the
		// file and seek to the last maxTranscriptBytes.
		f, fErr := os.Open(path)
		if fErr != nil {
			return TranscriptData{}, fmt.Errorf(
				"open transcript %s: %w", path, fErr,
			)
		}
		defer f.Close()

		if _, sErr := f.Seek(
			-maxTranscriptBytes, 2,
		); sErr != nil {
			return TranscriptData{}, fmt.Errorf(
				"seek transcript %s: %w", path, sErr,
			)
		}

		data, err = io.ReadAll(f)
		if err != nil {
			return TranscriptData{}, fmt.Errorf(
				"read transcript tail %s: %w", path, err,
			)
		}

		// Skip the first partial line after seeking.
		if idx := strings.IndexByte(string(data), '\n'); idx >= 0 {
			data = data[idx+1:]
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return TranscriptData{}, fmt.Errorf(
				"read transcript %s: %w", path, err,
			)
		}
	}

	content := string(data)

	// Tail to maxLines.
	lines := strings.Split(content, "\n")
	if len(lines) > r.maxLines {
		lines = lines[len(lines)-r.maxLines:]
		content = strings.Join(lines, "\n")
	}

	hash := sha256.Sum256([]byte(content))

	return TranscriptData{
		Content: content,
		Hash:    hex.EncodeToString(hash[:]),
	}, nil
}

// FindActiveSession discovers the most recent session file for a
// project key.
func (r *TranscriptReader) FindActiveSession(
	projectKey string,
) (string, error) {
	projectDir := r.projectDir(projectKey)

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf(
			"read project dir %s: %w", projectDir, err,
		)
	}

	// Find the most recently modified session file.
	type fileInfo struct {
		name    string
		modTime int64
	}

	var sessions []fileInfo
	for _, e := range entries {
		if e.IsDir() || !isSessionFile(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, fileInfo{
			name:    e.Name(),
			modTime: info.ModTime().Unix(),
		})
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf(
			"no session files in %s", projectDir,
		)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	// Return session ID (filename without extension).
	name := sessions[0].name
	return strings.TrimSuffix(name, filepath.Ext(name)), nil
}

// findTranscriptPath resolves the file path for a session transcript.
func (r *TranscriptReader) findTranscriptPath(
	projectKey, sessionID string,
) (string, error) {
	// Try several known locations for session data.
	candidates := []string{
		filepath.Join(
			r.projectDir(projectKey), sessionID+".jsonl",
		),
		filepath.Join(
			r.projectDir(projectKey), sessionID+".json",
		),
		filepath.Join(
			r.projectDir(projectKey),
			"sessions", sessionID+".jsonl",
		),
		filepath.Join(
			r.basePath, "projects",
			projectKey, sessionID+".jsonl",
		),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"%w for session %s in project %s",
		ErrTranscriptNotFound, sessionID, projectKey,
	)
}

// projectDir returns the directory for a project key. Claude Code
// stores project data in ~/.claude/projects/ using a mangled form of
// the working directory path where "/" is replaced with "-". If the
// given project key is already in mangled form, it is used as-is;
// otherwise we convert it.
func (r *TranscriptReader) projectDir(projectKey string) string {
	// If the key already looks like a Claude project key (starts
	// with "-" and contains no "/"), use it directly.
	if strings.HasPrefix(projectKey, "-") &&
		!strings.Contains(projectKey, "/") {

		return filepath.Join(
			r.basePath, "projects",
			filepath.Clean(projectKey),
		)
	}

	// Convert working directory path to Claude's mangled format.
	// Claude replaces both "/" and "." with "-":
	// /Users/roasbeef/github.com/foo → -Users-roasbeef-github-com-foo
	mangled := strings.ReplaceAll(projectKey, "/", "-")
	mangled = strings.ReplaceAll(mangled, ".", "-")

	// Sanitize against path traversal: the resulting directory
	// must remain under basePath/projects/.
	dir := filepath.Join(r.basePath, "projects", mangled)
	if !strings.HasPrefix(
		filepath.Clean(dir),
		filepath.Join(r.basePath, "projects"),
	) {
		// Fall back to a safe directory that won't exist,
		// causing transcript reads to return NotFound.
		return filepath.Join(r.basePath, "projects", "_invalid")
	}

	return dir
}

// isSessionFile checks if a filename looks like a session transcript.
func isSessionFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".jsonl" || ext == ".json"
}
