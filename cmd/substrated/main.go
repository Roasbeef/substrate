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

	"github.com/lightninglabs/darepo-client/baselib/actor"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/mcp"
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
	store := sqliteStore.Store

	// Create core services.
	mailSvc := mail.NewService(store)
	agentReg := agent.NewRegistry(store)
	identityMgr, err := agent.NewIdentityManager(store, agentReg)
	if err != nil {
		log.Fatalf("Failed to create identity manager: %v", err)
	}

	// Create the actor system and notification hub for gRPC streaming.
	actorSystem := actor.NewActorSystem()
	defer actorSystem.Shutdown(context.Background())

	notificationHub := actor.RegisterWithSystem(
		actorSystem,
		"notification-hub",
		mail.NotificationHubKey,
		mail.NewNotificationHub(),
	)
	log.Println("NotificationHub actor started")

	// Create the MCP server (unless web-only mode).
	var mcpServer *mcp.Server
	if !*webOnly {
		mcpServer = mcp.NewServer(store)
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

		// Pass the notification hub actor for gRPC streaming RPCs.
		grpcServer = subtraterpc.NewServer(
			grpcCfg, store, mailSvc, agentReg, identityMgr,
			notificationHub,
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

		webServer, err := web.NewServer(webCfg, store)
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
