// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"net/http"
	"strings"

	subtrateweb "github.com/roasbeef/subtrate/web"
)

// FrontendHandler returns an http.Handler that serves the React frontend.
// It serves static files from the embedded filesystem and falls back to
// index.html for SPA routing.
func FrontendHandler() (http.Handler, error) {
	// Get the dist subdirectory from the embedded filesystem.
	distFS, err := subtrateweb.GetDistFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Serve API and WebSocket routes normally (handled elsewhere).
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file directly.
		if path != "/" {
			// Check if file exists in the embedded filesystem.
			f, err := distFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				f.Close()
				// Set cache headers for static assets.
				if strings.HasPrefix(path, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// For non-existent paths, serve index.html (SPA fallback).
		// This enables client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}), nil
}
