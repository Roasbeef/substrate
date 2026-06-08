package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	watchTimeout   time.Duration
	watchHeartbeat time.Duration
	watchCheck     bool
	watchEvents    string
	watchMaxMsgs   int
)

// watchPollInterval is how often the watcher checks for new messages
// between heartbeats. Kept as a variable so tests can shorten it.
var watchPollInterval = 5 * time.Second

// watchRetryInterval is how long the watcher sleeps after a failed poll
// (e.g. server down) before retrying. The watcher never exits on
// transient errors; it parks and retries so the agent is not woken with
// useless error churn.
var watchRetryInterval = 30 * time.Second

// watchBodyLimit caps per-message body length in the wake digest. The
// digest is meant to carry enough payload that the agent usually does
// not need a follow-up inbox round-trip, without flooding its context.
const watchBodyLimit = 2000

// ErrWatcherArmed is returned when another live watcher already holds
// the lease for this agent.
var ErrWatcherArmed = &CLIError{
	Code:    ExitConflict,
	Message: "another watcher is already armed for this agent",
}

// ExitInterrupted is the exit code when the watcher is killed by a
// signal. It follows the 128+SIGINT convention so a deliberate kill is
// distinguishable from a wake (0) and from fatal errors (1) — the
// agent must not treat an interrupted watcher as a wake and re-arm.
const ExitInterrupted = 130

// errWatchInterrupted is returned when SIGINT/SIGTERM cancels the
// watcher. The message lands on stderr (not stdout), so no digest is
// emitted and the wake notification carries an explicit do-not-re-arm
// signal instead of empty output.
var errWatchInterrupted = &CLIError{
	Code:    ExitInterrupted,
	Message: "watch interrupted by signal; not a wake, do not re-arm",
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Block until there is work, then exit with a digest",
	Long: `Park until a mail event arrives, print a digest, and exit.

This command implements the background-watcher persistence pattern: the
agent runs it as a background task (Bash with run_in_background), ends
its turn normally, and is re-invoked by Claude Code when the watcher
exits. The watcher's stdout carries the new messages plus a re-arm
instruction.

Behaviors:
  - Self-draining: if unread mail already exists on startup, the
    watcher exits immediately with the backlog.
  - Lease: only one watcher per agent. A second invocation exits with
    code 5 (conflict) without disturbing the first.
  - Heartbeats: sends liveness heartbeats while parked, so agent
    status stays accurate without any hook churn.
  - Server-down: retries with backoff internally rather than exiting.

Exit codes:
  0    woke with a digest on stdout (or --timeout expired)
  1    fatal error (identity resolution, lease I/O)
  5    another watcher is already armed for this agent
  130  interrupted by signal (not a wake; do not re-arm)

Use --check to test the lease without arming: exit 0 if a live watcher
is armed, exit 1 otherwise (for hook scripts).`,
	Example: `  substrate watch --session-id "$CLAUDE_SESSION_ID"
  substrate watch --session-id "$CLAUDE_SESSION_ID" --timeout 4h
  substrate watch --session-id "$CLAUDE_SESSION_ID" --check`,
	// Errors are semantic (lease conflict, not-armed); usage spam would
	// only pollute the agent's context.
	SilenceUsage: true,
	RunE:         runWatch,
}

func init() {
	watchCmd.Flags().DurationVar(&watchTimeout, "timeout", 0,
		"Exit after this long with no events (0 = wait forever)")
	watchCmd.Flags().DurationVar(&watchHeartbeat, "heartbeat",
		30*time.Second, "Heartbeat interval while parked")
	watchCmd.Flags().BoolVar(&watchCheck, "check", false,
		"Check whether a watcher is armed (exit 0) without arming")
	watchCmd.Flags().StringVar(&watchEvents, "events", "mail",
		"Comma-separated event types to wake on (v1: mail only)")
	watchCmd.Flags().IntVar(&watchMaxMsgs, "max-messages", 10,
		"Maximum messages included in the wake digest")
}

// watchLockDir returns the directory holding watcher lease files,
// creating it if needed.
func watchLockDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home dir: %w", err)
	}

	dir := filepath.Join(home, ".subtrate", "watch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create lock dir: %w", err)
	}

	return dir, nil
}

// watchLockPath returns the lease file path for an agent.
func watchLockPath(agentID int64) (string, error) {
	dir, err := watchLockDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("agent-%d.lock", agentID)), nil
}

