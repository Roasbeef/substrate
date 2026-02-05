package build

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jrick/logrotate/rotator"
)

const (
	// DefaultMaxLogFiles is the default maximum number of rotated log
	// files to keep on disk.
	DefaultMaxLogFiles = 10

	// DefaultMaxLogFileSize is the default maximum log file size in MB
	// before rotation occurs.
	DefaultMaxLogFileSize = 20

	// DefaultLogFilename is the default log file name used when no
	// custom name is provided.
	DefaultLogFilename = "substrated.log"
)

// LogRotatorConfig holds the configuration for the log file rotator.
type LogRotatorConfig struct {
	// LogDir is the directory where log files are written.
	LogDir string

	// MaxLogFiles is the maximum number of rotated log files to keep.
	// Set to 0 to disable rotation (single file, unbounded growth).
	MaxLogFiles int

	// MaxLogFileSize is the maximum size of a log file in megabytes
	// before it is rotated.
	MaxLogFileSize int

	// Filename overrides the default log file name. If empty,
	// DefaultLogFilename is used.
	Filename string
}

// DefaultLogRotatorConfig returns a LogRotatorConfig with sane defaults.
func DefaultLogRotatorConfig() *LogRotatorConfig {
	return &LogRotatorConfig{
		MaxLogFiles:    DefaultMaxLogFiles,
		MaxLogFileSize: DefaultMaxLogFileSize,
		Filename:       DefaultLogFilename,
	}
}

// RotatingLogWriter wraps a jrick/logrotate rotator with a pipe-based
// io.Writer interface. It supports gzip compression of rotated files.
type RotatingLogWriter struct {
	// pipe is the write-end of the pipe feeding the rotator goroutine.
	pipe *io.PipeWriter

	// rotator manages file rotation with size limits and compression.
	rotator *rotator.Rotator
}

// NewRotatingLogWriter creates a new rotating log writer. InitLogRotator
// must be called before writing.
func NewRotatingLogWriter() *RotatingLogWriter {
	return &RotatingLogWriter{}
}

// InitLogRotator initializes the log file rotator. It creates the log
// directory if needed, configures rotation parameters, and starts the
// rotator goroutine. Must be called before the first Write.
func (r *RotatingLogWriter) InitLogRotator(cfg *LogRotatorConfig) error {
	filename := cfg.Filename
	if filename == "" {
		filename = DefaultLogFilename
	}

	logFile := filepath.Join(cfg.LogDir, filename)
	logDir := filepath.Dir(logFile)

	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create the rotator with size in kilobytes (config is in MB).
	var err error
	r.rotator, err = rotator.New(
		logFile,
		int64(cfg.MaxLogFileSize*1024),
		false,
		cfg.MaxLogFiles,
	)
	if err != nil {
		return fmt.Errorf("failed to create file rotator: %w", err)
	}

	// Use gzip compression for rotated files.
	r.rotator.SetCompressor(gzip.NewWriter(nil), ".gz")

	// Run the rotator in a background goroutine. Errors are logged to
	// stderr since the rotator itself is the log destination.
	pr, pw := io.Pipe()
	go func() {
		err := r.rotator.Run(pr)
		if err != nil {
			_, _ = fmt.Fprintf(
				os.Stderr,
				"failed to run file rotator: %v\n", err,
			)
		}
	}()

	r.pipe = pw

	return nil
}

// Write writes the byte slice to the log rotator pipe. If the rotator
// has not been initialized, the write is silently discarded.
func (r *RotatingLogWriter) Write(b []byte) (int, error) {
	if r.pipe != nil {
		return r.pipe.Write(b)
	}

	return len(b), nil
}

// Close closes the pipe writer, which signals the rotator goroutine to
// flush and exit. It also closes the underlying rotator.
func (r *RotatingLogWriter) Close() error {
	if r.pipe != nil {
		return r.pipe.Close()
	}

	return nil
}
