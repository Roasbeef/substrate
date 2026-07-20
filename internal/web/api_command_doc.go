package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// docMaxBytes caps how much of a referenced document is returned.
const docMaxBytes = 1 << 20 // 1 MiB

// AgentDocResponse is the payload for GET /api/v1/command/doc/{id}.
type AgentDocResponse struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Content  string `json:"content"`
	// Truncated reports that the file exceeded the size cap and only
	// the head is returned.
	Truncated bool `json:"truncated"`
}

// registerCommandDocRoutes registers the document viewer endpoint.
func (s *Server) registerCommandDocRoutes() {
	s.mux.HandleFunc("/api/v1/command/doc/", s.handleAgentDoc)
}

// handleAgentDoc serves a file referenced in an agent's message,
// resolved against that agent's working directory. Paths are strictly
// confined to the working directory.
func (s *Server) handleAgentDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w, "Method not allowed", http.StatusMethodNotAllowed,
		)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/command/doc/")
	agentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	agent, err := s.store.GetAgent(r.Context(), agentID)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	base := agent.WorkingDir
	if base == "" && strings.HasPrefix(agent.ProjectKey, "/") {
		base = agent.ProjectKey
	}
	if base == "" {
		http.Error(
			w, "agent has no working directory",
			http.StatusNotFound,
		)
		return
	}

	full, ok := resolveDocPath(base, relPath)
	if !ok {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(full)
	if err != nil {
		http.Error(w, "file not readable", http.StatusNotFound)
		return
	}
	defer f.Close()

	buf := make([]byte, docMaxBytes)
	n, _ := f.Read(buf)

	writeJSON(w, http.StatusOK, AgentDocResponse{
		Path:      relPath,
		Size:      fi.Size(),
		Modified:  fi.ModTime().UTC().Format(time.RFC3339),
		Content:   string(buf[:n]),
		Truncated: fi.Size() > docMaxBytes,
	})
}

// resolveDocPath joins a relative document path onto an agent's
// working directory, rejecting absolute paths and traversal outside
// the base.
func resolveDocPath(base, rel string) (string, bool) {
	if filepath.IsAbs(rel) {
		return "", false
	}

	full := filepath.Clean(filepath.Join(base, rel))
	cleanBase := filepath.Clean(base)
	if full != cleanBase &&
		!strings.HasPrefix(full, cleanBase+string(filepath.Separator)) {

		return "", false
	}

	return full, true
}
