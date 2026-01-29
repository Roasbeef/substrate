package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// SpawnConfig configures the agent spawning behavior.
type SpawnConfig struct {
	// CLIPath is the path to the claude CLI binary.
	CLIPath string

	// Model specifies which Claude model to use (e.g., "opus", "sonnet").
	Model string

	// WorkDir is the working directory for the spawned process.
	WorkDir string

	// SystemPrompt is the system prompt to use for the agent.
	SystemPrompt string

	// MaxTurns limits the number of conversation turns.
	MaxTurns int

	// PermissionMode controls tool execution permissions.
	PermissionMode claudeagent.PermissionMode

	// AllowDangerouslySkipPermissions enables bypassing permissions.
	AllowDangerouslySkipPermissions bool

	// NoSessionPersistence disables session saving (useful for testing).
	NoSessionPersistence bool

	// ConfigDir specifies a custom config directory for isolation (testing).
	// If set, the agent will use this directory instead of ~/.claude.
	ConfigDir string

	// Timeout is the maximum time to wait for a response.
	Timeout time.Duration
}

// DefaultSpawnConfig returns the default spawn configuration.
func DefaultSpawnConfig() *SpawnConfig {
	return &SpawnConfig{
		CLIPath: "claude",
		Model:   "claude-sonnet-4-5-20250929",
		Timeout: 5 * time.Minute,
	}
}

// SpawnResponse contains the response from a spawned agent.
type SpawnResponse struct {
	// Result contains the agent's text response.
	Result string

	// SessionID is the session ID for this conversation.
	SessionID string

	// CostUSD is the cost of this request in USD.
	CostUSD float64

	// DurationMS is the duration of the request in milliseconds.
	DurationMS int64

	// NumTurns is the number of conversation turns.
	NumTurns int

	// Error contains any error message.
	Error string

	// IsError indicates if this is an error result.
	IsError bool

	// Usage contains token usage information.
	Usage *claudeagent.NonNullableUsage
}

// Spawner manages spawning and interacting with Claude agents via the SDK.
type Spawner struct {
	cfg *SpawnConfig

	// Running processes tracked by session ID.
	processes   map[string]*SpawnedProcess
	processesMu sync.RWMutex
}

// SpawnedProcess represents a running or completed claude process.
type SpawnedProcess struct {
	SessionID string
	StartedAt time.Time
	EndedAt   *time.Time
	Prompt    string
	Response  *SpawnResponse
	Error     error
	client    *claudeagent.Client
}

// NewSpawner creates a new agent spawner.
func NewSpawner(cfg *SpawnConfig) *Spawner {
	if cfg == nil {
		cfg = DefaultSpawnConfig()
	}
	return &Spawner{
		cfg:       cfg,
		processes: make(map[string]*SpawnedProcess),
	}
}

// buildClientOptions constructs the SDK client options from config.
func (s *Spawner) buildClientOptions() []claudeagent.Option {
	opts := []claudeagent.Option{
		claudeagent.WithModel(s.cfg.Model),
	}

	if s.cfg.CLIPath != "" && s.cfg.CLIPath != "claude" {
		opts = append(opts, claudeagent.WithCLIPath(s.cfg.CLIPath))
	}

	if s.cfg.WorkDir != "" {
		opts = append(opts, claudeagent.WithCwd(s.cfg.WorkDir))
	}

	if s.cfg.SystemPrompt != "" {
		opts = append(opts, claudeagent.WithSystemPrompt(s.cfg.SystemPrompt))
	}

	if s.cfg.MaxTurns > 0 {
		opts = append(opts, claudeagent.WithMaxTurns(s.cfg.MaxTurns))
	}

	if s.cfg.PermissionMode != "" {
		opts = append(opts, claudeagent.WithPermissionMode(s.cfg.PermissionMode))
	}

	if s.cfg.AllowDangerouslySkipPermissions {
		opts = append(opts, claudeagent.WithAllowDangerouslySkipPermissions(true))
	}

	if s.cfg.NoSessionPersistence {
		opts = append(opts, claudeagent.WithNoSessionPersistence())
	}

	if s.cfg.ConfigDir != "" {
		opts = append(opts, claudeagent.WithConfigDir(s.cfg.ConfigDir))
	}

	return opts
}

// Spawn executes a prompt using the Claude SDK and returns the response.
func (s *Spawner) Spawn(ctx context.Context, prompt string) (*SpawnResponse, error) {
	opts := s.buildClientOptions()

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create claude client: %w", err)
	}
	defer client.Close()

	// Connect to the CLI subprocess.
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to claude CLI: %w", err)
	}

	// Collect the response from the query.
	var response SpawnResponse
	var lastAssistant claudeagent.AssistantMessage

	for msg := range client.Query(ctx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			lastAssistant = m
			response.SessionID = m.SessionID

		case claudeagent.ResultMessage:
			response.Result = m.Result
			response.SessionID = m.SessionID
			response.CostUSD = m.TotalCostUSD
			response.DurationMS = m.DurationMs
			response.NumTurns = m.NumTurns
			response.IsError = m.IsError
			response.Usage = m.Usage

			if m.IsError && len(m.Errors) > 0 {
				response.Error = m.Errors[0]
			}
		}
	}

	// If no result message, use the last assistant message content.
	if response.Result == "" && lastAssistant.MessageType() != "" {
		response.Result = lastAssistant.ContentText()
	}

	return &response, nil
}

