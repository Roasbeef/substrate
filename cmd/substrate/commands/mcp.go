package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/mcp"
)

var (
	// mcpTransport is the MCP transport type.
	mcpTransport string

	// mcpAddr is the address for HTTP-based MCP transports.
	mcpAddr string
)

// mcpCmd is the parent command for MCP operations.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server operations",
	Long: `Manage the MCP (Model Context Protocol) server.

The MCP server exposes Subtrate's mail, agent, and topic operations
as MCP tools that can be consumed by AI agents and Claude Code.`,
}

// mcpServeCmd starts the MCP server.
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an MCP server that proxies to the substrated daemon",
	Long: `Start an MCP server that connects to the running substrated daemon
via gRPC and exposes its operations as MCP tools.

Supports three transport modes:
  streamable-http  Modern HTTP-based transport (default)
  sse              Server-Sent Events transport (legacy)
  stdio            Standard I/O transport (for subprocess invocation)

The daemon must be running for this command to work.

Examples:
  substrate mcp serve                              # Streamable HTTP on :8090
  substrate mcp serve --transport sse --addr :9090  # SSE on :9090
  substrate mcp serve --transport stdio             # Stdio transport`,
	RunE: runMCPServe,
}

func init() {
	mcpServeCmd.Flags().StringVar(
		&mcpTransport, "transport", "streamable-http",
		"MCP transport: streamable-http, sse, or stdio",
	)
	mcpServeCmd.Flags().StringVar(
		&mcpAddr, "addr", "127.0.0.1:8090",
		"Listen address for HTTP transports (default: localhost only)",
	)

	mcpCmd.AddCommand(mcpServeCmd)
}

// runMCPServe connects to the daemon and starts an MCP server.
func runMCPServe(cmd *cobra.Command, args []string) error {
	// Validate transport flag.
	validTransports := []string{
		"streamable-http", "sse", "stdio",
	}
	if err := validateEnum(
		mcpTransport, "transport", validTransports,
	); err != nil {
		return err
	}

	// Determine gRPC address.
	addr := grpcAddr
	if addr == "" {
		addr = defaultGRPCAddr
	}

	// Connect to the daemon via gRPC.
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer conn.Close()

	// Verify connectivity with a quick health check.
	verifyCtx, verifyCancel := context.WithTimeout(
		context.Background(), grpcConnectTimeout,
	)
	defer verifyCancel()

	agentClient := subtraterpc.NewAgentClient(conn)
	if _, err := agentClient.ListAgents(
		verifyCtx, &subtraterpc.ListAgentsRequest{},
	); err != nil {
		return fmt.Errorf(
			"daemon not responding at %s: %w "+
				"(start with: make run)", addr, err,
		)
	}

	// Create MCP server with gRPC backend.
	backend := mcp.NewGRPCBackend(conn)
	mcpServer := mcp.NewServerWithBackend(backend)

	// Set up signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Start the MCP server on the selected transport.
	switch mcpTransport {
	case "stdio":
		fmt.Fprintf(os.Stderr,
			"MCP server starting (stdio, daemon=%s)\n", addr,
		)
		return mcpServer.Run(ctx, &sdkmcp.StdioTransport{})

	case "streamable-http":
		return serveStreamableHTTP(ctx, mcpServer, addr)

	case "sse":
		return serveSSE(ctx, mcpServer, addr)

	default:
		return fmt.Errorf("unknown transport: %s", mcpTransport)
	}
}

// serveStreamableHTTP starts the MCP server with streamable HTTP transport.
func serveStreamableHTTP(
	ctx context.Context, mcpServer *mcp.Server, daemonAddr string,
) error {
	handler := sdkmcp.NewStreamableHTTPHandler(
		func(r *http.Request) *sdkmcp.Server {
			return mcpServer.MCPServer()
		}, nil,
	)

	httpServer := &http.Server{
		Addr:              mcpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Shut down gracefully when context is cancelled.
	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	fmt.Fprintf(os.Stderr,
		"MCP server listening on %s (streamable-http, daemon=%s)\n",
		mcpAddr, daemonAddr,
	)

	if err := httpServer.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// serveSSE starts the MCP server with SSE transport.
func serveSSE(
	ctx context.Context, mcpServer *mcp.Server, daemonAddr string,
) error {
	handler := sdkmcp.NewSSEHandler(
		func(r *http.Request) *sdkmcp.Server {
			return mcpServer.MCPServer()
		}, nil,
	)

	httpServer := &http.Server{
		Addr:              mcpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Shut down gracefully when context is cancelled.
	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("SSE server shutdown error: %v", err)
		}
	}()

	fmt.Fprintf(os.Stderr,
		"MCP server listening on %s (sse, daemon=%s)\n",
		mcpAddr, daemonAddr,
	)

	if err := httpServer.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		return fmt.Errorf("SSE server error: %w", err)
	}

	return nil
}
