package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SummaryResponse is the JSON response for a single agent summary.
type SummaryResponse struct {
	AgentID     int64   `json:"agent_id"`
	AgentName   string  `json:"agent_name"`
	Summary     string  `json:"summary"`
	Delta       string  `json:"delta"`
	GeneratedAt string  `json:"generated_at"`
	IsStale     bool    `json:"is_stale"`
	CostUSD     float64 `json:"cost_usd"`
}

// SummaryHistoryEntry is a single entry in the summary history.
type SummaryHistoryEntry struct {
	ID             int64   `json:"id"`
	AgentID        int64   `json:"agent_id"`
	Summary        string  `json:"summary"`
	Delta          string  `json:"delta"`
	TranscriptHash string  `json:"transcript_hash"`
	CostUSD        float64 `json:"cost_usd"`
	CreatedAt      string  `json:"created_at"`
}

// registerSummaryRoutes registers the summary REST API endpoints.
func (s *Server) registerSummaryRoutes() {
	s.mux.HandleFunc(
		"/api/v1/agents/summaries", s.handleGetAllSummaries,
	)
	s.mux.HandleFunc(
		"/api/v1/agents/summary/", s.handleAgentSummaryRoutes,
	)
}

// handleGetAllSummaries returns summaries for all active agents.
func (s *Server) handleGetAllSummaries(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.summarySvc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "summary service not available",
		})
		return
	}

	ctx := r.Context()

	// Get active only filter.
	activeOnly := r.URL.Query().Get("active_only") != "false"

	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list agents",
		})
		return
	}

	var agentIDs []int64
	agentNames := make(map[int64]string)
	for _, a := range agents {
		if activeOnly {
			// Only include agents active in last 30 minutes.
			if time.Since(a.LastActiveAt) > 30*time.Minute {
				continue
			}
		}
		agentIDs = append(agentIDs, a.ID)
		agentNames[a.ID] = a.Name
	}

	results, err := s.summarySvc.GetAllSummaries(ctx, agentIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get summaries",
		})
		return
	}

	resp := make([]SummaryResponse, 0, len(results))
	for _, r := range results {
		resp = append(resp, SummaryResponse{
			AgentID:     r.AgentID,
			AgentName:   agentNames[r.AgentID],
			Summary:     r.Summary,
			Delta:       r.Delta,
			GeneratedAt: r.GeneratedAt.Format(time.RFC3339),
			IsStale:     r.IsStale,
			CostUSD:     r.CostUSD,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summaries": resp,
	})
}

// handleAgentSummaryRoutes routes agent-specific summary requests.
func (s *Server) handleAgentSummaryRoutes(
	w http.ResponseWriter, r *http.Request,
) {
	// Parse agent ID from URL: /api/v1/agents/summary/{agent_id}
	// or /api/v1/agents/summary/{agent_id}/history
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/summary/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "history" {
		s.handleGetSummaryHistory(w, r, agentID)
		return
	}

	s.handleGetAgentSummary(w, r, agentID)
}

// handleGetAgentSummary returns the current summary for an agent.
func (s *Server) handleGetAgentSummary(
	w http.ResponseWriter, r *http.Request, agentID int64,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.summarySvc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "summary service not available",
		})
		return
	}

	ctx := r.Context()

	result, err := s.summarySvc.GetSummary(ctx, agentID)
	if err != nil {
		// Return a null summary for benign errors (disabled
		// service, missing transcript) instead of a 500.
		if strings.Contains(err.Error(), "disabled") ||
			strings.Contains(err.Error(), "transcript") ||
			strings.Contains(err.Error(), "missing project_key") {

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"summary": nil,
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	if result == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"summary": nil,
		})
		return
	}

	// Look up agent name.
	agentName := ""
	agent, err := s.store.GetAgent(ctx, agentID)
	if err == nil {
		agentName = agent.Name
	}

	resp := SummaryResponse{
		AgentID:     result.AgentID,
		AgentName:   agentName,
		Summary:     result.Summary,
		Delta:       result.Delta,
		GeneratedAt: result.GeneratedAt.Format(time.RFC3339),
		IsStale:     result.IsStale,
		CostUSD:     result.CostUSD,
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary": resp,
	})
}

// handleGetSummaryHistory returns summary history for an agent.
func (s *Server) handleGetSummaryHistory(
	w http.ResponseWriter, r *http.Request, agentID int64,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.summarySvc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "summary service not available",
		})
		return
	}

	ctx := r.Context()

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history, err := s.summarySvc.GetSummaryHistory(
		ctx, agentID, limit,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	entries := make([]SummaryHistoryEntry, 0, len(history))
	for _, h := range history {
		entries = append(entries, SummaryHistoryEntry{
			ID:             h.ID,
			AgentID:        h.AgentID,
			Summary:        h.Summary,
			Delta:          h.Delta,
			TranscriptHash: h.TranscriptHash,
			CostUSD:        h.CostUSD,
			CreatedAt:      h.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": entries,
	})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
