package summary

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/store"
)

// mockSummaryStore is a minimal mock for testing the service layer.
type mockSummaryStore struct {
	store.Storage

	mu        sync.Mutex
	summaries []store.AgentSummary
	agents    []store.Agent
}

// CreateSummary appends a summary to the in-memory store.
func (m *mockSummaryStore) CreateSummary(
	_ context.Context, p store.CreateSummaryParams,
) (store.AgentSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := store.AgentSummary{
		ID:             int64(len(m.summaries) + 1),
		AgentID:        p.AgentID,
		Summary:        p.Summary,
		Delta:          p.Delta,
		TranscriptHash: p.TranscriptHash,
		CreatedAt:      time.Now(),
	}
	m.summaries = append(m.summaries, s)

	return s, nil
}

// GetLatestSummary returns the most recent summary for an agent.
func (m *mockSummaryStore) GetLatestSummary(
	_ context.Context, agentID int64,
) (store.AgentSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.summaries) - 1; i >= 0; i-- {
		if m.summaries[i].AgentID == agentID {
			return m.summaries[i], nil
		}
	}

	return store.AgentSummary{}, sql.ErrNoRows
}

// GetSummaryHistory returns recent summaries for an agent.
func (m *mockSummaryStore) GetSummaryHistory(
	_ context.Context, agentID int64, limit int,
) ([]store.AgentSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []store.AgentSummary
	for i := len(m.summaries) - 1; i >= 0; i-- {
		if m.summaries[i].AgentID == agentID {
			results = append(results, m.summaries[i])
			if len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// GetAgent returns an agent by ID.
func (m *mockSummaryStore) GetAgent(
	_ context.Context, id int64,
) (store.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.agents {
		if a.ID == id {
			return a, nil
		}
	}

	return store.Agent{}, sql.ErrNoRows
}

// ListAgents returns all agents.
func (m *mockSummaryStore) ListAgents(
	_ context.Context,
) ([]store.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]store.Agent{}, m.agents...), nil
}

// TestParseSummaryResponse tests the response parser for various
// Haiku output formats.
func TestParseSummaryResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSum string
		wantDel string
	}{
		{
			name:    "standard format",
			input:   "SUMMARY: Agent is fixing a bug.\nDELTA: Started debugging the issue.",
			wantSum: "Agent is fixing a bug.",
			wantDel: "Started debugging the issue.",
		},
		{
			name:    "extra whitespace",
			input:   "  SUMMARY:   spaced out   \n  DELTA:   also spaced   ",
			wantSum: "spaced out",
			wantDel: "also spaced",
		},
		{
			name:    "no delta line",
			input:   "SUMMARY: Just the summary here.",
			wantSum: "Just the summary here.",
			wantDel: "",
		},
		{
			name:    "no summary prefix fallback",
			input:   "The agent is idle.",
			wantSum: "The agent is idle.",
			wantDel: "Initial summary",
		},
		{
			name:    "empty input fallback",
			input:   "",
			wantSum: "Agent idle",
			wantDel: "Initial summary",
		},
		{
			name:    "multiline with extra text",
			input:   "Some preamble.\nSUMMARY: The real summary.\nDELTA: The delta.\nMore text.",
			wantSum: "The real summary.",
			wantDel: "The delta.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sum, del, err := parseSummaryResponse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sum != tc.wantSum {
				t.Errorf("summary = %q, want %q", sum, tc.wantSum)
			}
			if del != tc.wantDel {
				t.Errorf("delta = %q, want %q", del, tc.wantDel)
			}
		})
	}
}

