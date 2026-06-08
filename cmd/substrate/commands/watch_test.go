package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/stretchr/testify/require"
)

// withTempHome points HOME at a temp dir so lease files do not touch
// the real ~/.subtrate, restoring the original value on cleanup.
func withTempHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	return home
}

// TestWatchLeaseAcquireRelease verifies the basic lease lifecycle:
// acquire writes our PID, release removes the file.
func TestWatchLeaseAcquireRelease(t *testing.T) {
	withTempHome(t)

	release, err := acquireWatchLease(42)
	require.NoError(t, err)

	path, err := watchLockPath(42)
	require.NoError(t, err)
	require.Equal(t, os.Getpid(), readLeasePID(path))

	armed, err := watcherArmed(42)
	require.NoError(t, err)
	require.True(t, armed)

	release()

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))

	armed, err = watcherArmed(42)
	require.NoError(t, err)
	require.False(t, armed)
}

// TestWatchLeaseConflict verifies that a live lease blocks a second
// acquisition with ErrWatcherArmed (exit code 5, conflict).
func TestWatchLeaseConflict(t *testing.T) {
	withTempHome(t)

	// Use a long-lived child process as the lease holder so the PID
	// is alive but is not our own process.
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	path, err := watchLockPath(7)
	require.NoError(t, err)
	err = os.WriteFile(
		path, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644,
	)
	require.NoError(t, err)

	_, err = acquireWatchLease(7)
	require.ErrorIs(t, err, ErrWatcherArmed)

	cliErr, ok := err.(*CLIError)
	require.True(t, ok)
	require.Equal(t, ExitConflict, cliErr.ExitCode())
}

// TestWatchLeaseStaleReclaim verifies that a lease held by a dead PID
// is reclaimed by the next acquisition.
func TestWatchLeaseStaleReclaim(t *testing.T) {
	withTempHome(t)

	// Spawn and immediately reap a child so its PID is known-dead.
	cmd := exec.Command("true")
	require.NoError(t, cmd.Run())
	deadPID := cmd.Process.Pid

	path, err := watchLockPath(9)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(strconv.Itoa(deadPID)), 0o644)
	require.NoError(t, err)

	armed, err := watcherArmed(9)
	require.NoError(t, err)
	require.False(t, armed, "dead PID should not count as armed")

	release, err := acquireWatchLease(9)
	require.NoError(t, err)
	defer release()

	require.Equal(t, os.Getpid(), readLeasePID(path))
}

// TestWatchLeaseReleaseRespectsSuccessor verifies that releasing a
// lease we no longer own (because a successor reclaimed it) does not
// remove the successor's lease file.
func TestWatchLeaseReleaseRespectsSuccessor(t *testing.T) {
	withTempHome(t)

	release, err := acquireWatchLease(11)
	require.NoError(t, err)

	// Simulate a successor overwriting the lease with another PID.
	path, err := watchLockPath(11)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte("999999"), 0o644)
	require.NoError(t, err)

	release()

	// The successor's lease must survive our release.
	require.Equal(t, 999999, readLeasePID(path))
}

// TestReadLeasePIDMalformed verifies malformed or missing lease files
// read as PID 0 (not armed).
func TestReadLeasePIDMalformed(t *testing.T) {
	home := withTempHome(t)

	require.Equal(t, 0, readLeasePID(
		filepath.Join(home, "does-not-exist"),
	))

	bad := filepath.Join(home, "bad.lock")
	require.NoError(t, os.WriteFile(bad, []byte("not-a-pid"), 0o644))
	require.Equal(t, 0, readLeasePID(bad))
}

// TestPidAlive verifies liveness probing for our own PID and a
// known-dead PID.
func TestPidAlive(t *testing.T) {
	require.True(t, pidAlive(os.Getpid()))
	require.False(t, pidAlive(0))
	require.False(t, pidAlive(-1))

	cmd := exec.Command("true")
	require.NoError(t, cmd.Run())
	require.False(t, pidAlive(cmd.Process.Pid))
}

