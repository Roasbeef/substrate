package agent

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// Registry manages agent registration and lookup.
type Registry struct {
	store *db.Store
}

// NewRegistry creates a new agent registry.
func NewRegistry(store *db.Store) *Registry {
	return &Registry{store: store}
}

// RegisterAgent creates a new agent with the given name.
func (r *Registry) RegisterAgent(ctx context.Context, name string,
	projectKey string, gitBranch string) (*sqlc.Agent, error) {

	now := time.Now().Unix()

	agent, err := r.store.Queries().CreateAgent(
		ctx, sqlc.CreateAgentParams{
			Name: name,
			ProjectKey: sql.NullString{
				String: projectKey,
				Valid:  projectKey != "",
			},
			GitBranch: sql.NullString{
				String: gitBranch,
				Valid:  gitBranch != "",
			},
			CreatedAt:    now,
			LastActiveAt: now,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Create the agent's inbox topic.
	_, err = r.store.Queries().GetOrCreateAgentInboxTopic(
		ctx, sqlc.GetOrCreateAgentInboxTopicParams{
			Column1: sql.NullString{
				String: name,
				Valid:  true,
			},
			CreatedAt: now,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create inbox topic: %w", err)
	}

	return &agent, nil
}

// GetAgent retrieves an agent by ID.
func (r *Registry) GetAgent(ctx context.Context,
	id int64) (*sqlc.Agent, error) {

	agent, err := r.store.Queries().GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return &agent, nil
}

// GetAgentByName retrieves an agent by name.
func (r *Registry) GetAgentByName(ctx context.Context,
	name string) (*sqlc.Agent, error) {

	agent, err := r.store.Queries().GetAgentByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return &agent, nil
}

// ListAgents returns all registered agents.
func (r *Registry) ListAgents(ctx context.Context) ([]sqlc.Agent, error) {
	return r.store.Queries().ListAgents(ctx)
}

// ListAgentsByProject returns agents for a specific project.
func (r *Registry) ListAgentsByProject(ctx context.Context,
	projectKey string) ([]sqlc.Agent, error) {

	return r.store.Queries().ListAgentsByProject(ctx, sql.NullString{
		String: projectKey,
		Valid:  true,
	})
}

// UpdateLastActive updates the agent's last active timestamp.
func (r *Registry) UpdateLastActive(ctx context.Context, id int64) error {
	return r.store.Queries().UpdateAgentLastActive(
		ctx, sqlc.UpdateAgentLastActiveParams{
			LastActiveAt: time.Now().Unix(),
			ID:           id,
		},
	)
}

// GenerateMemoableName generates a unique, memorable agent name.
// Uses adjective + noun combination for easy recall.
func GenerateMemoableName() string {
	adjectives := []string{
		"Swift", "Bright", "Silent", "Bold", "Clever",
		"Golden", "Silver", "Crystal", "Azure", "Crimson",
		"Emerald", "Amber", "Violet", "Coral", "Jade",
		"Onyx", "Ruby", "Sapphire", "Pearl", "Opal",
		"Mystic", "Noble", "Radiant", "Serene", "Vivid",
	}

	nouns := []string{
		"Castle", "Tower", "Haven", "Grove", "Peak",
		"Valley", "River", "Ocean", "Forest", "Garden",
		"Phoenix", "Dragon", "Griffin", "Falcon", "Raven",
		"Wolf", "Bear", "Tiger", "Lion", "Eagle",
		"Star", "Moon", "Sun", "Comet", "Nova",
	}

	// Use UUID to seed selection for uniqueness.
	id := uuid.New()
	bytes := id[:]

	adjIdx := int(bytes[0]) % len(adjectives)
	nounIdx := int(bytes[1]) % len(nouns)

	return adjectives[adjIdx] + nouns[nounIdx]
}

// EnsureUniqueAgentName generates a unique agent name, adding a suffix if
// needed.
func (r *Registry) EnsureUniqueAgentName(ctx context.Context) (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		name := GenerateMemoableName()

		_, err := r.store.Queries().GetAgentByName(ctx, name)
		if err != nil {
			// Name not found, it's unique.
			if err == sql.ErrNoRows {
				return name, nil
			}
			return "", fmt.Errorf("failed to check name: %w", err)
		}
	}

	// Fall back to UUID suffix after 10 attempts.
	id := uuid.New()
	return fmt.Sprintf("Agent-%s", id.String()[:8]), nil
}