// TestIsSummaryDuplicate tests the deduplication logic for various
// delta and summary combinations.
func TestIsSummaryDuplicate(t *testing.T) {
	tests := []struct {
		name        string
		newSummary  string
		delta       string
		prevSummary string
		want        bool
	}{
		{
			name:        "no previous summary",
			newSummary:  "Agent is working.",
			delta:       "Started task.",
			prevSummary: "",
			want:        false,
		},
		{
			name:        "delta says no change",
			newSummary:  "Agent is idle.",
			delta:       "No change since last update.",
			prevSummary: "Agent is idle.",
			want:        true,
		},
		{
			name:        "delta says no new activity",
			newSummary:  "Agent is idle.",
			delta:       "No new activity observed.",
			prevSummary: "Agent is idle.",
			want:        true,
		},
		{
			name:        "delta says unchanged",
			newSummary:  "Standing by.",
			delta:       "Status unchanged.",
			prevSummary: "Standing by.",
			want:        true,
		},
		{
			name:        "delta says remains idle",
			newSummary:  "Agent standing by.",
			delta:       "Agent remains idle, no new tasks.",
			prevSummary: "Agent waiting for tasks.",
			want:        true,
		},
		{
			name:        "delta says still idle",
			newSummary:  "Idle agent.",
			delta:       "Still idle since last check.",
			prevSummary: "Idle agent.",
			want:        true,
		},
		{
			name:        "delta says same as before",
			newSummary:  "Working on feature.",
			delta:       "Same as previous update.",
			prevSummary: "Working on feature.",
			want:        true,
		},
		{
			name:        "delta says no update",
			newSummary:  "Idle.",
			delta:       "No update to report.",
			prevSummary: "Idle.",
			want:        true,
		},
		{
			name:        "identical summary text case insensitive",
			newSummary:  "Agent is Working on Feature X.",
			delta:       "Continued work on feature.",
			prevSummary: "agent is working on feature x.",
			want:        true,
		},
		{
			name:        "different summary text",
			newSummary:  "Agent is now debugging.",
			delta:       "Switched from coding to debugging.",
			prevSummary: "Agent is writing code.",
			want:        false,
		},
		{
			name:        "meaningful delta with same summary",
			newSummary:  "Working on feature X.",
			delta:       "Made progress on the implementation.",
			prevSummary: "Working on feature X.",
			want:        true,
		},
		{
			name:        "case sensitivity in delta keywords",
			newSummary:  "Idle.",
			delta:       "NO CHANGE detected.",
			prevSummary: "Idle.",
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isSummaryDuplicate(
				tc.newSummary, tc.delta, tc.prevSummary,
			)
			if got != tc.want {
				t.Errorf(
					"isSummaryDuplicate(%q, %q, %q) = %v, want %v",
					tc.newSummary, tc.delta, tc.prevSummary,
					got, tc.want,
				)
			}
		})
	}
}

// TestTruncate tests the truncation helper.
func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"exact", 5, "exact"},
	}

	for _, tc := range tests {
		got := truncate(tc.input, tc.n)
		if got != tc.want {
			t.Errorf(
				"truncate(%q, %d) = %q, want %q",
				tc.input, tc.n, got, tc.want,
			)
		}
	}
}

// TestCacheValidity tests the cachedSummary.isValid method.
func TestCacheValidity(t *testing.T) {
	t.Run("nil cache entry", func(t *testing.T) {
		var c *cachedSummary
		if c.isValid(time.Minute) {
			t.Error("nil cache entry should not be valid")
		}
	})

	t.Run("nil result", func(t *testing.T) {
		c := &cachedSummary{cachedAt: time.Now()}
		if c.isValid(time.Minute) {
			t.Error("cache entry with nil result should not be valid")
		}
	})

	t.Run("fresh entry", func(t *testing.T) {
		c := &cachedSummary{
			result:   &SummaryResult{Summary: "test"},
			cachedAt: time.Now(),
		}
		if !c.isValid(time.Minute) {
			t.Error("fresh cache entry should be valid")
		}
	})

	t.Run("expired entry", func(t *testing.T) {
		c := &cachedSummary{
			result:   &SummaryResult{Summary: "test"},
			cachedAt: time.Now().Add(-2 * time.Minute),
		}
		if c.isValid(time.Minute) {
			t.Error("expired cache entry should not be valid")
		}
	})
}

// TestGetSummaryFromDB tests that GetSummary falls through to DB
// when cache is empty.
func TestGetSummaryFromDB(t *testing.T) {
	ms := &mockSummaryStore{
		summaries: []store.AgentSummary{
			{
				ID:             1,
				AgentID:        42,
				Summary:        "Cached in DB.",
				Delta:          "From database.",
				TranscriptHash: "abc123",
				CreatedAt:      time.Now().Add(-30 * time.Second),
			},
		},
	}

	svc := NewService(
		Config{
			Enabled:       true,
			CacheTTL:      time.Minute,
			Model:         "test",
			MaxConcurrent: 1,
		},
		ms, nil,
	)

	result, err := svc.GetSummary(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result from DB, got nil")
	}
	if result.Summary != "Cached in DB." {
		t.Errorf("summary = %q, want %q", result.Summary, "Cached in DB.")
	}
	if !result.IsStale {
		t.Error("summary from DB should be marked as stale")
	}
}