// TestWatchWatermarkRoundTrip verifies watermark persistence: missing
// file reads as 0, writes round-trip, and malformed content reads as 0.
func TestWatchWatermarkRoundTrip(t *testing.T) {
	withTempHome(t)

	require.Equal(t, int64(0), readWatchWatermark(42))

	writeWatchWatermark(42, 7428)
	require.Equal(t, int64(7428), readWatchWatermark(42))

	// Watermarks are per-agent.
	require.Equal(t, int64(0), readWatchWatermark(43))

	// Malformed content reads as 0.
	path, err := watchWatermarkPath(42)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("junk"), 0o644))
	require.Equal(t, int64(0), readWatchWatermark(42))
}

// TestFilterFreshMessages verifies that messages at or below the
// watermark are dropped and the max fresh ID is reported, preventing
// the re-arm wake loop on an already-digested backlog.
func TestFilterFreshMessages(t *testing.T) {
	msgs := []mail.InboxMessage{
		{ID: 5}, {ID: 10}, {ID: 7},
	}

	// No watermark: everything is fresh.
	fresh, maxID := filterFreshMessages(msgs, 0)
	require.Len(t, fresh, 3)
	require.Equal(t, int64(10), maxID)

	// Mid watermark: only newer messages survive.
	fresh, maxID = filterFreshMessages(msgs, 7)
	require.Len(t, fresh, 1)
	require.Equal(t, int64(10), fresh[0].ID)
	require.Equal(t, int64(10), maxID)

	// Watermark at the top: nothing fresh, no wake.
	fresh, maxID = filterFreshMessages(msgs, 10)
	require.Empty(t, fresh)
	require.Equal(t, int64(0), maxID)
}

// TestErrWatchInterruptedExitCode verifies the signal-interrupt error
// carries the 128+SIGINT exit code so a kill is distinguishable from a
// wake (0) and a fatal error (1).
func TestErrWatchInterruptedExitCode(t *testing.T) {
	require.Equal(t, ExitInterrupted, errWatchInterrupted.ExitCode())
	require.Equal(t, 130, ExitInterrupted)
}

// TestFormatWatchDigest verifies the wake digest includes message
// payloads, truncation, overflow counts, and the re-arm footer.
func TestFormatWatchDigest(t *testing.T) {
	msgs := []mail.InboxMessage{
		{
			ID:         1,
			SenderName: "Daedalus",
			Subject:    "Re: review",
			ThreadID:   "thread-a",
			Priority:   mail.PriorityUrgent,
			Body:       "pushed fixup abc123",
			CreatedAt:  time.Now(),
		},
		{
			ID:        2,
			SenderID:  77,
			Subject:   "long one",
			ThreadID:  "thread-b",
			Body:      strings.Repeat("x", watchBodyLimit+100),
			CreatedAt: time.Now(),
		},
		{
			ID:        3,
			Subject:   "overflow",
			ThreadID:  "thread-c",
			Body:      "hidden",
			CreatedAt: time.Now(),
		},
	}

	out := formatWatchDigest(msgs, "TestAgent", 2)

	// Header counts all messages.
	require.Contains(t, out, "3 new message(s) for TestAgent")

	// First message: full payload with urgency marker.
	require.Contains(t, out, "[#1] From: Daedalus  [URGENT]")
	require.Contains(t, out, "pushed fixup abc123")
	require.Contains(t, out, "Thread: thread-a")

	// Second message: anonymous sender fallback and body truncation.
	require.Contains(t, out, "From: Agent#77")
	require.Contains(t, out, "`substrate read 2` for full message")
	require.NotContains(t, out, strings.Repeat("x", watchBodyLimit+1))

	// Third message: hidden behind the max-messages cap.
	require.NotContains(t, out, "hidden")
	require.Contains(t, out, "and 1 more")

	// Re-arm footer present.
	require.Contains(t, out, "Re-arm the watcher")
	require.Contains(t, out, "substrate watch")
}

// TestFormatWatchTimeout verifies the empty wake digest names the
// timeout and still carries the re-arm footer.
func TestFormatWatchTimeout(t *testing.T) {
	out := formatWatchTimeout("TestAgent", 4*time.Hour)

	require.Contains(t, out, "no events after 4h0m0s for TestAgent")
	require.Contains(t, out, "Re-arm the watcher")
}

// TestWatchRearmFooterSessionID verifies the footer embeds an explicit
// session ID when one is configured.
func TestWatchRearmFooterSessionID(t *testing.T) {
	old := sessionID
	sessionID = "sess-123"
	t.Cleanup(func() { sessionID = old })

	out := watchRearmFooter()
	require.Contains(t, out, `--session-id "sess-123"`)
}
