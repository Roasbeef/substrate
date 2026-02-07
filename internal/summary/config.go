package summary

import "time"

const (
	// DefaultModel is the Claude model used for summarization.
	DefaultModel = "claude-haiku-4-5-20251001"

	// DefaultCacheTTL is how long a cached summary remains valid.
	DefaultCacheTTL = 30 * time.Second

	// DefaultRefreshInterval is the background refresh period.
	DefaultRefreshInterval = 45 * time.Second

	// DefaultMaxTranscriptLines is the max lines read from a
	// session transcript for summarization.
	DefaultMaxTranscriptLines = 200

	// DefaultMaxConcurrent is the max simultaneous Haiku calls.
	DefaultMaxConcurrent = 3

	// DefaultHistoryLimit is the default number of summary history
	// entries returned.
	DefaultHistoryLimit = 20
)

// Config holds configuration for the summary service.
type Config struct {
	// Enabled controls whether summarization is active.
	Enabled bool

	// Model is the Claude model to use for summarization.
	Model string

	// CacheTTL is how long cached summaries remain valid.
	CacheTTL time.Duration

	// RefreshInterval is the background refresh period.
	RefreshInterval time.Duration

	// MaxTranscriptLines is the max lines to read from transcripts.
	MaxTranscriptLines int

	// MaxConcurrent is the max simultaneous Haiku calls.
	MaxConcurrent int

	// TranscriptBasePath is the base path for Claude session data.
	TranscriptBasePath string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:            true,
		Model:              DefaultModel,
		CacheTTL:           DefaultCacheTTL,
		RefreshInterval:    DefaultRefreshInterval,
		MaxTranscriptLines: DefaultMaxTranscriptLines,
		MaxConcurrent:      DefaultMaxConcurrent,
		TranscriptBasePath: "",
	}
}
