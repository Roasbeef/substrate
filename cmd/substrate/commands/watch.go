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

// errAlreadyArmed is returned by acquireWatchLease when another live
// watcher already holds the advisory lock for this agent. It is a
// benign condition, not a fatal error: runWatch treats it as an exit-0
// no-op rather than a conflict, so a redundant re-arm does not surface
// as a failed background task.
var errAlreadyArmed = errors.New(
	"another watcher is already armed for this agent",
)

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
  - Lease: only one watcher per agent, enforced by an advisory file
    lock (flock). A second invocation while one is live is a benign
    no-op: it prints an "already armed" notice and exits 0 without
    disturbing the active watcher. The kernel releases the lock
    automatically if a watcher dies, so there is no stale-lock or
    PID-reuse hazard.
  - Heartbeats: sends liveness heartbeats while parked, so agent
    status stays accurate without any hook churn.
  - Server-down: retries with backoff internally rather than exiting.

Exit codes:
  0    woke with a digest on stdout, --timeout expired, or a watcher
       was already armed (no-op; do not re-arm)
  1    fatal error (identity resolution, lease I/O)
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

// readLeasePID reads the PID stored in a lease file. Returns 0 if the
// file does not exist or is malformed. The PID is advisory only: the
// flock, not this content, is the source of truth for ownership. It is
// written so `--check` and crash forensics can name the holder.
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

// watcherArmed reports whether a live watcher holds the advisory lock
// for the given agent. It probes with a non-blocking flock: if the lock
// is held the probe fails with EWOULDBLOCK (armed); if it succeeds no
// watcher is live, so it immediately drops the lock again. The kernel
// frees the lock when a holder dies, so a crashed watcher never reads
// as armed — there is no stale-lock or PID-reuse hazard.
func watcherArmed(agentID int64) (bool, error) {
	path, err := watchLockPath(agentID)
	if err != nil {
		return false, err
	}

	// O_RDONLY is enough: flock is independent of the open mode, and a
	// missing file means no watcher has ever armed.
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}
	defer f.Close()

	err = syscall.Flock(
		int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB,
	)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return true, nil
		}

		return false, err
	}

	// We took the lock, so nobody holds it. Release it before exit.
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return false, nil
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

// watchLease holds the exclusive advisory lock for an agent's watcher.
// The lock lives for the lifetime of the open file descriptor; closing
// it (via release) drops the lock, and the kernel drops it too if the
// process dies, so there is no stale-lock bookkeeping.
type watchLease struct {
	f *os.File
}

// acquireWatchLease claims the watcher lock for an agent with a
// non-blocking exclusive flock. The check and the claim are a single
// atomic kernel operation, so two watchers racing to arm cannot both
// win — exactly one gets the lock and the rest get errAlreadyArmed.
// This closes the TOCTOU hole in the old check-then-write lease, which
// could leave two live watchers for one agent. Returns the held lease,
// errAlreadyArmed if a live watcher already holds it, or a wrapped
// error on I/O failure.
func acquireWatchLease(agentID int64) (*watchLease, error) {
	path, err := watchLockPath(agentID)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lease: %w", err)
	}

	err = syscall.Flock(
		int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB,
	)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errAlreadyArmed
		}

		return nil, fmt.Errorf("failed to lock lease: %w", err)
	}

	// We hold the lock. Record our PID for observability only — the
	// flock above, not this content, gates ownership.
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	}

	return &watchLease{f: f}, nil
}

// release drops the advisory lock and closes the descriptor. The lease
// file is left in place: a waiter may already hold a descriptor on it,
// and ownership is gated by the flock, not by the file's existence.
func (l *watchLease) release() {
	if l == nil || l.f == nil {
		return
	}

	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
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

	lease, err := acquireWatchLease(agentID)
	switch {
	case errors.Is(err, errAlreadyArmed):
		// A live watcher already covers this agent. Re-arming is a
		// benign no-op, not a failure: print a notice and exit 0 so
		// the background task does not surface as a red failure and
		// the agent does not burn a turn investigating it.
		fmt.Print(formatAlreadyArmed(agentNameStr, agentID))
		return nil

	case err != nil:
		return err
	}
	defer lease.release()

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

// formatAlreadyArmed renders the no-op notice emitted when a watcher is
// re-armed while a live one already holds the lease. It exits 0, so the
// message must make clear this is expected and that the agent should
// NOT re-arm again — otherwise the agent could loop, arming repeatedly.
func formatAlreadyArmed(agentName string, agentID int64) string {
	holder := ""
	if path, err := watchLockPath(agentID); err == nil {
		if pid := readLeasePID(path); pid > 0 {
			holder = fmt.Sprintf(" (held by PID %d)", pid)
		}
	}

	var sb strings.Builder

	fmt.Fprintf(&sb,
		"== substrate watch: already armed for %s%s — no-op ==\n\n",
		agentName, holder)
	sb.WriteString(
		"A live watcher is already parked for this agent, so this " +
			"invocation did nothing. This is expected and harmless. " +
			"Do NOT run `substrate watch` again: end your turn, and " +
			"the existing watcher will wake you when mail arrives.\n",
	)

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
