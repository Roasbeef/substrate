// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/mailclient"
	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserAgentName is the name of the agent used for human-sent messages.
const UserAgentName = "User"

// Server is the HTTP server for the Subtrate web UI.
type Server struct {
	store        store.Storage
	registry     *agent.Registry
	heartbeatMgr *agent.HeartbeatManager
	mailClient   *mailclient.Client         // Shared mail client (required).
	actClient    *mailclient.ActivityClient // Shared activity client (required).
	notifHubRef  NotificationHubRef         // Notification hub reference (optional).
	hub          *Hub                       // WebSocket hub for real-time updates.
	notifBridge  *HubNotificationBridge     // Bridge for actor notifications to WebSocket.
	mux          *http.ServeMux
	gatewayMux   *runtime.ServeMux // grpc-gateway REST proxy mux (optional).
	srv          *http.Server
	addr         string
	grpcEndpoint string // gRPC endpoint for gateway proxy.
}

// Config holds configuration for the web server.
type Config struct {
	Addr string

	// MailRef is the mail actor reference (required).
	MailRef mail.MailActorRef

	// ActivityRef is the activity actor reference (required).
	ActivityRef activity.ActivityActorRef

	// NotificationHubRef is the notification hub reference (optional).
	// When provided, enables real-time actor-based notifications to WebSocket clients.
	NotificationHubRef NotificationHubRef

	// GRPCEndpoint is the gRPC server endpoint for the gateway proxy (optional).
	// When provided, enables REST-to-gRPC proxy via grpc-gateway.
	// Example: "localhost:10009"
	GRPCEndpoint string
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr: ":8080",
	}
}

// NewServer creates a new web server. The config must have non-nil MailRef and
// ActivityRef since the server requires actor system support for all operations.
func NewServer(cfg *Config, st store.Storage,
	registry *agent.Registry,
) (*Server, error) {
	// Validate required actor refs are provided.
	if cfg.MailRef == nil {
		return nil, fmt.Errorf("config.MailRef is required")
	}
	if cfg.ActivityRef == nil {
		return nil, fmt.Errorf("config.ActivityRef is required")
	}

	heartbeatMgr := agent.NewHeartbeatManager(registry, nil)

	s := &Server{
		store:        st,
		registry:     registry,
		heartbeatMgr: heartbeatMgr,
		mailClient:   mailclient.NewClient(cfg.MailRef),
		actClient:    mailclient.NewActivityClient(cfg.ActivityRef),
		notifHubRef:  cfg.NotificationHubRef,
		mux:          http.NewServeMux(),
		addr:         cfg.Addr,
		grpcEndpoint: cfg.GRPCEndpoint,
	}

	// API v1 routes are now served by grpc-gateway REST proxy.
	// The manual routes in api_v1.go are deprecated.

	// Register grpc-gateway REST proxy if gRPC endpoint is configured.
	if cfg.GRPCEndpoint != "" {
		if err := s.registerGateway(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to register grpc-gateway: %w", err)
		}
	}

	// Initialize and start WebSocket hub.
	s.hub = NewHub(s)
	go s.hub.Run()

	// Start notification bridge if notification hub is configured.
	if cfg.NotificationHubRef != nil {
		s.notifBridge = NewHubNotificationBridge(s.hub, cfg.NotificationHubRef)
		s.notifBridge.Start()
	}

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
	// Stop the notification bridge first.
	if s.notifBridge != nil {
		s.notifBridge.Stop()
	}

	// Stop the WebSocket hub.
	if s.hub != nil {
		s.hub.Stop()
	}

	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// registerGateway sets up the grpc-gateway REST proxy to forward requests to the
// gRPC server. This allows REST clients to access gRPC services via HTTP/JSON.
func (s *Server) registerGateway(ctx context.Context) error {
	// Create gateway mux with custom JSON marshaling options.
	s.gatewayMux = runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			EmitDefaults: true,
			OrigName:     true,
		}),
	)

	// gRPC dial options for connecting to the gRPC server.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Register Mail service handler.
	err := subtraterpc.RegisterMailHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register Mail handler: %w", err)
	}

	// Register Agent service handler.
	err = subtraterpc.RegisterAgentHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register Agent handler: %w", err)
	}

	// Register Session service handler.
	err = subtraterpc.RegisterSessionHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register Session handler: %w", err)
	}

	// Register Activity service handler.
	err = subtraterpc.RegisterActivityHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register Activity handler: %w", err)
	}

	// Register Stats service handler.
	err = subtraterpc.RegisterStatsHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf("failed to register Stats handler: %w", err)
	}

	// Register ReviewService handler.
	err = subtraterpc.RegisterReviewServiceHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to register ReviewService handler: %w", err,
		)
	}

	// Register TaskService handler.
	err = subtraterpc.RegisterTaskServiceHandlerFromEndpoint(
		ctx, s.gatewayMux, s.grpcEndpoint, opts,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to register TaskService handler: %w", err,
		)
	}

	// Mount the gateway at /api/v1/ as the primary API endpoint.
	// The gateway paths in mail.yaml already include /api/v1, so no prefix stripping needed.
	s.mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		s.gatewayMux.ServeHTTP(w, r)
	})

	log.Printf("grpc-gateway REST proxy registered at /api/v1/ -> %s", s.grpcEndpoint)
	return nil
}
