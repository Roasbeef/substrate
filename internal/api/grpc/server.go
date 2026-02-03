package subtraterpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/mailclient"
)

// ServerConfig holds configuration for the gRPC server.
type ServerConfig struct {
	// ListenAddr is the address to listen on (e.g., "localhost:10009").
	ListenAddr string

	// ServerPingTime is the duration after which the server pings the client.
	// If not set, defaults to 5 minutes.
	ServerPingTime time.Duration

	// ServerPingTimeout is the duration the server waits for ping ack.
	// If not set, defaults to 1 minute.
	ServerPingTimeout time.Duration

	// ClientPingMinWait is the minimum time between client pings.
	// If not set, defaults to 5 seconds.
	ClientPingMinWait time.Duration

	// ClientAllowPingWithoutStream allows pings even without active streams.
	ClientAllowPingWithoutStream bool

	// MailRef is the actor reference for mail operations (required).
	MailRef mail.MailActorRef

	// ActivityRef is the actor reference for activity operations (required).
	ActivityRef activity.ActivityActorRef
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddr:                   "localhost:10009",
		ServerPingTime:               5 * time.Minute,
		ServerPingTimeout:            1 * time.Minute,
		ClientPingMinWait:            5 * time.Second,
		ClientAllowPingWithoutStream: true,
	}
}

// Server is the gRPC server for Subtrate.
type Server struct {
	cfg          ServerConfig
	store        *db.Store
	mailSvc      *mail.Service              // Direct service for operations not via actor.
	mailClient   *mailclient.Client         // Shared mail client (required).
	actClient    *mailclient.ActivityClient // Shared activity client (required).
	agentReg     *agent.Registry
	identityMgr  *agent.IdentityManager
	heartbeatMgr *agent.HeartbeatManager

	// notificationHub is the actor reference for the notification hub.
	// Used for event-driven message delivery to streaming clients.
	notificationHub actor.ActorRef[mail.NotificationRequest, mail.NotificationResponse]

	grpcServer *grpc.Server
	listener   net.Listener

	started bool
	mu      sync.RWMutex

	// quit is closed when the server is shutting down.
	quit chan struct{}
	wg   sync.WaitGroup

	UnimplementedMailServer
	UnimplementedAgentServer
	UnimplementedSessionServer
	UnimplementedActivityServer
	UnimplementedStatsServer
}

// NewServer creates a new gRPC server instance.
func NewServer(
	cfg ServerConfig,
	store *db.Store,
	mailSvc *mail.Service,
	agentReg *agent.Registry,
	identityMgr *agent.IdentityManager,
	heartbeatMgr *agent.HeartbeatManager,
	notificationHub actor.ActorRef[mail.NotificationRequest, mail.NotificationResponse],
) *Server {
	return &Server{
		cfg:             cfg,
		store:           store,
		mailSvc:         mailSvc,
		mailClient:      mailclient.NewClient(cfg.MailRef),
		actClient:       mailclient.NewActivityClient(cfg.ActivityRef),
		agentReg:        agentReg,
		identityMgr:     identityMgr,
		heartbeatMgr:    heartbeatMgr,
		notificationHub: notificationHub,
		quit:            make(chan struct{}),
	}
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("server already started")
	}

	// Create listener.
	lis, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.ListenAddr, err)
	}
	s.listener = lis

	// Build server options with keepalive and interceptors.
	opts := s.buildServerOptions()

	// Create gRPC server.
	s.grpcServer = grpc.NewServer(opts...)

	// Register services.
	RegisterMailServer(s.grpcServer, s)
	RegisterAgentServer(s.grpcServer, s)
	RegisterSessionServer(s.grpcServer, s)
	RegisterActivityServer(s.grpcServer, s)
	RegisterStatsServer(s.grpcServer, s)

	// Start serving in a goroutine.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		slog.Info("gRPC server listening", "addr", s.cfg.ListenAddr)
		if err := s.grpcServer.Serve(lis); err != nil {
			// Only log if not a graceful shutdown.
			select {
			case <-s.quit:
			default:
				slog.Error("gRPC server error", "error", err)
			}
		}
	}()

	s.started = true
	return nil
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	close(s.quit)
	s.grpcServer.GracefulStop()
	s.wg.Wait()

	s.started = false
	slog.Info("gRPC server stopped")
	return nil
}

// buildServerOptions creates gRPC server options with keepalive and interceptors.
// Pattern based on lnd's server configuration.
func (s *Server) buildServerOptions() []grpc.ServerOption {
	// Keepalive settings - mirrors lnd's approach for long-lived connections.
	serverKeepalive := keepalive.ServerParameters{
		// Time after which server pings the client if no activity.
		Time: s.cfg.ServerPingTime,
		// Timeout for ping ack before considering connection dead.
		Timeout: s.cfg.ServerPingTimeout,
	}

	// Client keepalive enforcement policy.
	clientKeepalive := keepalive.EnforcementPolicy{
		// Minimum time between client pings.
		MinTime: s.cfg.ClientPingMinWait,
		// Allow pings even when there are no active streams.
		PermitWithoutStream: s.cfg.ClientAllowPingWithoutStream,
	}

	return []grpc.ServerOption{
		grpc.KeepaliveParams(serverKeepalive),
		grpc.KeepaliveEnforcementPolicy(clientKeepalive),

		// Chain unary interceptors: logging -> request validation.
		grpc.ChainUnaryInterceptor(
			s.loggingUnaryInterceptor,
			s.validationUnaryInterceptor,
		),

		// Chain stream interceptors for streaming RPCs.
		grpc.ChainStreamInterceptor(
			s.loggingStreamInterceptor,
		),
	}
}

// loggingUnaryInterceptor logs all unary RPC calls.
// Based on lnd's rpcperms interceptor pattern.
func (s *Server) loggingUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	// Log the incoming request.
	slog.Debug("RPC request",
		"method", info.FullMethod,
	)

	// Call the handler.
	resp, err := handler(ctx, req)

	// Log the result.
	duration := time.Since(start)
	if err != nil {
		slog.Warn("RPC failed",
			"method", info.FullMethod,
			"duration", duration,
			"error", err,
		)
	} else {
		slog.Debug("RPC completed",
			"method", info.FullMethod,
			"duration", duration,
		)
	}

	return resp, err
}

// validationUnaryInterceptor validates common request parameters.
func (s *Server) validationUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// Check if server is shutting down.
	select {
	case <-s.quit:
		return nil, status.Error(codes.Unavailable, "server is shutting down")
	default:
	}

	return handler(ctx, req)
}

// loggingStreamInterceptor logs streaming RPC calls.
func (s *Server) loggingStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	slog.Debug("Stream RPC started",
		"method", info.FullMethod,
	)

	err := handler(srv, ss)

	duration := time.Since(start)
	if err != nil {
		slog.Warn("Stream RPC failed",
			"method", info.FullMethod,
			"duration", duration,
			"error", err,
		)
	} else {
		slog.Debug("Stream RPC completed",
			"method", info.FullMethod,
			"duration", duration,
		)
	}

	return err
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}
