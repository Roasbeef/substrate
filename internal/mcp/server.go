package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// Server wraps the MCP server with mail service dependencies.
type Server struct {
	server   *mcp.Server
	dbStore  *db.Store
	storage  store.Storage
	mailSvc  *mail.Service
	mailRef  mail.MailActorRef // Optional actor ref for mail operations.
	registry *agent.Registry
}

// Config holds configuration for the MCP server.
type Config struct {
	// Store is the database store.
	Store *db.Store

	// MailActorRef is an optional actor reference for mail operations.
	// If set, mail operations will use the actor system.
	MailActorRef mail.MailActorRef
}

// NewServer creates a new MCP server with all mail tools registered.
func NewServer(dbStore *db.Store) *Server {
	return NewServerWithConfig(Config{Store: dbStore})
}

// NewServerWithConfig creates a new MCP server with the given configuration.
func NewServerWithConfig(cfg Config) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "subtrate",
		Version: "0.1.0",
	}, nil)

	// Create Storage wrapper from the db.Store.
	storage := store.FromDB(cfg.Store.DB())

	s := &Server{
		server:   mcpServer,
		dbStore:  cfg.Store,
		storage:  storage,
		mailSvc:  mail.NewServiceWithStore(storage),
		mailRef:  cfg.MailActorRef,
		registry: agent.NewRegistry(cfg.Store),
	}

	// Register all mail tools.
	s.registerTools()

	return s
}

// Run starts the MCP server on the given transport.
func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.server.Run(ctx, transport)
}

// registerTools registers all mail-related tools.
func (s *Server) registerTools() {
	// Core mail tools.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "send_mail",
		Description: "Send a message to one or more agents",
	}, s.handleSendMail)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "fetch_inbox",
		Description: "Fetch inbox messages for an agent",
	}, s.handleFetchInbox)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "read_message",
		Description: "Read a specific message and mark it as read",
	}, s.handleReadMessage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "ack_message",
		Description: "Acknowledge receipt of a message",
	}, s.handleAckMessage)

	// State transition tools.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "mark_read",
		Description: "Mark a message as read",
	}, s.handleMarkRead)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "star_message",
		Description: "Star or unstar a message for later reference",
	}, s.handleStarMessage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "snooze_message",
		Description: "Snooze a message until a specified time",
	}, s.handleSnoozeMessage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "archive_message",
		Description: "Archive a message",
	}, s.handleArchiveMessage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "trash_message",
		Description: "Move a message to trash",
	}, s.handleTrashMessage)

	// Pub/sub tools.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "subscribe",
		Description: "Subscribe to a topic to receive messages",
	}, s.handleSubscribe)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "unsubscribe",
		Description: "Unsubscribe from a topic",
	}, s.handleUnsubscribe)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_topics",
		Description: "List available topics",
	}, s.handleListTopics)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "publish",
		Description: "Publish a message to a topic",
	}, s.handlePublish)

	// Query tools.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search",
		Description: "Search messages using full-text search",
	}, s.handleSearch)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_status",
		Description: "Get mail status summary for an agent",
	}, s.handleGetStatus)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "poll_changes",
		Description: "Poll for new messages since last offset",
	}, s.handlePollChanges)

	// Agent management.
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "register_agent",
		Description: "Register a new agent with a name",
	}, s.handleRegisterAgent)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "whoami",
		Description: "Get the current agent identity",
	}, s.handleWhoAmI)
}
