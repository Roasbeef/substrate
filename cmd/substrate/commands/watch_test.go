package commands

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/stretchr/testify/require"
)

// withTempHome gives a test separate home and project directories. Watch
// state belongs under the latter, while HOME remains relevant to the other
// CLI paths these tests exercise.
func withTempHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())

	return home
}

// TestWatchLockDirUsesProjectState proves the watcher and an imported hook
// can agree on their notification state even when their HOME directories
// differ. Loom deliberately gives those two processes different homes while
// keeping their project directory the same.
func TestWatchLockDirUsesProjectState(t *testing.T) {
	watcherHome := withTempHome(t)
	hookHome := t.TempDir()
	workspace := t.TempDir()
	nested := filepath.Join(workspace, "nested", "deeper")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	t.Setenv("CLAUDE_PROJECT_DIR", workspace)

	previousProject := projectDir
	projectDir = ""
	t.Cleanup(func() {
		projectDir = previousProject
	})

	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousDir))
	})

	dir, err := watchLockDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(workspace, ".substrate", "watch"), dir)
	require.DirExists(t, dir)

	key := sessionLeaseKey("lease-probe")

	lease, err := acquireWatchLease(key)
	require.NoError(t, err)
	t.Cleanup(func() {
		lease.release()
	})

	// The hook's real HOME cannot hide the watcher the tool acquired.
	t.Setenv("HOME", hookHome)
	armed, err := watcherArmed(key)
	require.NoError(t, err)
	require.True(t, armed)
	require.NoDirExists(t, filepath.Join(watcherHome, ".subtrate", "watch"))
	require.NoDirExists(t, filepath.Join(hookHome, ".subtrate", "watch"))
}

// TestWatchLockDirFindsGitRootFromNestedWorkingDir proves that an unset
// project environment still selects the worktree root. The Stop hook uses
// this same walk when Claude does not provide CLAUDE_PROJECT_DIR.
func TestWatchLockDirFindsGitRootFromNestedWorkingDir(t *testing.T) {
	withTempHome(t)
	workspace := t.TempDir()
	nested := filepath.Join(workspace, "nested", "deeper")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Setenv("CLAUDE_PROJECT_DIR", "")

	previousProject := projectDir
	projectDir = ""
	t.Cleanup(func() {
		projectDir = previousProject
	})

	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousDir))
	})
	canonicalWorkspace, err := filepath.EvalSymlinks(workspace)
	require.NoError(t, err)

	dir, err := watchLockDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(canonicalWorkspace, ".substrate", "watch"), dir)
}

// TestWatchLockDirPrefersExplicitProject keeps a caller's explicit project
// selection ahead of both its environment and current directory.
func TestWatchLockDirPrefersExplicitProject(t *testing.T) {
	withTempHome(t)
	explicit := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())

	previousProject := projectDir
	projectDir = explicit
	t.Cleanup(func() {
		projectDir = previousProject
	})

	dir, err := watchLockDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(explicit, ".substrate", "watch"), dir)
}

// TestWatchLeaseAcquireRelease verifies the basic lease lifecycle:
// acquire takes the lock and records our PID, watcherArmed reports
// armed while held, and release drops the lock so the agent reads as
// unarmed again. The lease file is intentionally left in place after
// release — ownership is gated by the flock, not the file's existence.
func TestWatchLeaseAcquireRelease(t *testing.T) {
	withTempHome(t)

	lease, err := acquireWatchLease("agent-42")
	require.NoError(t, err)

	path, err := watchLockPath("agent-42")
	require.NoError(t, err)
	require.Equal(t, os.Getpid(), readLeasePID(path))

	armed, err := watcherArmed("agent-42")
	require.NoError(t, err)
	require.True(t, armed)

	lease.release()

	armed, err = watcherArmed("agent-42")
	require.NoError(t, err)
	require.False(t, armed, "lease must read unarmed after release")
}

// TestWatchLeaseConflict verifies that a live lease blocks a second
// acquisition with errAlreadyArmed. Because flock is keyed on the open
// file description, a second acquire in this same process contends with
// the first exactly as a separate process would.
func TestWatchLeaseConflict(t *testing.T) {
	withTempHome(t)

	lease, err := acquireWatchLease("agent-7")
	require.NoError(t, err)
	defer lease.release()

	_, err = acquireWatchLease("agent-7")
	require.ErrorIs(t, err, errAlreadyArmed)
}

