package commands

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComputeAgentStatus verifies status derivation from elapsed time and
// session state across all threshold boundaries.
func TestComputeAgentStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		elapsed    time.Duration
		hasSession bool
		want       string
	}{
		{
			name:       "busy with active session under 5 min",
			elapsed:    2 * time.Minute,
			hasSession: true,
			want:       "busy",
		},
		{
			name:       "active without session under 5 min",
			elapsed:    2 * time.Minute,
			hasSession: false,
			want:       "active",
		},
		{
			name:       "idle between 5 and 30 min",
			elapsed:    15 * time.Minute,
			hasSession: false,
			want:       "idle",
		},
		{
			name:       "offline after 30 min",
			elapsed:    45 * time.Minute,
			hasSession: false,
			want:       "offline",
		},
		{
			name:       "boundary: exactly 5 min without session",
			elapsed:    5 * time.Minute,
			hasSession: false,
			want:       "idle",
		},
		{
			name:       "boundary: exactly 5 min with session",
			elapsed:    5 * time.Minute,
			hasSession: true,
			want:       "idle",
		},
		{
			name:       "boundary: exactly 30 min",
			elapsed:    30 * time.Minute,
			hasSession: false,
			want:       "offline",
		},
		{
			name:       "boundary: just under 5 min",
			elapsed:    4*time.Minute + 59*time.Second,
			hasSession: false,
			want:       "active",
		},
		{
			name:       "boundary: just under 30 min",
			elapsed:    29*time.Minute + 59*time.Second,
			hasSession: false,
			want:       "idle",
		},
		{
			name:       "zero duration without session",
			elapsed:    0,
			hasSession: false,
			want:       "active",
		},
		{
			name:       "zero duration with session",
			elapsed:    0,
			hasSession: true,
			want:       "busy",
		},
		{
			name:       "offline with session still set",
			elapsed:    1 * time.Hour,
			hasSession: true,
			want:       "offline",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := computeAgentStatus(tc.elapsed, tc.hasSession)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestFilterDiscoveredAgents verifies client-side filtering by status,
// project prefix, and agent name substring. Subtests run sequentially
// because filterDiscoveredAgents reads package-level flag vars.
func TestFilterDiscoveredAgents(t *testing.T) {
	agents := []DiscoveredAgentInfo{
		{
			Name:       "AlphaAgent",
			Status:     "active",
			ProjectKey: "subtrate",
		},
		{
			Name:       "BetaAgent",
			Status:     "busy",
			ProjectKey: "subtrate",
		},
		{
			Name:       "GammaAgent",
			Status:     "idle",
			ProjectKey: "lnd",
		},
		{
			Name:       "DeltaAgent",
			Status:     "offline",
			ProjectKey: "lnd-next",
		},
	}

	tests := []struct {
		name    string
		status  string
		project string
		agName  string
		want    []string
	}{
		{
			name: "no filters returns all",
			want: []string{
				"AlphaAgent", "BetaAgent",
				"GammaAgent", "DeltaAgent",
			},
		},
		{
			name:   "filter by single status",
			status: "active",
			want:   []string{"AlphaAgent"},
		},
		{
			name:   "filter by multiple statuses",
			status: "active,busy",
			want:   []string{"AlphaAgent", "BetaAgent"},
		},
		{
			name:    "filter by project prefix",
			project: "lnd",
			want:    []string{"GammaAgent", "DeltaAgent"},
		},
		{
			name:    "filter by exact project",
			project: "subtrate",
			want:    []string{"AlphaAgent", "BetaAgent"},
		},
		{
			name:   "filter by name substring case insensitive",
			agName: "alpha",
			want:   []string{"AlphaAgent"},
		},
		{
			name:   "filter by name partial match",
			agName: "Agent",
			want: []string{
				"AlphaAgent", "BetaAgent",
				"GammaAgent", "DeltaAgent",
			},
		},
		{
			name:    "combined filters",
			status:  "active,busy",
			project: "subtrate",
			want:    []string{"AlphaAgent", "BetaAgent"},
		},
		{
			name:    "combined filters with name narrows further",
			status:  "active,busy",
			project: "subtrate",
			agName:  "beta",
			want:    []string{"BetaAgent"},
		},
		{
			name:   "no matches returns empty",
			status: "active",
			agName: "nonexistent",
			want:   []string{},
		},
		{
			name:   "status with spaces trimmed",
			status: " active , busy ",
			want:   []string{"AlphaAgent", "BetaAgent"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set package-level filter vars for this test.
			// These are used by filterDiscoveredAgents.
			discoverStatus = tc.status
			discoverProject = tc.project
			discoverName = tc.agName

			// Copy input to avoid mutation.
			input := make([]DiscoveredAgentInfo, len(agents))
			copy(input, agents)

			got := filterDiscoveredAgents(input)

			names := make([]string, len(got))
			for i, a := range got {
				names[i] = a.Name
			}
			require.Equal(t, tc.want, names)
		})
	}
}
