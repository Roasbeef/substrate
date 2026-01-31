// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// UserAgentName is the name of the agent used for human-sent messages.
const UserAgentName = "User"

// Server is the HTTP server for the Subtrate web UI.
type Server struct {
	store        *db.Store
	registry     *agent.Registry
	heartbeatMgr *agent.HeartbeatManager
	actorRefs    *ActorRefs // Optional actor references for Ask/Tell pattern.
	hub          *Hub       // WebSocket hub for real-time updates.
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
		mux:          http.NewServeMux(),
		addr:         cfg.Addr,
	}

	// Register API v1 routes (JSON API for React frontend).
	s.registerAPIV1Routes()

	// Initialize and start WebSocket hub.
	s.hub = NewHub(s)
	go s.hub.Run()

	// Register WebSocket route.
	s.mux.HandleFunc("/ws", s.handleWebSocket)

	// Serve React frontend for all other routes.
	frontendHandler, err := FrontendHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to create frontend handler: %w", err)
	}
	s.mux.Handle("/", frontendHandler)

	return s, nil
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
	// Stop the WebSocket hub first.
	if s.hub != nil {
		s.hub.Stop()
	}

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
