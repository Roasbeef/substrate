// Command subtrate-web runs the Subtrate web UI server standalone.
// This is useful for development and testing without the full MCP server.
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

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/web"
)

func main() {
	var (
		dbPath = flag.String("db", "~/.subtrate/subtrate.db", "Path to SQLite database")
		addr   = flag.String("addr", ":8080", "Web server address")
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

	// Ensure database directory exists.
	if err := os.MkdirAll(dbPathExpanded[:len(dbPathExpanded)-len("/subtrate.db")], 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Open the database with migrations.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sqliteStore, err := db.NewSqliteStore(&db.SqliteConfig{
		DatabaseFileName: dbPathExpanded,
	}, logger)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer sqliteStore.Close()

	// Create and start the web server.
	cfg := web.DefaultConfig()
	cfg.Addr = *addr

	server, err := web.NewServer(cfg, sqliteStore.Store)
	if err != nil {
		log.Fatalf("Failed to create web server: %v", err)
	}

	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		server.Shutdown(context.Background())
	}()

	log.Printf("Starting Subtrate web server on http://localhost%s", *addr)
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
