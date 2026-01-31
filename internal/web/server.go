// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed static/css/*.css static/js/*.js static/js/vendor/*.js static/icons/*.svg
var staticFS embed.FS

// UserAgentName is the name of the agent used for human-sent messages.
const UserAgentName = "User"

// Server is the HTTP server for the Subtrate web UI.
type Server struct {
	store        *db.Store
	registry     *agent.Registry
	heartbeatMgr *agent.HeartbeatManager
	actorRefs    *ActorRefs // Optional actor references for Ask/Tell pattern.
	templates    map[string]*template.Template // Page-specific template sets.
	partials     *template.Template            // Shared partials.
	mux          *http.ServeMux
	srv          *http.Server
	addr         string
	userAgentID  int64 // Cached ID for the User agent.
}

// Config holds configuration for the web server.
type Config struct {
	Addr string

	// ActorRefs holds optional actor references for the Ask/Tell pattern.
	// If nil, the server will use direct database access.
	ActorRefs *ActorRefs
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr: ":8080",
	}
}

// NewServer creates a new web server.
func NewServer(cfg *Config, store *db.Store) (*Server, error) {
	registry := agent.NewRegistry(store)
	heartbeatMgr := agent.NewHeartbeatManager(registry, nil)

	s := &Server{
		store:        store,
		registry:     registry,
		heartbeatMgr: heartbeatMgr,
		actorRefs:    cfg.ActorRefs,
		templates:    make(map[string]*template.Template),
		mux:          http.NewServeMux(),
		addr:         cfg.Addr,
	}

	// Common template functions.
	funcMap := template.FuncMap{
		"formatTime":     formatTime,
		"formatTimeAgo":  formatTimeAgo,
		"formatDateTime": formatDateTime,
		"formatDuration": formatDuration,
		"formatDeadline": formatDeadline,
		"timeSection":    timeSection,
		"truncate":       truncate,
		"markdown":       markdownToHTML,
		"shortProject":   shortProject,
	}

	// Parse shared partials.
	partials, err := template.New("").Funcs(funcMap).ParseFS(
		templatesFS,
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse partials: %w", err)
	}
	s.partials = partials

	// Parse each page template separately (with layout and partials).
	// This avoids conflicts between pages defining the same block names.
	pages := []string{"inbox.html", "agents-dashboard.html"}
	for _, page := range pages {
		// Clone partials and add layout + page template.
		tmpl, err := template.Must(partials.Clone()).ParseFS(
			templatesFS,
			"templates/layout.html",
			"templates/"+page,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		s.templates[page] = tmpl
	}

	// Register routes.
	s.registerRoutes()

	return s, nil
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// Static files with no-cache headers in dev mode.
	staticSub, _ := fs.Sub(staticFS, "static")
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.FS(staticSub)))
	noCacheStatic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable caching for JS files during development.
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		staticHandler.ServeHTTP(w, r)
	})
	s.mux.Handle("/static/", noCacheStatic)

	// Page routes.
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/inbox", s.handleInbox)
	s.mux.HandleFunc("/starred", s.handleStarred)
	s.mux.HandleFunc("/snoozed", s.handleSnoozed)
	s.mux.HandleFunc("/sent", s.handleSent)
	s.mux.HandleFunc("/archive", s.handleArchive)
	s.mux.HandleFunc("/agents", s.handleAgentsDashboard)
	s.mux.HandleFunc("/sessions", s.handleSessions)
	s.mux.HandleFunc("/settings", s.handleSettings)

	// HTMX partial routes.
	s.mux.HandleFunc("/inbox/messages", s.handleInboxMessages)
	s.mux.HandleFunc("/starred/messages", s.handleStarredMessages)
	s.mux.HandleFunc("/snoozed/messages", s.handleSnoozedMessages)
	s.mux.HandleFunc("/sent/messages", s.handleSentMessages)
	s.mux.HandleFunc("/archive/messages", s.handleArchivedMessages)
	s.mux.HandleFunc("/compose", s.handleCompose)
	s.mux.HandleFunc("/search", s.handleSearch)
	s.mux.HandleFunc("/thread/", s.handleThread)
	s.mux.HandleFunc("/topic/", s.handleTopicView)
	s.mux.HandleFunc("/agents/new", s.handleNewAgentModal)
	s.mux.HandleFunc("/agents/", s.handleAgentAction) // Catch-all for /agents/{id}, /agents/{id}/message, etc.
	s.mux.HandleFunc("/sessions/new", s.handleNewSessionModal)

	// API routes (return HTML partials for HTMX).
	s.mux.HandleFunc("/api/status", s.handleAPIStatus)
	s.mux.HandleFunc("/api/topics", s.handleAPITopics)
	s.mux.HandleFunc("/api/agents", s.handleAPIAgents)
	s.mux.HandleFunc("/api/agents/sidebar", s.handleAPIAgentsSidebar)
	s.mux.HandleFunc("/api/agents/cards", s.handleAPIAgentsCards)
	s.mux.HandleFunc("/api/activity", s.handleAPIActivity)
	s.mux.HandleFunc("/api/sessions/active", s.handleAPIActiveSessions)
	s.mux.HandleFunc("/api/sessions", s.handleAPISessionCreate)
	s.mux.HandleFunc("/api/heartbeat", s.handleAPIHeartbeat)
	s.mux.HandleFunc("/api/agents/status", s.handleAPIAgentsStatus)

	// Thread action routes.
	s.mux.HandleFunc("/api/threads/", s.handleThreadAction)

	// Message action routes.
	s.mux.HandleFunc("/api/messages/send", s.handleMessageSend)
	s.mux.HandleFunc("/api/messages/", s.handleMessageAction)

	// Autocomplete routes.
	s.mux.HandleFunc("/api/autocomplete/recipients", s.handleAutocompleteRecipients)

	// SSE event streams.
	s.mux.HandleFunc("/events/agents", s.handleSSEAgents)
	s.mux.HandleFunc("/events/activity", s.handleSSEActivity)
	s.mux.HandleFunc("/events/inbox", s.handleSSEInbox)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:         s.addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting web server on %s", s.addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// ensureUserAgent ensures the User agent exists and caches its ID.
