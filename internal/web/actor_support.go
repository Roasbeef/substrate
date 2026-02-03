package web

import (
	"context"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/mail"
)

// getAgentStatus gets an agent's mail status via the shared mail client.
func (s *Server) getAgentStatus(
	ctx context.Context, agentID int64,
) (mail.GetStatusResponse, error) {
	return s.mailClient.GetAgentStatus(ctx, agentID)
}

// listRecentActivities lists recent activities via the shared activity client.
func (s *Server) listRecentActivities(
	ctx context.Context, limit int,
) ([]activity.Activity, error) {
	return s.actClient.ListRecentActivities(ctx, limit)
}

// countsToMap converts agent status counts to a map for JSON serialization.
func countsToMap(counts *agent.StatusCounts) map[string]int {
	if counts == nil {
		return map[string]int{}
	}
	return map[string]int{
		"active":  counts.Active,
		"busy":    counts.Busy,
		"idle":    counts.Idle,
		"offline": counts.Offline,
	}
}
