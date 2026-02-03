// Package web provides the embedded React frontend filesystem.
package web

import (
	"embed"
	"io/fs"
)

// FrontendFS embeds the built React frontend application.
// The frontend must be built before building the Go binary:
//
//	cd web/frontend && bun run build
//
// This embeds all files from web/frontend/dist/ into the binary.
//
//go:embed all:frontend/dist
var FrontendFS embed.FS

// GetDistFS returns the dist subdirectory as a filesystem for serving.
// This unwraps the "frontend/dist" prefix from embedded paths.
func GetDistFS() (fs.FS, error) {
	return fs.Sub(FrontendFS, "frontend/dist")
}
