package main

import (
	"context"
	"flag"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btcsuite/btclog/v2"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/build"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/mcp"
	"github.com/roasbeef/subtrate/internal/review"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/roasbeef/subtrate/internal/summary"
	"github.com/roasbeef/subtrate/internal/web"
)

func main() {
	var (
		dbPath         = flag.String("db", "~/.subtrate/subtrate.db", "Path to SQLite database")
		webAddr        = flag.String("web", ":8080", "Web server address (empty to disable)")
		grpcAddr       = flag.String("grpc", "localhost:10009", "gRPC server address (empty to disable)")
		enableMCP      = flag.Bool("mcp", false, "Enable MCP stdio transport (default: web + gRPC only)")
		logDir         = flag.String("log-dir", "~/.subtrate/logs", "Directory for log files (empty to disable file logging)")
		maxLogFiles    = flag.Int("max-log-files", build.DefaultMaxLogFiles, "Maximum number of rotated log files to keep")
		maxLogFileSize = flag.Int("max-log-file-size", build.DefaultMaxLogFileSize, "Maximum log file size in MB before rotation")
	)
	flag.Parse()

	// Expand home directory in paths.
	expandHome := func(path string) string {
		expanded := os.ExpandEnv(path)
		if expanded == path && len(path) > 0 && path[0] == '~' {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf(
					"Failed to get home directory: %v",
					err,
				)
			}
			expanded = home + path[1:]
		}
		return expanded
	}

	dbPathExpanded := expandHome(*dbPath)
	logDirExpanded := expandHome(*logDir)

	// Initialize the rotating log file writer if a log directory is
	// configured. This creates ~/.subtrate/logs/substrated.log with
	// automatic rotation and gzip compression of old files.
	var logRotator *build.RotatingLogWriter
	if logDirExpanded != "" {
		logRotator = build.NewRotatingLogWriter()
		err := logRotator.InitLogRotator(
			&build.LogRotatorConfig{
				LogDir:         logDirExpanded,
				MaxLogFiles:    *maxLogFiles,
				MaxLogFileSize: *maxLogFileSize,
			},
		)
		if err != nil {
			log.Printf(
				"Failed to init log rotator: %v "+
					"(continuing without file logging)",
				err,
			)
			logRotator = nil
		} else {
			defer logRotator.Close()

			// Redirect the standard log package to write to both
			// stderr and the log file (only if rotator init
			// succeeded).
			multiWriter := io.MultiWriter(os.Stderr, logRotator)
			log.SetOutput(multiWriter)
			log.SetFlags(log.LstdFlags)
		}
	}

	// Log version and build information at startup.
	log.Printf("substrated version %s commit=%s go=%s",
		build.Version(), commitInfo(), build.GoVersion,
	)

	// Create a logger for the database.
	logger := slog.Default()

	// Open the database with migrations.
	sqliteStore, err := db.NewSqliteStore(&db.SqliteConfig{
		DatabaseFileName: dbPathExpanded,
	}, logger)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer sqliteStore.Close()

	// Get the underlying store for services.
	dbStore := sqliteStore.Store
	storage := store.FromDB(dbStore.DB())

	// Create agent registry, heartbeat manager, and identity manager.
	agentReg := agent.NewRegistry(dbStore)

	// Ensure the default "User" agent exists for the web UI inbox.
	if _, err := agentReg.EnsureDefaultAgent(
		context.Background(), web.UserAgentName,
	); err != nil {
		log.Fatalf("Failed to ensure User agent: %v", err)
	}

	heartbeatMgr := agent.NewHeartbeatManager(agentReg, nil)
	identityMgr, err := agent.NewIdentityManager(dbStore, agentReg)
	if err != nil {
		log.Fatalf("Failed to create identity manager: %v", err)
	}

	// Create btclog handlers for structured subsystem logging. When file
	// logging is enabled, logs go to both the console and the rotating
	// log file (matching lnd's dual-stream pattern).
	var btclogHandlers []btclog.Handler
	consoleHandler := btclog.NewDefaultHandler(os.Stderr)
	btclogHandlers = append(btclogHandlers, consoleHandler)

	if logRotator != nil {
		fileHandler := btclog.NewDefaultHandler(logRotator)
		btclogHandlers = append(btclogHandlers, fileHandler)

		log.Printf(
			"Log file rotation enabled: dir=%s, max_files=%d, "+
				"max_size=%dMB",
			logDirExpanded, *maxLogFiles, *maxLogFileSize,
		)
	}

	// Combine handlers into a single btclog.Handler via HandlerSet.
	// This fans out each log record to all handlers (console + file).
	combinedHandler := build.NewHandlerSet(btclogHandlers...)

	// Wire up the actor system's btclog logger so lifecycle events
	// (registration, shutdown, stop) are visible in daemon logs.
	actorLogger := btclog.NewSLogger(combinedHandler)
	actor.UseLogger(actorLogger)

	// Wire up the review subsystem logger so reviewer lifecycle events
	// (spawn, stop, result parsing) are visible in daemon logs.
	reviewLogger := actorLogger.WithPrefix(review.Subsystem)
	review.UseLogger(reviewLogger)

	// Create the actor system.
	actorSystem := actor.NewActorSystem()
	defer func() {
		// Use a bounded timeout to prevent indefinite blocking
		// if actor cleanup stalls (e.g., reviewer subprocess
		// shutdown). 30s allows headroom for the reviewer's 15s
		// WithCleanupTimeout plus the SDK's 5s transport SIGKILL.
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer shutdownCancel()

		if err := actorSystem.Shutdown(shutdownCtx); err != nil {
			log.Printf(
				"Actor system shutdown incomplete: %v "+
					"(some goroutines may have leaked)",
				err,
			)
		}
	}()

	// Create and register the notification hub for real-time notifications.
	// This hub receives notifications from the mail service and delivers them
	// to WebSocket clients and gRPC streams.
	notificationHub := actor.RegisterWithSystem(
		actorSystem,
		"notification-hub",
		mail.NotificationHubKey,
		mail.NewNotificationHub(),
	)
	log.Println("NotificationHub actor started")

	// Create the mail service with the notification hub reference.
	// This enables real-time notifications when messages are sent.
	mailSvc := mail.NewService(mail.ServiceConfig{
		Store:           storage,
		NotificationHub: notificationHub,
	})

	// Register the mail service as an actor.
	mailRef := actor.RegisterWithSystem(
		actorSystem,
		"mail-service",
		mail.MailServiceKey,
		mailSvc,
	)
	log.Println("Mail actor started with NotificationHub integration")

	// Create and register the activity actor.
	activitySvc := activity.NewService(activity.ServiceConfig{
		Store: storage,
	})
	activityRef := actor.RegisterWithSystem(
		actorSystem,
		"activity-service",
		activity.ActivityServiceKey,
		activitySvc,
	)
	log.Println("Activity actor started")

	// Create and register the review service actor. This handles code
	// review orchestration, FSM state management, and reviewer agent
	// coordination. The actor system reference enables reviewer sub-actors
	// to be registered as proper actors for graceful shutdown.
	reviewSvc := review.NewService(review.ServiceConfig{
		Store:       storage,
		ActorSystem: actorSystem,
	})
	reviewRef := actor.RegisterWithSystem(
		actorSystem,
		"review-service",
		review.ReviewServiceKey,
		reviewSvc,
		actor.WithCleanupTimeout(30*time.Second),
	)

	// Recover any active reviews from the database (restart recovery).
	if err := reviewSvc.RecoverActiveReviews(
		context.Background(),
	); err != nil {
		log.Printf(
			"Warning: failed to recover active reviews: %v", err,
		)
	}
	log.Printf(
		"Review actor started (%d active reviews recovered)",
		reviewSvc.ActiveReviewCount(),
	)

	// Create the summary service for agent activity summaries.
	summaryCfg := summary.DefaultConfig()
	summarySvc := summary.NewService(
		summaryCfg, storage, slog.Default(),
	)
	log.Println("Summary service created")

	// Create the MCP server if MCP stdio mode is enabled.
	var mcpServer *mcp.Server
	if *enableMCP {
		mcpServer = mcp.NewServer(dbStore)
	}

	// Set up signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf(
			"Received %v, initiating graceful shutdown "+
				"(send again to force exit)...", sig,
		)
		cancel()

		// Wait for a second signal to force-exit. The goroutine
		// stays alive so subsequent Ctrl+C signals are consumed
		// rather than silently dropped by the buffered channel.
		sig = <-sigCh
		log.Printf(
			"Received %v again, forcing immediate exit",
			sig,
		)
		os.Exit(1)
	}()

	// Start gRPC server if enabled.
	var grpcServer *subtraterpc.Server
	if *grpcAddr != "" {
		grpcCfg := subtraterpc.DefaultServerConfig()
		grpcCfg.ListenAddr = *grpcAddr
		grpcCfg.MailRef = mailRef
		grpcCfg.ActivityRef = activityRef
		grpcCfg.ReviewRef = reviewRef

		// Pass the notification hub actor for gRPC streaming RPCs.
		grpcServer = subtraterpc.NewServer(
			grpcCfg, dbStore, mailSvc, agentReg, identityMgr,
			heartbeatMgr, notificationHub,
		)
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
		defer grpcServer.Stop()
		log.Printf("gRPC server listening on %s", *grpcAddr)
	}

	// Start the web server if enabled.
	if *webAddr != "" {
		webCfg := web.DefaultConfig()
		webCfg.Addr = *webAddr
		webCfg.MailRef = mailRef
		webCfg.ActivityRef = activityRef
		webCfg.NotificationHubRef = web.NewActorNotificationHubRef(notificationHub)

		// Enable grpc-gateway REST proxy if gRPC server is running.
		if *grpcAddr != "" {
			webCfg.GRPCEndpoint = *grpcAddr
		}

		// Wire summary service into the web server.
		webCfg.SummarySvc = summarySvc

		webServer, err := web.NewServer(webCfg, storage, agentReg)
		if err != nil {
			log.Fatalf("Failed to create web server: %v", err)
		}

		// Wire summary update notifications from the summary service to
		// WebSocket hub so the UI refreshes when summaries are generated.
		if summarySvc != nil && webServer.GetHub() != nil {
			summarySvc.OnSummaryGenerated = func(
				agentID int64, summaryText, delta string,
			) {
				webServer.GetHub().BroadcastSummaryUpdate(
					agentID, map[string]any{
						"summary": summaryText,
						"delta":   delta,
					},
				)
			}
		}

		// Wire task change notifications from gRPC server to WebSocket hub
		// so the UI updates in real time when tasks are created or modified.
		if grpcServer != nil && webServer.GetHub() != nil {
			grpcServer.SetTaskNotifier(
				web.NewHubTaskNotifier(webServer.GetHub()),
			)
			log.Println("Task change notifications wired: gRPC → WebSocket")
		}

		go func() {
			log.Printf("Starting web server on %s", *webAddr)
			if err := webServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("Web server error: %v", err)
			}
		}()

		// Shut down web server on context cancellation.
		go func() {
			<-ctx.Done()
			webServer.Shutdown(context.Background())
		}()
	}

	// Start the background summary refresh loop.
	go summarySvc.RunBackgroundRefresh(ctx)
	log.Println("Summary background refresh started")

	// Run the MCP server on stdio transport if enabled, otherwise
	// block until signal.
	if *enableMCP {
		log.Println("Starting substrated MCP server...")
		if err := mcpServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		log.Println("Running in web+gRPC mode (no MCP stdio)")
		// Block until signal received.
		<-ctx.Done()
	}
}

// commitInfo returns the best available commit identifier. It prefers the
// Commit string set via ldflags (which includes tag info), falling back to
// the VCS commit hash from runtime/debug.
func commitInfo() string {
	if build.Commit != "" {
		return build.Commit
	}
	if build.CommitHash != "" {
		return build.CommitHash
	}

	return "dev"
}
