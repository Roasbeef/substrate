package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestClassifyEvent verifies the subject/body based event
// classification precedence.
func TestClassifyEvent(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		body     string
		priority string
		isPlan   bool
		want     string
	}{
		{
			name: "plan by linkage", subject: "anything",
			isPlan: true, want: eventKindPlan,
		},
		{
			name: "plan by prefix", subject: "[PLAN] Fix race",
			want: eventKindPlan,
		},
		{
			name: "diff by marker", subject: "changes",
			body: "intro\n" + diffMarker + "\ndiff --git a b",
			want: eventKindDiff,
		},
		{
			name: "diff by prefix", subject: "[Diff] branch",
			want: eventKindDiff,
		},
		{
			name: "review report", subject: "Review: approve",
			want: eventKindReview,
		},
		{
			name: "urgent is question", subject: "hmm",
			priority: "urgent", want: eventKindQuestion,
		},
		{
			name:    "interrogative subject",
			subject: "Should I rebase?", want: eventKindQuestion,
		},
		{
			name:    "decision marker",
			subject: "Decision needed: SSE vs WS",
			want:    eventKindQuestion,
		},
		{
			name: "status prefix", subject: "[Status] Agent",
			want: eventKindStatus,
		},
		{
			name: "plain message", subject: "FYI",
			want: eventKindMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEvent(
				tt.subject, tt.body, tt.priority, tt.isPlan,
			)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestParseWaitingFor verifies extraction of the status-update
// "Waiting for:" trailer.
func TestParseWaitingFor(t *testing.T) {
	waiting, body := parseWaitingFor(
		"Did the thing.\n\nWaiting for: Plan approval",
	)
	require.Equal(t, "Plan approval", waiting)
	require.Equal(t, "Did the thing.", body)

	// No trailer.
	waiting, body = parseWaitingFor("Just an update.")
	require.Empty(t, waiting)
	require.Equal(t, "Just an update.", body)

	// Mid-line mention is not a trailer.
	waiting, _ = parseWaitingFor("I am Waiting for: things to settle")
	require.Empty(t, waiting)
}

// TestWaitingNeedsHuman verifies benign waiting reasons are filtered.
func TestWaitingNeedsHuman(t *testing.T) {
	require.False(t, waitingNeedsHuman(""))
	require.False(t, waitingNeedsHuman("Nothing yet - mid implementation"))
	require.False(t, waitingNeedsHuman("None"))
	require.True(t, waitingNeedsHuman("Plan approval"))
	require.True(t, waitingNeedsHuman("Decision: SSE vs WebSocket"))
}

// TestEventNeedsAction verifies the actionable event predicate.
func TestEventNeedsAction(t *testing.T) {
	require.True(t, eventNeedsAction(
		eventKindPlan, "unread", "normal", "pending",
	))
	require.False(t, eventNeedsAction(
		eventKindPlan, "unread", "normal", "approved",
	))
	require.True(t, eventNeedsAction(
		eventKindQuestion, "unread", "normal", "",
	))
	require.False(t, eventNeedsAction(
		eventKindQuestion, "acked", "normal", "",
	))
	require.True(t, eventNeedsAction(
		eventKindMessage, "unread", "urgent", "",
	))
	require.False(t, eventNeedsAction(
		eventKindStatus, "unread", "normal", "",
	))
}

// TestBuildLaneCountsAndWaiting verifies lane aggregation of unread,
// needs-action counts, and waiting-for extraction ordering.
func TestBuildLaneCountsAndWaiting(t *testing.T) {
	now := time.Now()

	events := []CommandEvent{
		{
			State: "unread", NeedsAction: true,
			CreatedAt: now.UTC().Format(time.RFC3339),
		},
		{
			State: "read", WaitingFor: "Plan approval",
			CreatedAt: now.Add(-time.Hour).UTC().
				Format(time.RFC3339),
		},
		{
			State: "unread", WaitingFor: "old reason",
			CreatedAt: now.Add(-2 * time.Hour).UTC().
				Format(time.RFC3339),
		},
	}

	lane := buildLane(
		7, "TestAgent", "proj.git/x", "main", "testing",
		"active", "sess-1", now, events,
	)

	require.Equal(t, 2, lane.UnreadCount)
	require.Equal(t, 1, lane.NeedsAction)
	require.Equal(t, "Plan approval", lane.WaitingFor)
	require.Equal(t, events[0].CreatedAt, lane.LastEventAt)
	require.Equal(t, int64(7), lane.Agent.ID)
}

// TestLaneAttentionBlocked verifies a blocked marker is emitted only
// when there are no actionable events but the agent waits on a human.
func TestLaneAttentionBlocked(t *testing.T) {
	lane := CommandLane{
		Agent:      CommandLaneAgent{ID: 1, Name: "A"},
		WaitingFor: "Plan approval",
		Events:     []CommandEvent{},
	}
	items := laneAttention(lane)
	require.Len(t, items, 1)
	require.Equal(t, attentionKindBlocked, items[0].Kind)

	// With an actionable plan event, no extra blocked marker.
	lane.Events = []CommandEvent{{
		Kind: eventKindPlan, NeedsAction: true,
		Subject: "[PLAN] Fix", PlanReviewID: "uuid",
	}}
	items = laneAttention(lane)
	require.Len(t, items, 1)
	require.Equal(t, attentionKindPlan, items[0].Kind)
	require.Equal(t, "Fix", items[0].Title)
}

// TestSortLanesOrder verifies attention-first, then liveness ordering.
func TestSortLanesOrder(t *testing.T) {
	lanes := []CommandLane{
		{Agent: CommandLaneAgent{Name: "offline", Status: "offline"}},
		{Agent: CommandLaneAgent{Name: "idle", Status: "idle"}},
		{
			Agent:       CommandLaneAgent{Name: "needy", Status: "idle"},
			NeedsAction: 1,
		},
		{Agent: CommandLaneAgent{Name: "active", Status: "active"}},
	}

	sortLanes(lanes)

	require.Equal(t, "needy", lanes[0].Agent.Name)
	require.Equal(t, "active", lanes[1].Agent.Name)
	require.Equal(t, "idle", lanes[2].Agent.Name)
	require.Equal(t, "offline", lanes[3].Agent.Name)
}

// TestSortAttentionOrder verifies plan > question > urgent > blocked.
func TestSortAttentionOrder(t *testing.T) {
	items := []AttentionItem{
		{Kind: attentionKindBlocked},
		{Kind: attentionKindUrgent},
		{Kind: attentionKindPlan},
		{Kind: attentionKindQuestion},
	}

	sortAttention(items)

	require.Equal(t, attentionKindPlan, items[0].Kind)
	require.Equal(t, attentionKindQuestion, items[1].Kind)
	require.Equal(t, attentionKindUrgent, items[2].Kind)
	require.Equal(t, attentionKindBlocked, items[3].Kind)
}
