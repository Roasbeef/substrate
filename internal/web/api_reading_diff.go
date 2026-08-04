package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/readingdiff"
)

// maxReadingDiffRequestBytes bounds a request body. It sits above the
// compiler's own diff limit so an oversized patch produces the compiler's
// actionable message rather than a bare 413.
const maxReadingDiffRequestBytes = readingdiff.MaxDiffBytes + (32 << 10)

// readingDiffTimeout bounds one abridgement request end to end. It sits above
// the generator's own budget so a stuck model surfaces as the generator's
// clearer error rather than as a severed connection.
const readingDiffTimeout = 12 * time.Minute

// ReadingDiffRequest is the payload for POST /api/v1/reading-diff.
type ReadingDiffRequest struct {
	// Patch is the raw unified diff to abridge.
	Patch string `json:"patch"`

	// RepoPath, when set, lets the generator inspect the surrounding source
	// to judge whether a row is load-bearing.
	RepoPath string `json:"repo_path,omitempty"`
}

// ReadingDiffResponse is the abridged result plus everything the viewer needs
// to render it and expand elided regions.
type ReadingDiffResponse struct {
	// ReadingDiff is the abridged patch text.
	ReadingDiff string `json:"reading_diff"`

	// Summary is the one-line description of the change.
	Summary string `json:"summary"`

	// Elision is the human-readable retention manifest, for example
	// "kept 12/240 changed lines in 3/7 files".
	Elision string `json:"elision"`

	// Segments maps runs of original patch lines onto their fate, letting
	// the viewer offer to expand an elided region in place.
	Segments []readingdiff.Segment `json:"segments"`

	// Stats carries the raw retention counters.
	Stats readingdiff.Stats `json:"stats"`
}

// registerReadingDiffRoutes registers the reading-diff endpoint.
func (s *Server) registerReadingDiffRoutes() {
	s.mux.HandleFunc("/api/v1/reading-diff", s.handleReadingDiff)
}

// handleReadingDiff abridges a patch into a reading diff.
//
// The route is a POST because the patch travels in the body: a diff routinely
// runs to hundreds of kilobytes, far past any safe URL length, and it is the
// cache key rather than a resource identifier.
func (s *Server) handleReadingDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Extend this response's deadlines past the server-wide 15 second write
	// timeout.
	//
	// A cache miss runs a model over the whole patch and can take minutes,
	// which the shared timeout would cut off mid-flight — the connection dies
	// while the handler is still computing, so the caller waits out the whole
	// abridgement and then receives nothing. That made every fresh
	// abridgement impossible while cache hits, being instant, worked fine and
	// hid the problem.
	//
	// Raising the timeout for the whole server would weaken it for every other
	// endpoint, so the deadline is lifted only here.
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(readingDiffTimeout)
	if err := rc.SetWriteDeadline(deadline); err != nil {
		log.Printf("reading-diff: cannot extend write deadline: %v", err)
	}
	if err := rc.SetReadDeadline(deadline); err != nil {
		log.Printf("reading-diff: cannot extend read deadline: %v", err)
	}

	if s.readingDiffs == nil {
		// The feature needs a model-backed generator. A daemon started
		// without one should say so plainly rather than fail obscurely.
		http.Error(
			w, "reading diffs are not enabled on this server",
			http.StatusServiceUnavailable,
		)

		return
	}

	var req ReadingDiffRequest
	dec := json.NewDecoder(http.MaxBytesReader(
		w, r.Body, maxReadingDiffRequestBytes,
	))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)

		return
	}

	if strings.TrimSpace(req.Patch) == "" {
		http.Error(w, "patch is required", http.StatusBadRequest)

		return
	}

	res, err := s.readingDiffs.Get(r.Context(), readingdiff.Request{
		UnifiedDiff: req.Patch,
		RepoRoot:    req.RepoPath,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)

		return
	}

	segments := res.Segments
	if segments == nil {
		segments = []readingdiff.Segment{}
	}

	writeJSON(w, http.StatusOK, ReadingDiffResponse{
		ReadingDiff: res.ReadingDiff,
		Summary:     res.Summary,
		Elision:     res.ElisionLine(),
		Segments:    segments,
		Stats:       res.Stats,
	})
}