// pidAlive reports whether a process with the given PID is running. On
// Unix, signal 0 probes existence without delivering a signal.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}

	// EPERM means the process exists but belongs to another user.
	return errors.Is(err, syscall.EPERM)
}

// readLeasePID reads the PID stored in a lease file. Returns 0 if the
// file does not exist or is malformed.
func readLeasePID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}

	return pid
}

// watcherArmed reports whether a live watcher process holds the lease
// for the given agent.
func watcherArmed(agentID int64) (bool, error) {
	path, err := watchLockPath(agentID)
	if err != nil {
		return false, err
	}

	return pidAlive(readLeasePID(path)), nil
}

// watchWatermarkPath returns the digest watermark file path for an
// agent. The watermark records the highest message ID already emitted
// in a wake digest, so a re-armed watcher does not re-wake on the same
// unread backlog (the agent may act on a digest without marking the
// messages read, e.g. replying via send).
func watchWatermarkPath(agentID int64) (string, error) {
	dir, err := watchLockDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(
		dir, fmt.Sprintf("agent-%d.watermark", agentID),
	), nil
}

// readWatchWatermark returns the stored watermark for an agent, or 0
// if missing or malformed.
func readWatchWatermark(agentID int64) int64 {
	path, err := watchWatermarkPath(agentID)
	if err != nil {
		return 0
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	id, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	return id
}

// writeWatchWatermark persists the highest digested message ID for an
// agent. Best effort: a failed write only risks one duplicate wake.
func writeWatchWatermark(agentID, id int64) {
	path, err := watchWatermarkPath(agentID)
	if err != nil {
		return
	}

	_ = os.WriteFile(
		path, []byte(strconv.FormatInt(id, 10)), 0o644,
	)
}

// filterFreshMessages returns the messages with IDs above the
// watermark, plus the highest ID seen across the fresh set (0 if
// none). Messages at or below the watermark were already delivered in
// a previous wake digest and must not re-trigger a wake.
func filterFreshMessages(
	msgs []mail.InboxMessage, watermark int64,
) ([]mail.InboxMessage, int64) {
	var fresh []mail.InboxMessage
	maxID := int64(0)

	for _, msg := range msgs {
		if msg.ID <= watermark {
			continue
		}

		fresh = append(fresh, msg)
		if msg.ID > maxID {
			maxID = msg.ID
		}
	}

	return fresh, maxID
}

// acquireWatchLease claims the watcher lease for an agent by writing
// this process's PID. Stale leases (dead PID) are reclaimed. Returns a
// release function, or ErrWatcherArmed if a live watcher exists.
func acquireWatchLease(agentID int64) (func(), error) {
	path, err := watchLockPath(agentID)
	if err != nil {
		return nil, err
	}

	if pidAlive(readLeasePID(path)) {
		return nil, ErrWatcherArmed
	}

	pid := os.Getpid()
	err = os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to write lease: %w", err)
	}

	release := func() {
		// Only remove the lease if we still own it; a successor
		// may have reclaimed a lease we left stale.
		if readLeasePID(path) == pid {
			_ = os.Remove(path)
		}
	}

	return release, nil
}

// runWatch implements the watch command. It arms the lease, parks until
// a mail event (or timeout), prints a wake digest, and exits.
func runWatch(cmd *cobra.Command, args []string) error {
	// Cancel cleanly on SIGINT/SIGTERM so the lease is released when
	// the harness or user kills the background task.
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// --check: report lease state without arming. Exit 0 if armed,
	// exit 1 (via error) otherwise, so hook scripts can branch on it.
	if watchCheck {
		armed, err := watcherArmed(agentID)
		if err != nil {
			return err
		}

		if armed {
			fmt.Printf("armed (agent %s)\n", agentNameStr)
			return nil
		}

		return fmt.Errorf("not armed (agent %s)", agentNameStr)
	}

	release, err := acquireWatchLease(agentID)
	if err != nil {
		return err
	}
	defer release()

	// Initial heartbeat marks the agent active immediately.
	_ = client.UpdateHeartbeat(ctx, agentID)

	var deadline time.Time
	if watchTimeout > 0 {
		deadline = time.Now().Add(watchTimeout)
	}

	lastHeartbeat := time.Now()
	watermark := readWatchWatermark(agentID)

	for {
		// Self-draining check: unread mail newer than the digest
		// watermark ends the park immediately, including backlog
		// that arrived while no watcher was armed. Messages at or
		// below the watermark were already delivered in a prior
		// wake digest, so they do not re-wake the agent — without
		// this, an agent that acts on a digest without marking
		// the mail read would re-arm into an instant, unbounded
		// wake loop on the same backlog.
		msgs, _, err := client.PollChanges(ctx, agentID, nil)
		switch {
		case err == nil && len(msgs) > 0:
			fresh, maxID := filterFreshMessages(
				msgs, watermark,
			)
			if len(fresh) > 0 {
				writeWatchWatermark(agentID, maxID)
				fmt.Print(formatWatchDigest(
					fresh, agentNameStr, watchMaxMsgs,
				))
				return nil
			}
			// Only already-digested backlog: keep parking.

		case err != nil:
			// Transient failure (server down, DB busy): park
			// and retry rather than waking the agent with an
			// error. The context check below still honors
			// kill signals during the retry sleep.
			if !sleepCtx(ctx, watchRetryInterval) {
				return errWatchInterrupted
			}
			continue
		}

		// Timeout: wake with an empty digest so the agent renews
		// the lease on its own schedule.
		if !deadline.IsZero() && time.Now().After(deadline) {
			fmt.Print(formatWatchTimeout(
				agentNameStr, watchTimeout,
			))
			return nil
		}

		// Periodic heartbeat keeps agent status accurate while
		// parked.
		if time.Since(lastHeartbeat) >= watchHeartbeat {
			_ = client.UpdateHeartbeat(ctx, agentID)
			lastHeartbeat = time.Now()
		}

		if !sleepCtx(ctx, watchPollInterval) {
			return errWatchInterrupted
		}
	}
}

