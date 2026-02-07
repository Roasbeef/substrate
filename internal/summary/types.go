package summary

import "time"

// SummaryResult holds the output of a Haiku summarization.
type SummaryResult struct {
	AgentID        int64
	AgentName      string
	Summary        string
	Delta          string
	TranscriptHash string
	GeneratedAt    time.Time
	CostUSD        float64
	IsStale        bool
	Error          error
}

// cachedSummary is an in-memory cache entry for an agent summary.
type cachedSummary struct {
	result         *SummaryResult
	transcriptHash string
	cachedAt       time.Time
	generating     bool
}

// isValid returns true if the cache entry is still fresh.
func (c *cachedSummary) isValid(ttl time.Duration) bool {
	if c == nil || c.result == nil {
		return false
	}
	return time.Since(c.cachedAt) < ttl
}
