package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// IdentityManager handles agent identity persistence for Claude Code sessions.
// It maintains mappings between Claude sessions and Subtrate agents, allowing
// agents to maintain identity across session restarts and compactions.
type IdentityManager struct {
	store       *db.Store
	registry    *Registry
	identityDir string
}

// IdentityFile represents the JSON structure stored in identity files.
type IdentityFile struct {
	SessionID       string           `json:"session_id"`
	AgentName       string           `json:"agent_name"`
	AgentID         int64            `json:"agent_id"`
	ProjectKey      string           `json:"project_key,omitempty"`
	GitBranch       string           `json:"git_branch,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	LastActiveAt    time.Time        `json:"last_active_at"`
	ConsumerOffsets map[string]int64 `json:"consumer_offsets,omitempty"`
}

// IdentityManagerOption is a functional option for configuring IdentityManager.
type IdentityManagerOption func(*identityManagerConfig)

// identityManagerConfig holds configuration options for IdentityManager.
type identityManagerConfig struct {
	identityDir string
}

// WithIdentityDir sets a custom identity directory for the manager. This is
// useful for testing with temporary directories.
func WithIdentityDir(dir string) IdentityManagerOption {
	return func(c *identityManagerConfig) {
		c.identityDir = dir
	}
}

// NewIdentityManager creates a new identity manager. By default it uses
// ~/.subtrate/identities for storage. Use WithIdentityDir to override.
func NewIdentityManager(store *db.Store, registry *Registry,
	opts ...IdentityManagerOption,
) (*IdentityManager, error) {
	// Apply default configuration.
	cfg := &identityManagerConfig{}

	// Apply options.
	for _, opt := range opts {
		opt(cfg)
	}

	// If no custom directory specified, use the default.
	if cfg.identityDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.identityDir = filepath.Join(home, ".subtrate", "identities")
	}

	// Ensure directory structure exists.
	dirs := []string{
		filepath.Join(cfg.identityDir, "by-session"),
		filepath.Join(cfg.identityDir, "by-project"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create dir %s: %w",
				dir, err)
		}
	}

	return &IdentityManager{
		store:       store,
		registry:    registry,
		identityDir: cfg.identityDir,
	}, nil
}

// EnsureIdentity ensures an agent identity exists for the given session. If
// no identity exists, it creates a new agent with a memorable name. If an
// existing identity is restored, the project_key and git_branch are updated
// to reflect the current session's context.
func (m *IdentityManager) EnsureIdentity(ctx context.Context, sessionID string,
	projectKey string, gitBranch string,
) (*IdentityFile, error) {
	// Try to restore existing identity.
	identity, err := m.RestoreIdentity(ctx, sessionID)
	if err == nil {
		// Update identity with current context.
		identity.LastActiveAt = time.Now()
		if err := m.updateAgentContext(
			ctx, identity, projectKey, gitBranch,
		); err != nil {
			return nil, err
		}
		if err := m.saveIdentityFile(identity); err != nil {
			return nil, err
		}
		if err := m.saveSessionDB(ctx, identity); err != nil {
			return nil, err
		}
		return identity, nil
	}

	// Try to find an identity for this project.
	if projectKey != "" {
		identity, err = m.GetProjectDefaultIdentity(ctx, projectKey)
		if err == nil {
			// Associate this session with the existing agent.
			identity.SessionID = sessionID
			identity.LastActiveAt = time.Now()
			if err := m.updateAgentContext(
				ctx, identity, projectKey, gitBranch,
			); err != nil {
				return nil, err
			}
			if err := m.saveIdentityFile(identity); err != nil {
				return nil, err
			}
			if err := m.saveSessionDB(ctx, identity); err != nil {
				return nil, err
			}
			return identity, nil
		}
	}

	// Create a new agent with a memorable name.
	name, err := m.registry.EnsureUniqueAgentName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent name: %w", err)
	}

	agent, err := m.registry.RegisterAgent(ctx, name, projectKey, gitBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	// Create identity file.
	identity = &IdentityFile{
		SessionID:       sessionID,
		AgentName:       agent.Name,
		AgentID:         agent.ID,
		ProjectKey:      projectKey,
		GitBranch:       gitBranch,
		CreatedAt:       time.Now(),
		LastActiveAt:    time.Now(),
		ConsumerOffsets: make(map[string]int64),
	}

	if err := m.saveIdentityFile(identity); err != nil {
		return nil, err
	}

	if err := m.saveSessionDB(ctx, identity); err != nil {
		return nil, err
	}

	// Set as project default if this is a new project.
	if projectKey != "" {
		if err := m.SetProjectDefault(ctx, projectKey, agent.Name); err != nil {
			// Non-fatal, log but continue.
			fmt.Printf("Warning: failed to set project default: %v\n",
				err)
		}
	}

	return identity, nil
}

// RestoreIdentity restores an agent identity from the session ID. Database
// operations are wrapped in a read transaction for consistent reads.
func (m *IdentityManager) RestoreIdentity(ctx context.Context,
	sessionID string,
) (*IdentityFile, error) {
	// First try file-based storage.
	filePath := filepath.Join(
		m.identityDir, "by-session", sessionID+".json",
	)

	data, err := os.ReadFile(filePath)
	if err == nil {
		var identity IdentityFile
		if err := json.Unmarshal(data, &identity); err != nil {
			return nil, fmt.Errorf("failed to parse identity: %w",
				err)
		}

		// Verify agent still exists in database.
		_, err = m.registry.GetAgent(ctx, identity.AgentID)
		if err != nil {
			return nil, fmt.Errorf("agent no longer exists: %w",
				err)
		}

		return &identity, nil
	}

	// Try database with read transaction for consistent reads.
	var identity *IdentityFile
	err = m.store.WithReadTx(ctx, func(ctx context.Context,
		q *sqlc.Queries,
	) error {
		session, err := q.GetSessionIdentity(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		agent, err := q.GetAgent(ctx, session.AgentID)
		if err != nil {
			return fmt.Errorf("agent not found: %w", err)
		}

		identity = &IdentityFile{
			SessionID:       sessionID,
			AgentName:       agent.Name,
			AgentID:         agent.ID,
			CreatedAt:       time.Unix(session.CreatedAt, 0),
			LastActiveAt:    time.Unix(session.LastActiveAt, 0),
			ConsumerOffsets: make(map[string]int64),
		}

		if session.ProjectKey.Valid {
			identity.ProjectKey = session.ProjectKey.String
		}
		if session.GitBranch.Valid {
			identity.GitBranch = session.GitBranch.String
		}

		// Load consumer offsets.
		offsets, err := q.ListConsumerOffsetsByAgent(ctx, agent.ID)
		if err == nil {
			for _, offset := range offsets {
				identity.ConsumerOffsets[offset.TopicName] = offset.LastOffset
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return identity, nil
}

// SaveIdentity persists the current identity state (including offsets). All
// database operations are wrapped in a transaction to ensure atomicity.
func (m *IdentityManager) SaveIdentity(ctx context.Context,
	identity *IdentityFile,
) error {
	identity.LastActiveAt = time.Now()

	if err := m.saveIdentityFile(identity); err != nil {
		return err
	}

	// Wrap all database operations in a transaction.
	return m.store.WithTx(ctx, func(ctx context.Context,
		q *sqlc.Queries,
	) error {
		// Save session identity.
		if identity.SessionID != "" {
			err := q.CreateSessionIdentity(
				ctx, sqlc.CreateSessionIdentityParams{
					SessionID: identity.SessionID,
					AgentID:   identity.AgentID,
					ProjectKey: sql.NullString{
						String: identity.ProjectKey,
						Valid:  identity.ProjectKey != "",
					},
					GitBranch: sql.NullString{
						String: identity.GitBranch,
						Valid:  identity.GitBranch != "",
					},
					CreatedAt:    identity.CreatedAt.Unix(),
					LastActiveAt: identity.LastActiveAt.Unix(),
				},
			)
			if err != nil {
				return fmt.Errorf("failed to save session: %w", err)
			}
		}

		// Save consumer offsets.
		for topicName, offset := range identity.ConsumerOffsets {
			topic, err := q.GetTopicByName(ctx, topicName)
			if err != nil {
				continue
			}

			err = q.UpsertConsumerOffset(
				ctx, sqlc.UpsertConsumerOffsetParams{
					AgentID:    identity.AgentID,
					TopicID:    topic.ID,
					LastOffset: offset,
					UpdatedAt:  time.Now().Unix(),
				},
			)
			if err != nil {
				return fmt.Errorf("failed to save offset: %w", err)
			}
		}

		return nil
	})
}

// GetProjectDefaultIdentity returns the default agent for a project. Database
// operations are wrapped in a read transaction for consistent reads.
func (m *IdentityManager) GetProjectDefaultIdentity(ctx context.Context,
	projectKey string,
) (*IdentityFile, error) {
	// Try file-based storage first.
	hash := hashProjectKey(projectKey)
	filePath := filepath.Join(
		m.identityDir, "by-project", hash+".json",
	)

	data, err := os.ReadFile(filePath)
	if err == nil {
		var identity IdentityFile
		if err := json.Unmarshal(data, &identity); err != nil {
			return nil, fmt.Errorf("failed to parse identity: %w",
				err)
		}

		// Verify agent still exists.
		_, err = m.registry.GetAgent(ctx, identity.AgentID)
		if err != nil {
			return nil, fmt.Errorf("agent no longer exists: %w",
				err)
		}

		return &identity, nil
	}

	// Try database with read transaction for consistent reads.
	var identity *IdentityFile
	err = m.store.WithReadTx(ctx, func(ctx context.Context,
		q *sqlc.Queries,
	) error {
		session, err := q.GetSessionIdentityByProject(
			ctx, sql.NullString{String: projectKey, Valid: true},
		)
		if err != nil {
			return fmt.Errorf("no default for project: %w", err)
		}

		agent, err := q.GetAgent(ctx, session.AgentID)
		if err != nil {
			return fmt.Errorf("agent not found: %w", err)
		}

		identity = &IdentityFile{
			SessionID:       session.SessionID,
			AgentName:       agent.Name,
			AgentID:         agent.ID,
			ProjectKey:      projectKey,
			CreatedAt:       time.Unix(session.CreatedAt, 0),
			LastActiveAt:    time.Unix(session.LastActiveAt, 0),
			ConsumerOffsets: make(map[string]int64),
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return identity, nil
}

// SetProjectDefault sets the default agent for a project.
func (m *IdentityManager) SetProjectDefault(ctx context.Context,
	projectKey string, agentName string,
) error {
	agent, err := m.registry.GetAgentByName(ctx, agentName)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	identity := &IdentityFile{
		AgentName:       agent.Name,
		AgentID:         agent.ID,
		ProjectKey:      projectKey,
		CreatedAt:       time.Now(),
		LastActiveAt:    time.Now(),
		ConsumerOffsets: make(map[string]int64),
	}

	// Save to project file.
	hash := hashProjectKey(projectKey)
	filePath := filepath.Join(
		m.identityDir, "by-project", hash+".json",
	)

	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}

	return nil
}

// CurrentIdentity returns the current agent identity for output.
func (m *IdentityManager) CurrentIdentity(ctx context.Context,
	sessionID string,
) (*IdentityFile, error) {
	return m.RestoreIdentity(ctx, sessionID)
}

// ListIdentities returns all known identities.
func (m *IdentityManager) ListIdentities(ctx context.Context) ([]IdentityFile,
	error,
) {
	var identities []IdentityFile

	// Read from by-session directory.
	sessionDir := filepath.Join(m.identityDir, "by-session")
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(sessionDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var identity IdentityFile
		if err := json.Unmarshal(data, &identity); err != nil {
			continue
		}

		identities = append(identities, identity)
	}

	return identities, nil
}

// saveIdentityFile writes an identity to the file system.
func (m *IdentityManager) saveIdentityFile(identity *IdentityFile) error {
	if identity.SessionID == "" {
		return nil
	}

	filePath := filepath.Join(
		m.identityDir, "by-session", identity.SessionID+".json",
	)

	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}

	return nil
}

// saveSessionDB saves the session identity to the database.
func (m *IdentityManager) saveSessionDB(ctx context.Context,
	identity *IdentityFile,
) error {
	if identity.SessionID == "" {
		return nil
	}

	return m.store.Queries().CreateSessionIdentity(
		ctx, sqlc.CreateSessionIdentityParams{
			SessionID: identity.SessionID,
			AgentID:   identity.AgentID,
			ProjectKey: sql.NullString{
				String: identity.ProjectKey,
				Valid:  identity.ProjectKey != "",
			},
			GitBranch: sql.NullString{
				String: identity.GitBranch,
				Valid:  identity.GitBranch != "",
			},
			CreatedAt:    identity.CreatedAt.Unix(),
			LastActiveAt: identity.LastActiveAt.Unix(),
		},
	)
}

// updateAgentContext updates the identity and agent record with the current
// project context. This ensures the agent's git_branch and project_key reflect
// the current session, which may change between sessions. It also updates
// discovery metadata (working_dir, hostname) for agent discovery.
func (m *IdentityManager) updateAgentContext(ctx context.Context,
	identity *IdentityFile, projectKey string, gitBranch string,
) error {
	// Update identity fields.
	if projectKey != "" {
		identity.ProjectKey = projectKey
	}
	if gitBranch != "" {
		identity.GitBranch = gitBranch
	}

	// Update agent record in database if we have context to update.
	if projectKey != "" || gitBranch != "" {
		err := m.store.Queries().UpdateAgentGitBranch(
			ctx, sqlc.UpdateAgentGitBranchParams{
				GitBranch: sql.NullString{
					String: identity.GitBranch,
					Valid:  identity.GitBranch != "",
				},
				ProjectKey: sql.NullString{
					String: identity.ProjectKey,
					Valid:  identity.ProjectKey != "",
				},
				LastActiveAt: time.Now().Unix(),
				ID:           identity.AgentID,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to update agent context: %w", err)
		}
	}

	// Update discovery metadata (working_dir and hostname). These are
	// set automatically so agents are discoverable without explicit
	// configuration.
	hostname, _ := os.Hostname()
	workingDir := projectKey // projectKey is the raw directory path.

	if workingDir != "" || hostname != "" {
		err := m.store.Queries().UpdateAgentDiscoveryInfo(
			ctx, sqlc.UpdateAgentDiscoveryInfoParams{
				WorkingDir:   workingDir,
				Hostname:     hostname,
				LastActiveAt: time.Now().Unix(),
				ID:           identity.AgentID,
			},
		)
		if err != nil {
			// Non-fatal: discovery info is supplementary.
			fmt.Fprintf(os.Stderr,
				"Warning: failed to update discovery info: %v\n",
				err,
			)
		}
	}

	return nil
}

// LoadCachedIdentity reads a cached identity file for the given session ID
// without verifying against the database. This is used in queue mode when
// the database is unavailable — the agent name from the cached file is used
// to construct queued operations that will be resolved at drain time.
func LoadCachedIdentity(sessionID string) (*IdentityFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	filePath := filepath.Join(
		home, ".subtrate", "identities",
		"by-session", sessionID+".json",
	)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read identity file: %w", err)
	}

	var identity IdentityFile
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, fmt.Errorf("parse identity file: %w", err)
	}

	return &identity, nil
}

// hashProjectKey creates a filesystem-safe hash of a project path.
func hashProjectKey(projectKey string) string {
	// Simple hash for filesystem safety.
	h := uint32(0)
	for _, c := range projectKey {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}
