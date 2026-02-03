package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/mcp"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/roasbeef/subtrate/internal/web"
)

func main() {
	var (
		dbPath   = flag.String("db", "~/.subtrate/subtrate.db", "Path to SQLite database")
		webAddr  = flag.String("web", ":8080", "Web server address (empty to disable)")
		grpcAddr = flag.String("grpc", "localhost:10009", "gRPC server address (empty to disable)")
		webOnly  = flag.Bool("web-only", false, "Run web + gRPC servers only (no MCP stdio)")
	)
	flag.Parse()

	// Expand home directory.
	dbPathExpanded := os.ExpandEnv(*dbPath)
	if dbPathExpanded == *dbPath && (*dbPath)[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		dbPathExpanded = home + (*dbPath)[1:]
	}

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
	heartbeatMgr := agent.NewHeartbeatManager(agentReg, nil)
	identityMgr, err := agent.NewIdentityManager(dbStore, agentReg)
	if err != nil {
		log.Fatalf("Failed to create identity manager: %v", err)
	}

	// Create the actor system.
	actorSystem := actor.NewActorSystem()
	defer actorSystem.Shutdown(context.Background())

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

	// Create the MCP server (unless web-only mode).
	var mcpServer *mcp.Server
	if !*webOnly {
		mcpServer = mcp.NewServer(dbStore)
	}

	// Set up signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Start gRPC server if enabled.
	var grpcServer *subtraterpc.Server
	if *grpcAddr != "" {
		grpcCfg := subtraterpc.DefaultServerConfig()
		grpcCfg.ListenAddr = *grpcAddr
		grpcCfg.MailRef = mailRef
		grpcCfg.ActivityRef = activityRef

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

		webServer, err := web.NewServer(webCfg, storage, agentReg)
		if err != nil {
			log.Fatalf("Failed to create web server: %v", err)
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

	// Run the MCP server on stdio transport, unless web-only mode.
	if *webOnly {
		log.Println("Running in web+gRPC mode (no MCP stdio)")
		// Block until signal received.
		<-ctx.Done()
	} else {
		log.Println("Starting substrated MCP server...")
		if err := mcpServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}
