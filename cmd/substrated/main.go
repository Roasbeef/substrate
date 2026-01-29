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
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mcp"
	"github.com/roasbeef/subtrate/internal/web"
)

func main() {
	var (
		dbPath  = flag.String("db", "~/.subtrate/subtrate.db", "Path to SQLite database")
		webAddr = flag.String("web", ":8080", "Web server address (empty to disable)")
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

	// Create the MCP server.
	mcpServer := mcp.NewServer(store)

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

	// Run the MCP server on stdio transport.
	log.Println("Starting substrated MCP server...")
	if err := mcpServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
