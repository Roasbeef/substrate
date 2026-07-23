package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// attachMaxBytes caps uploaded attachment size.
const attachMaxBytes = 8 << 20 // 8 MiB

// attachExts whitelists servable attachment extensions and their
// content types. Images only for now; the shared directory convention
// leaves room for more types later.
var attachExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// AttachmentsDir returns the shared attachments directory. Both the
// daemon (uploads, serving) and the CLI (substrate send --attach)
// write here, so images travel by filename reference instead of
// bloating message bodies.
func AttachmentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".subtrate", "attachments")
}

// UploadResponse is the payload for a successful attachment upload.
type UploadResponse struct {
	File     string `json:"file"`
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
}

// registerAttachmentRoutes registers upload and serving endpoints.
func (s *Server) registerAttachmentRoutes() {
	s.mux.HandleFunc("/api/v1/command/upload", s.handleAttachUpload)
	s.mux.HandleFunc("/api/v1/attachments/", s.handleAttachServe)
}

// handleAttachUpload stores a dropped image in the shared attachments
// directory under a random name and returns the markdown reference to
// embed in a message body.
func (s *Server) handleAttachUpload(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Error(
			w, "Method not allowed", http.StatusMethodNotAllowed,
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, attachMaxBytes)
	if err := r.ParseMultipartForm(attachMaxBytes); err != nil {
		http.Error(w, "upload too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := attachExts[ext]; !ok {
		http.Error(
			w, "unsupported file type", http.StatusBadRequest,
		)
		return
	}

	// Random filename prevents collisions and path games; the
	// original name survives only in the markdown alt text.
	var rb [12]byte
	if _, err := rand.Read(rb[:]); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	name := hex.EncodeToString(rb[:]) + ext

	dir := AttachmentsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	url := "/api/v1/attachments/" + name
	writeJSON(w, http.StatusOK, UploadResponse{
		File: name,
		URL:  url,
		Markdown: fmt.Sprintf(
			"![%s](%s)", filepath.Base(header.Filename), url,
		),
	})
}

// handleAttachServe streams a stored attachment. Names are strictly
// basename + whitelisted extension, so no traversal is possible.
func (s *Server) handleAttachServe(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		http.Error(
			w, "Method not allowed", http.StatusMethodNotAllowed,
		)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/attachments/")
	if name != filepath.Base(name) || name == "" {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}

	ctype, ok := attachExts[strings.ToLower(filepath.Ext(name))]
	if !ok {
		http.Error(w, "unsupported type", http.StatusBadRequest)
		return
	}

	f, err := os.Open(filepath.Join(AttachmentsDir(), name))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, f) //nolint:errcheck
}