// sleepCtx sleeps for d unless the context is canceled first. Returns
// false if the context was canceled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// formatWatchDigest renders the wake digest: the new messages with
// bodies (truncated), followed by handling and re-arm instructions.
// The digest is the watcher's exit payload — it is what the agent sees
// in the background-task notification, so it should usually be enough
// to act on without an extra inbox round-trip.
func formatWatchDigest(
	msgs []mail.InboxMessage, agentName string, maxMsgs int,
) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "== substrate watch: %d new message(s) for %s ==\n\n",
		len(msgs), agentName)

	shown := len(msgs)
	if maxMsgs > 0 && shown > maxMsgs {
		shown = maxMsgs
	}

	for i := 0; i < shown; i++ {
		msg := msgs[i]

		sender := msg.SenderName
		if sender == "" {
			sender = fmt.Sprintf("Agent#%d", msg.SenderID)
		}

		fmt.Fprintf(&sb, "[#%d] From: %s", msg.ID, sender)
		if msg.Priority == mail.PriorityUrgent {
			sb.WriteString("  [URGENT]")
		}
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Subject: %s\n", msg.Subject)
		fmt.Fprintf(&sb, "Thread: %s\n", msg.ThreadID)

		body := msg.Body
		if len(body) > watchBodyLimit {
			body = body[:watchBodyLimit] + fmt.Sprintf(
				"\n... [truncated, `substrate read %d` "+
					"for full message]", msg.ID,
			)
		}
		fmt.Fprintf(&sb, "%s\n\n---\n\n", body)
	}

	if len(msgs) > shown {
		fmt.Fprintf(&sb, "... and %d more (see `substrate inbox`)\n\n",
			len(msgs)-shown)
	}

	sb.WriteString(watchRearmFooter())

	return sb.String()
}

// formatWatchTimeout renders the wake output when the watch timeout
// expired with no events.
func formatWatchTimeout(agentName string, timeout time.Duration) string {
	var sb strings.Builder

	fmt.Fprintf(&sb,
		"== substrate watch: no events after %s for %s ==\n\n",
		timeout, agentName)
	sb.WriteString(watchRearmFooter())

	return sb.String()
}

// watchRearmFooter returns the standing instruction appended to every
// wake digest. It tells the agent how to process events and re-arm.
func watchRearmFooter() string {
	sid := sessionID
	if sid == "" {
		sid = os.Getenv("CLAUDE_SESSION_ID")
	}

	sidArg := `--session-id "$CLAUDE_SESSION_ID"`
	if sid != "" {
		sidArg = fmt.Sprintf("--session-id %q", sid)
	}

	return fmt.Sprintf(`Next steps:
1. Handle the messages above. Read with `+"`substrate read <id> %s`"+`,
   reply with `+"`substrate send %s --to <sender> --thread <thread-id> ...`"+`.
2. Re-arm the watcher so you keep getting woken: run
   `+"`substrate watch %s`"+`
   via the Bash tool with run_in_background: true, then end your turn.
`, sidArg, sidArg, sidArg)
}