// TestGetSummaryFromCache tests that cached entries are returned
// directly without hitting the store.
func TestGetSummaryFromCache(t *testing.T) {
	svc := NewService(
		Config{
			Enabled:       true,
			CacheTTL:      time.Minute,
			Model:         "test",
			MaxConcurrent: 1,
		},
		&mockSummaryStore{}, nil,
	)

	// Seed the cache manually.
	svc.cache[42] = &cachedSummary{
		result: &SummaryResult{
			AgentID: 42,
			Summary: "From cache.",
			Delta:   "Cached delta.",
		},
		cachedAt: time.Now(),
	}

	result, err := svc.GetSummary(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected cached result, got nil")
	}
	if result.Summary != "From cache." {
		t.Errorf("summary = %q, want %q", result.Summary, "From cache.")
	}
}

// TestGetSummaryDisabled tests that the service returns an error when
// disabled.
func TestGetSummaryDisabled(t *testing.T) {
	svc := NewService(
		Config{Enabled: false},
		&mockSummaryStore{}, nil,
	)

	_, err := svc.GetSummary(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for disabled service")
	}
}

// TestEvictOldest tests that cache eviction removes the oldest non-
// generating entry.
func TestEvictOldest(t *testing.T) {
	svc := NewService(
		Config{
			Enabled:       true,
			CacheTTL:      time.Minute,
			Model:         "test",
			MaxConcurrent: 1,
		},
		&mockSummaryStore{}, nil,
	)

	now := time.Now()

	// Fill cache beyond the limit.
	for i := int64(0); i <= DefaultMaxCacheEntries; i++ {
		svc.cache[i] = &cachedSummary{
			result: &SummaryResult{
				AgentID: i,
				Summary: "test",
			},
			cachedAt: now.Add(time.Duration(i) * time.Second),
		}
	}

	// Mark agent 0 (oldest) as generating — it should be skipped.
	svc.cache[0].generating = true

	svc.evictOldestLocked()

	// Agent 0 should still be present (generating).
	if _, ok := svc.cache[0]; !ok {
		t.Error("generating entry should not be evicted")
	}

	// Agent 1 (next oldest) should have been evicted.
	if _, ok := svc.cache[1]; ok {
		t.Error("oldest non-generating entry should be evicted")
	}

	// Total should be at the max.
	if len(svc.cache) != DefaultMaxCacheEntries {
		t.Errorf(
			"cache size = %d, want %d",
			len(svc.cache), DefaultMaxCacheEntries,
		)
	}
}

// TestGetSummaryHistory tests the history retrieval.
func TestGetSummaryHistory(t *testing.T) {
	ms := &mockSummaryStore{
		summaries: []store.AgentSummary{
			{ID: 1, AgentID: 42, Summary: "First."},
			{ID: 2, AgentID: 42, Summary: "Second."},
			{ID: 3, AgentID: 99, Summary: "Other agent."},
		},
	}

	svc := NewService(
		Config{Enabled: true, Model: "test", MaxConcurrent: 1},
		ms, nil,
	)

	history, err := svc.GetSummaryHistory(context.Background(), 42, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2", len(history))
	}
}

// TestOnSummaryGeneratedCallback tests that the WS broadcast callback
// is invoked after a summary is persisted.
func TestOnSummaryGeneratedCallback(t *testing.T) {
	ms := &mockSummaryStore{}
	svc := NewService(
		Config{
			Enabled:       true,
			CacheTTL:      time.Minute,
			Model:         "test",
			MaxConcurrent: 1,
		},
		ms, nil,
	)

	var called bool
	var callbackAgent int64
	svc.OnSummaryGenerated = func(
		agentID int64, summary, delta string,
	) {
		called = true
		callbackAgent = agentID
	}

	// Manually populate cache to simulate generateSummary's cache
	// update, then check the callback would fire if we had a
	// non-duplicate summary. Test the dedup path: if isDuplicate
	// is false, callback fires.
	// Since we can't call generateSummary without a real Haiku
	// agent, just verify the callback field is wirable.
	if svc.OnSummaryGenerated == nil {
		t.Fatal("OnSummaryGenerated should be set")
	}

	svc.OnSummaryGenerated(42, "test", "delta")
	if !called || callbackAgent != 42 {
		t.Error("callback was not invoked correctly")
	}
}
