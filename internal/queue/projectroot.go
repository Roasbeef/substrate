package queue

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot locates the project root directory. It tries the given
// projectDir first, then walks up looking for a .git directory, and falls
// back to the current working directory.
func FindProjectRoot(projectDir string) (string, error) {
	// If an explicit project directory was provided, use it directly.
	if projectDir != "" {
		abs, err := filepath.Abs(projectDir)
		if err != nil {
			return "", fmt.Errorf("resolve project dir: %w", err)
		}

		return abs, nil
	}

	// Walk up from the current directory looking for .git.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := cwd
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .git.
			break
		}

		dir = parent
	}

	// Fall back to the current working directory.
	return cwd, nil
}

// QueueDBPath returns the path to the queue database file within the
// given project root.
func QueueDBPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".substrate", "queue.db")
}

// EnsureQueueDir creates the .substrate directory within the project root
// if it does not already exist.
func EnsureQueueDir(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".substrate")

	return os.MkdirAll(dir, 0o700)
}