// TestWatchLeaseStaleReclaim verifies that a lease file left behind by
// a dead watcher (content present, no flock held) does not block a new
// acquisition. The kernel released the dead holder's lock, so the file
// is just stale bytes — acquire succeeds and overwrites the PID.
func TestWatchLeaseStaleReclaim(t *testing.T) {
	withTempHome(t)

	// Spawn and immediately reap a child so its PID is known-dead,
	// then plant it as leftover lease content with no lock held.
	cmd := exec.Command("true")
	require.NoError(t, cmd.Run())
	deadPID := cmd.Process.Pid

	path, err := watchLockPath("agent-9")
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(strconv.Itoa(deadPID)), 0o644)
	require.NoError(t, err)

	armed, err := watcherArmed("agent-9")
	require.NoError(t, err)
	require.False(t, armed, "unlocked stale file must not read armed")

	lease, err := acquireWatchLease("agent-9")
	require.NoError(t, err)
	defer lease.release()

	require.Equal(t, os.Getpid(), readLeasePID(path))
}

// TestWatchLeaseConcurrent is the core mutual-exclusion guarantee:
// when N goroutines race to arm the same agent, exactly one wins and
// the rest get errAlreadyArmed. The winner holds its lock until every
// attempt has resolved, so this exercises true contention rather than
// sequential acquire/release. This is the regression test for the
// TOCTOU hole that previously let two live watchers share one agent.
func TestWatchLeaseConcurrent(t *testing.T) {
	withTempHome(t)

	const n = 16

	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		mu      sync.Mutex
		winners []*watchLease
		armedNo int
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Block until released so all attempts contend at once.
			<-start

			lease, err := acquireWatchLease("agent-123")

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners = append(winners, lease)
			case errors.Is(err, errAlreadyArmed):
				armedNo++
			default:
				t.Errorf("unexpected acquire error: %v", err)
			}
		}()
	}

	close(start)
	wg.Wait()

	require.Len(t, winners, 1, "exactly one watcher may hold the lease")
	require.Equal(t, n-1, armedNo, "all losers must see errAlreadyArmed")

	// Releasing the sole winner frees the lease for a fresh acquire.
	winners[0].release()

	lease, err := acquireWatchLease("agent-123")
	require.NoError(t, err)
	lease.release()
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

// TestWatchWatermarkRoundTrip verifies watermark persistence: missing
// file reads as 0, writes round-trip, and malformed content reads as 0.
func TestWatchWatermarkRoundTrip(t *testing.T) {
	withTempHome(t)

	require.Equal(t, int64(0), readWatchWatermark("agent-42"))

	writeWatchWatermark("agent-42", 7428)
	require.Equal(t, int64(7428), readWatchWatermark("agent-42"))

	// Watermarks are per lease key.
	require.Equal(t, int64(0), readWatchWatermark("agent-43"))

	// Malformed content reads as 0.
	path, err := watchWatermarkPath("agent-42")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("junk"), 0o644))
	require.Equal(t, int64(0), readWatchWatermark("agent-42"))
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

// TestWatchLeaseKeySessionScoped is the regression test for the bug
// that left most sessions watcher-less: the lease used to be keyed by
// agent, but many sessions share one agent (every session opened in a
// project resolves to that project's default agent). The first session
// took the only lease and every later one read as "already armed" while
// no watcher was ever armed on its behalf. Distinct sessions must get
// distinct keys even on a single agent.
func TestWatchLeaseKeySessionScoped(t *testing.T) {
	withTempHome(t)

	keyA := sessionLeaseKey("session-a")
	keyB := sessionLeaseKey("session-b")
	require.NotEqual(t, keyA, keyB)

	// Both sessions arm concurrently on the same agent.
	leaseA, err := acquireWatchLease(keyA)
	require.NoError(t, err)
	defer leaseA.release()

	leaseB, err := acquireWatchLease(keyB)
	require.NoError(t, err, "a second session must be able to arm")
	defer leaseB.release()

	// Re-arming the same session is still the benign no-op.
	_, err = acquireWatchLease(keyA)
	require.ErrorIs(t, err, errAlreadyArmed)

	// Watermarks are session-scoped too: one session waking on a
	// message must not suppress the other session's wake.
	writeWatchWatermark(keyA, 900)
	require.Equal(t, int64(0), readWatchWatermark(keyB))
}

// TestWatchLeaseKeyFallbackAndSanitizing verifies that an invocation
// without a session ID still gets the agent-keyed lease, and that a
// session ID cannot name anything outside the lease directory.
func TestWatchLeaseKeyFallbackAndSanitizing(t *testing.T) {
	require.Equal(t, "agent-42", agentLeaseKey(42))
	require.Equal(
		t, "session-.._.._etc_passwd",
		sessionLeaseKey("../../etc/passwd"),
	)
}
