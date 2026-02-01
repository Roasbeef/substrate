package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	pollQuiet       bool
	pollWait        time.Duration
	pollAlwaysBlock bool
)

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll for new messages",
	Long: `Check for new messages since last poll.

For Claude Code hook integration, use --format hook to get JSON output
suitable for Stop/SubagentStop hooks:

  substrate poll --format hook              # Quick check, allow exit if no messages
  substrate poll --wait=55s --format hook   # Long-poll for 55s
  substrate poll --wait=55s --format hook --always-block  # Persistent agent pattern

The --always-block flag outputs {"decision": "block"} even when there are no
messages, keeping the agent alive indefinitely (useful for main agents).
Without it, {"decision": null} is output when no messages exist.`,
	RunE: runPoll,
}

func init() {
	pollCmd.Flags().BoolVar(&pollQuiet, "quiet", false,
		"Only output if there are new messages")
	pollCmd.Flags().DurationVar(&pollWait, "wait", 0,
		"Wait for messages (0 = no wait, e.g. 55s for hook timeout)")
	pollCmd.Flags().BoolVar(&pollAlwaysBlock, "always-block", false,
		"Always output block decision even with no messages (persistent agent)")
}

// hookDecision represents the JSON output for Claude Code Stop hooks.
type hookDecision struct {
	Decision *string `json:"decision"`         // "block" or null
	Reason   string  `json:"reason,omitempty"` // Explanation shown to Claude
}

func runPoll(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		// On error in hook mode with always-block, still block to keep alive.
		if outputFormat == "hook" && pollAlwaysBlock {
			return outputHookDecisionBlock(
				"Error connecting to Subtrate. Agent staying alive.",
			)
		}
		return err
	}
	defer client.Close()

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		// On error in hook mode with always-block, still block.
		if outputFormat == "hook" && pollAlwaysBlock {
			return outputHookDecisionBlock(
				"Error resolving agent identity. Agent staying alive.",
			)
		}
		return err
	}

	// Send heartbeat to indicate agent activity.
	_ = client.UpdateHeartbeat(ctx, agentID)

	// Long-polling loop.
	deadline := time.Now().Add(pollWait)
	pollInterval := 5 * time.Second

	for {
		// PollChanges with empty offsets to get all unread messages.
		newMessages, _, err := client.PollChanges(ctx, agentID, nil)
		if err != nil {
			// On error in hook mode with always-block, still block.
			if outputFormat == "hook" && pollAlwaysBlock {
				return outputHookDecisionBlock(
					"Error checking mail. Agent staying alive.",
				)
			}
			return err
		}

		// If we have messages, output them and return.
		if len(newMessages) > 0 {
			return outputPollMessages(newMessages, agentNameStr)
		}

		// No messages - check if we should continue polling.
		if pollWait == 0 || time.Now().After(deadline) {
			// Time's up or no wait requested.
			return outputNoMessages(agentNameStr)
		}

		// Wait before next poll, but don't overshoot deadline.
		sleepDuration := pollInterval
		remaining := time.Until(deadline)
		if remaining < sleepDuration {
			sleepDuration = remaining
		}
		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
	}
}

// outputPollMessages outputs messages based on format.
func outputPollMessages(msgs []mail.InboxMessage, agentName string) error {
	switch outputFormat {
	case "hook":
		return outputHookDecisionWithMessages(msgs)
	case "json":
		return outputJSON(map[string]any{
			"new_messages": msgs,
		})
	case "context":
		fmt.Print(formatContext(msgs))
		return nil
	default:
		fmt.Printf("New messages for %s:\n\n", agentName)
		for _, msg := range msgs {
			fmt.Print(formatMessage(msg))
			fmt.Println()
		}
	}
	return nil
}

// outputNoMessages handles output when there are no messages.
func outputNoMessages(agentName string) error {
	switch outputFormat {
	case "hook":
		if pollAlwaysBlock {
			// Persistent agent pattern: always block to stay alive.
			// Instruct Claude to output text so it completes a response,
			// which triggers the Stop hook again, creating a polling loop.
			return outputHookDecisionBlock(
				"No new messages. Say 'Standing by for messages.' and wait for the next check.",
			)
		}
		// Allow exit when no messages.
		return outputHookDecisionAllow()
	case "json":
		return outputJSON(map[string]any{
			"new_messages": []any{},
		})
	case "context":
		// Context mode: quiet when no messages.
		return nil
	default:
		if !pollQuiet {
			fmt.Printf("No new messages for %s.\n", agentName)
		}
	}
	return nil
}

// outputHookDecisionBlock outputs a block decision for Claude Code hooks.
func outputHookDecisionBlock(reason string) error {
	decision := "block"
	output := hookDecision{
		Decision: &decision,
		Reason:   reason,
	}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// outputHookDecisionAllow outputs an allow decision (null) for Claude Code hooks.
func outputHookDecisionAllow() error {
	output := hookDecision{
		Decision: nil,
	}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// outputHookDecisionWithMessages formats messages for hook output.
func outputHookDecisionWithMessages(msgs []mail.InboxMessage) error {
	urgentCount := countUrgent(msgs)
	reason := formatHookReason(msgs, urgentCount)

	decision := "block"
	output := hookDecision{
		Decision: &decision,
		Reason:   reason,
	}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// countUrgent counts urgent messages.
func countUrgent(msgs []mail.InboxMessage) int {
	count := 0
	for _, m := range msgs {
		if m.Priority == mail.PriorityUrgent {
			count++
		}
	}
	return count
}

// formatHookReason formats the reason string for hook output.
func formatHookReason(msgs []mail.InboxMessage, urgentCount int) string {
	var sb strings.Builder

	if urgentCount > 0 {
		sb.WriteString(fmt.Sprintf(
			"You have %d unread messages (%d URGENT):\n",
			len(msgs), urgentCount,
		))
	} else {
		sb.WriteString(fmt.Sprintf(
			"You have %d unread messages:\n",
			len(msgs),
		))
	}

	// List up to 5 messages.
	limit := min(5, len(msgs))

	for i := 0; i < limit; i++ {
		msg := msgs[i]
		sb.WriteString("- ")
		if msg.Priority == mail.PriorityUrgent {
			sb.WriteString("[URGENT] ")
		}
		senderDisplay := msg.SenderName
		if senderDisplay == "" {
			senderDisplay = fmt.Sprintf("Agent#%d", msg.SenderID)
		}
		sb.WriteString(fmt.Sprintf("From: %s - %q", senderDisplay, msg.Subject))
		if msg.Deadline != nil {
			remaining := time.Until(*msg.Deadline)
			if remaining > 0 {
				sb.WriteString(fmt.Sprintf(" (deadline: %s)",
					formatDuration(remaining)))
			} else {
				sb.WriteString(" (OVERDUE)")
			}
		}
		sb.WriteString("\n")
	}

	if len(msgs) > 5 {
		sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(msgs)-5))
	}

	sb.WriteString("\nUse `substrate inbox` to see all messages.")

	return sb.String()
}