// SpawnWithResume executes a prompt continuing an existing session.
func (s *Spawner) SpawnWithResume(
	ctx context.Context, sessionID, prompt string,
) (*SpawnResponse, error) {

	opts := s.buildClientOptions()
	opts = append(opts, claudeagent.WithResume(sessionID))

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create claude client: %w", err)
	}
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to claude CLI: %w", err)
	}

	var response SpawnResponse
	var lastAssistant claudeagent.AssistantMessage

	for msg := range client.Query(ctx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			lastAssistant = m
			response.SessionID = m.SessionID

		case claudeagent.ResultMessage:
			response.Result = m.Result
			response.SessionID = m.SessionID
			response.CostUSD = m.TotalCostUSD
			response.DurationMS = m.DurationMs
			response.NumTurns = m.NumTurns
			response.IsError = m.IsError
			response.Usage = m.Usage

			if m.IsError && len(m.Errors) > 0 {
				response.Error = m.Errors[0]
			}
		}
	}

	if response.Result == "" && lastAssistant.MessageType() != "" {
		response.Result = lastAssistant.ContentText()
	}

	return &response, nil
}

// SpawnAsync spawns an agent asynchronously and returns immediately.
func (s *Spawner) SpawnAsync(
	ctx context.Context, sessionID, prompt string,
) error {

	proc := &SpawnedProcess{
		SessionID: sessionID,
		StartedAt: time.Now(),
		Prompt:    prompt,
	}

	s.processesMu.Lock()
	s.processes[sessionID] = proc
	s.processesMu.Unlock()

	go func() {
		resp, err := s.Spawn(ctx, prompt)

		s.processesMu.Lock()
		defer s.processesMu.Unlock()

		proc.Response = resp
		proc.Error = err
		now := time.Now()
		proc.EndedAt = &now
	}()

	return nil
}

// GetProcess returns a spawned process by session ID.
func (s *Spawner) GetProcess(sessionID string) *SpawnedProcess {
	s.processesMu.RLock()
	defer s.processesMu.RUnlock()
	return s.processes[sessionID]
}

// ListProcesses returns all tracked processes.
func (s *Spawner) ListProcesses() []*SpawnedProcess {
	s.processesMu.RLock()
	defer s.processesMu.RUnlock()

	procs := make([]*SpawnedProcess, 0, len(s.processes))
	for _, p := range s.processes {
		procs = append(procs, p)
	}
	return procs
}

// SpawnWithHook spawns an agent and integrates with the heartbeat system.
func (s *Spawner) SpawnWithHook(
	ctx context.Context,
	heartbeatMgr *HeartbeatManager,
	agentID int64,
	prompt string,
) (*SpawnResponse, error) {

	// Record heartbeat at start.
	if heartbeatMgr != nil {
		heartbeatMgr.RecordHeartbeat(ctx, agentID)
	}

	// Spawn the agent.
	resp, err := s.Spawn(ctx, prompt)

	// Record heartbeat at end.
	if heartbeatMgr != nil {
		heartbeatMgr.RecordHeartbeat(ctx, agentID)
	}

	return resp, err
}

// StreamingSpawn spawns an agent and streams the response via callback.
func (s *Spawner) StreamingSpawn(
	ctx context.Context,
	prompt string,
	callback func(msg claudeagent.Message),
) (*SpawnResponse, error) {

	opts := s.buildClientOptions()

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create claude client: %w", err)
	}
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to claude CLI: %w", err)
	}

	var response SpawnResponse
	var lastAssistant claudeagent.AssistantMessage

	for msg := range client.Query(ctx, prompt) {
		// Call the callback for each message.
		if callback != nil {
			callback(msg)
		}

		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			lastAssistant = m
			response.SessionID = m.SessionID

		case claudeagent.ResultMessage:
			response.Result = m.Result
			response.SessionID = m.SessionID
			response.CostUSD = m.TotalCostUSD
			response.DurationMS = m.DurationMs
			response.NumTurns = m.NumTurns
			response.IsError = m.IsError
			response.Usage = m.Usage

			if m.IsError && len(m.Errors) > 0 {
				response.Error = m.Errors[0]
			}
		}
	}

	if response.Result == "" && lastAssistant.MessageType() != "" {
		response.Result = lastAssistant.ContentText()
	}

	return &response, nil
}

// SpawnInteractive creates a bidirectional stream for multi-turn conversations.
func (s *Spawner) SpawnInteractive(
	ctx context.Context,
) (*InteractiveSession, error) {

	opts := s.buildClientOptions()

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create claude client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to claude CLI: %w", err)
	}

	stream, err := client.Stream(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return &InteractiveSession{
		client: client,
		stream: stream,
		ctx:    ctx,
	}, nil
}

// InteractiveSession represents a multi-turn conversation session.
type InteractiveSession struct {
	client *claudeagent.Client
	stream *claudeagent.Stream
	ctx    context.Context
}

// Send submits a user message to the stream.
func (s *InteractiveSession) Send(prompt string) error {
	return s.stream.Send(s.ctx, prompt)
}

// Messages returns an iterator over response messages.
func (s *InteractiveSession) Messages() func(yield func(claudeagent.Message) bool) {
	return s.stream.Messages()
}

// SessionID returns the current session ID.
func (s *InteractiveSession) SessionID() string {
	return s.stream.SessionID()
}

// Interrupt sends an interrupt signal to stop the current generation.
func (s *InteractiveSession) Interrupt() error {
	return s.stream.Interrupt(s.ctx)
}

// Close terminates the session and cleans up resources.
func (s *InteractiveSession) Close() error {
	if s.stream != nil {
		s.stream.Close()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