func (s *Server) ensureUserAgent(ctx context.Context) error {
	// Try to get existing User agent.
	agent, err := s.store.Queries().GetAgentByName(ctx, UserAgentName)
	if err == nil {
		s.userAgentID = agent.ID
		return nil
	}

	// Create the User agent.
	agent, err = s.store.Queries().CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         UserAgentName,
		CreatedAt:    time.Now().Unix(),
		LastActiveAt: time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to create User agent: %w", err)
	}

	s.userAgentID = agent.ID
	log.Printf("Created User agent with ID %d", s.userAgentID)
	return nil
}

// getUserAgentID returns the cached User agent ID, initializing if needed.
func (s *Server) getUserAgentID(ctx context.Context) int64 {
	if s.userAgentID == 0 {
		if err := s.ensureUserAgent(ctx); err != nil {
			log.Printf("Warning: failed to ensure User agent: %v", err)
			return 0
		}
	}
	return s.userAgentID
}

// Template helper functions.

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("3:04 PM")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

// formatDateTime formats a time with date and time.
func formatDateTime(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Today at " + t.Format("3:04 PM")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2 at 3:04 PM")
	}
	return t.Format("Jan 2, 2006 at 3:04 PM")
}

// formatTimeAgo formats a time as a relative duration.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2")
	}
}

// formatDeadline formats a deadline time.
func formatDeadline(t time.Time) string {
	if t.IsZero() {
		return "No deadline"
	}
	now := time.Now()
	if t.Before(now) {
		return "Overdue"
	}
	d := t.Sub(now)
	if d < time.Hour {
		return fmt.Sprintf("in %d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("in %d hours", int(d.Hours()))
	}
	return t.Format("Jan 2 at 3:04 PM")
}

// formatDuration formats a duration since a start time.
func formatDuration(start time.Time) string {
	d := time.Since(start)
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// shortProject extracts the last 2 path segments from a project path.
// e.g., "/Users/foo/gocode/src/github.com/roasbeef/subtrate" -> "roasbeef/subtrate"
func shortProject(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return path
}

// markdownToHTML converts markdown to HTML using goldmark.
func markdownToHTML(s string) template.HTML {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		// Fallback to escaped text on error.
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(buf.String())
}

// timeSection returns a section label for grouping messages by time period.
// Used for Google Inbox-style time grouping (Today, Yesterday, This Week, Earlier).
func timeSection(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)

	if t.After(today) || t.Equal(today) {
		return "Today"
	}
	if t.After(yesterday) || t.Equal(yesterday) {
		return "Yesterday"
	}
	if t.After(weekAgo) {
		return "This Week"
	}
	// For older messages, show the month.
	if t.Year() == now.Year() {
		return t.Format("January")
	}
	return t.Format("January 2006")
}
