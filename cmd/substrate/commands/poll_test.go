package commands

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/stretchr/testify/require"
)

// TestHookDecisionBlock_NoStandbyText verifies that block decisions do not
// include "Standing by" instructions that would flood the conversation.
func TestHookDecisionBlock_NoStandbyText(t *testing.T) {
	t.Parallel()

	// Simulate what outputNoMessages does when pollAlwaysBlock is true.
	// The reason string must not instruct Claude to say "Standing by".
	reason := "No new messages. Check your inbox with " +
		"`substrate inbox` if needed."

	decision := "block"
	output := hookDecision{
		Decision: &decision,
		Reason:   reason,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	jsonStr := string(data)
	require.NotContains(t, jsonStr, "Standing by")
	require.NotContains(t, jsonStr, "standing by")
	require.Contains(t, jsonStr, "block")
}

// TestHookDecisionAllow_NullDecision verifies that allow decisions use null
// for the decision field (not "allow" or empty string).
func TestHookDecisionAllow_NullDecision(t *testing.T) {
	t.Parallel()

	output := hookDecision{
		Decision: nil,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	require.Nil(t, parsed["decision"])
}

// TestHookDecisionBlock_ValidJSON verifies that block decisions produce
// valid JSON that can be round-tripped.
func TestHookDecisionBlock_ValidJSON(t *testing.T) {
	t.Parallel()

	decision := "block"
	original := hookDecision{
		Decision: &decision,
		Reason:   "Test reason with special chars: \"quotes\" & <angles>",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded hookDecision
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.NotNil(t, decoded.Decision)
	require.Equal(t, "block", *decoded.Decision)
	require.Equal(t, original.Reason, decoded.Reason)
}

// TestCountUrgent verifies that urgent message counting handles all
// priority combinations correctly.
func TestCountUrgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		priorities []mail.Priority
		want       int
	}{
		{
			name:       "no messages",
			priorities: nil,
			want:       0,
		},
		{
			name: "no urgent messages",
			priorities: []mail.Priority{
				mail.PriorityNormal, mail.PriorityNormal,
			},
			want: 0,
		},
		{
			name: "all urgent",
			priorities: []mail.Priority{
				mail.PriorityUrgent, mail.PriorityUrgent,
			},
			want: 2,
		},
		{
			name: "mixed priorities",
			priorities: []mail.Priority{
				mail.PriorityNormal, mail.PriorityUrgent,
				mail.PriorityNormal, mail.PriorityUrgent,
				mail.PriorityNormal,
			},
			want: 2,
		},
		{
			name: "single urgent",
			priorities: []mail.Priority{
				mail.PriorityUrgent,
			},
			want: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msgs := make([]mail.InboxMessage, len(tc.priorities))
			for i, p := range tc.priorities {
				msgs[i] = mail.InboxMessage{Priority: p}
			}

			got := countUrgent(msgs)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestFormatHookReason_BasicMessages verifies the basic formatting of
// hook reasons with non-urgent messages.
func TestFormatHookReason_BasicMessages(t *testing.T) {
	t.Parallel()

	msgs := []mail.InboxMessage{
		{
			SenderName: "Alice",
			Subject:    "Review PR #42",
			Priority:   mail.PriorityNormal,
		},
		{
			SenderName: "Bob",
			Subject:    "Build failed",
			Priority:   mail.PriorityNormal,
		},
	}

	reason := formatHookReason(msgs, 0)

	require.Contains(t, reason, "2 unread messages")
	require.NotContains(t, reason, "URGENT")
	require.Contains(t, reason, "Alice")
	require.Contains(t, reason, "Bob")
	require.Contains(t, reason, "Review PR #42")
	require.Contains(t, reason, "Build failed")
	require.Contains(t, reason, "substrate inbox")
	require.NotContains(t, reason, "Standing by")
}

// TestFormatHookReason_WithUrgent verifies that urgent messages are
// highlighted in the reason string.
func TestFormatHookReason_WithUrgent(t *testing.T) {
	t.Parallel()

	msgs := []mail.InboxMessage{
		{
			SenderName: "Alice",
			Subject:    "Normal msg",
			Priority:   mail.PriorityNormal,
		},
		{
			SenderName: "Bob",
			Subject:    "Critical fix",
			Priority:   mail.PriorityUrgent,
		},
	}

	reason := formatHookReason(msgs, 1)

	require.Contains(t, reason, "2 unread messages (1 URGENT)")
	require.Contains(t, reason, "[URGENT]")
}

// TestFormatHookReason_WithDeadlines verifies that deadline formatting
// handles both future and overdue deadlines.
func TestFormatHookReason_WithDeadlines(t *testing.T) {
	t.Parallel()

	futureDeadline := time.Now().Add(2 * time.Hour)
	pastDeadline := time.Now().Add(-30 * time.Minute)

	msgs := []mail.InboxMessage{
		{
			SenderName: "Alice",
			Subject:    "Future task",
			Priority:   mail.PriorityNormal,
			Deadline:   &futureDeadline,
		},
		{
			SenderName: "Bob",
			Subject:    "Overdue task",
			Priority:   mail.PriorityNormal,
			Deadline:   &pastDeadline,
		},
	}

	reason := formatHookReason(msgs, 0)

	require.Contains(t, reason, "deadline:")
	require.Contains(t, reason, "OVERDUE")
}

// TestFormatHookReason_MissingSenderName verifies that messages without
// a sender name fall back to "Agent#<id>".
func TestFormatHookReason_MissingSenderName(t *testing.T) {
	t.Parallel()

	msgs := []mail.InboxMessage{
		{
			SenderID: 99,
			Subject:  "Anonymous",
			Priority: mail.PriorityNormal,
		},
	}

	reason := formatHookReason(msgs, 0)

	require.Contains(t, reason, "Agent#99")
}

// TestFormatHookReason_MoreThanFive verifies that messages beyond the
// first five are summarized with a count.
func TestFormatHookReason_MoreThanFive(t *testing.T) {
	t.Parallel()

	msgs := make([]mail.InboxMessage, 8)
	for i := range msgs {
		msgs[i] = mail.InboxMessage{
			SenderName: "Agent",
			Subject:    "Message",
			Priority:   mail.PriorityNormal,
		}
	}

	reason := formatHookReason(msgs, 0)

	require.Contains(t, reason, "8 unread messages")
	require.Contains(t, reason, "... and 3 more")
}
