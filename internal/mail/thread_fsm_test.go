package mail

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestThreadFSM_UnreadToRead(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	require.Equal(t, "unread", fsm.StateString())
	require.False(t, fsm.IsTerminal())

	outbox, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())

	// Should emit persist and notify events.
	require.Len(t, outbox, 2)
	persist, ok := outbox[0].(PersistStateChange)
	require.True(t, ok)
	require.Equal(t, "read", persist.NewState)
	require.NotNil(t, persist.ReadAt)

	notify, ok := outbox[1].(NotifyStateChange)
	require.True(t, ok)
	require.Equal(t, "unread", notify.OldState)
	require.Equal(t, "read", notify.NewState)
}

func TestThreadFSM_UnreadToStarred(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	outbox, err := manager.Star(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "starred", fsm.StateString())
	require.Len(t, outbox, 2)
}

func TestThreadFSM_StarredToUnstarred(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Star the message.
	_, err := manager.Star(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "starred", fsm.StateString())

	// Unstar it.
	outbox, err := manager.Unstar(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())
	require.Len(t, outbox, 2)
}

func TestThreadFSM_UnreadToSnoozed(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	wakeTime := time.Now().Add(2 * time.Hour)
	outbox, err := manager.Snooze(ctx, fsm, wakeTime)
	require.NoError(t, err)
	require.Equal(t, "snoozed", fsm.StateString())

	// Should emit persist, schedule wake, and notify.
	require.Len(t, outbox, 3)

	persist, ok := outbox[0].(PersistStateChange)
	require.True(t, ok)
	require.Equal(t, "snoozed", persist.NewState)
	require.NotNil(t, persist.SnoozedUntil)

	scheduleWake, ok := outbox[1].(ScheduleWake)
	require.True(t, ok)
	require.Equal(t, wakeTime.Unix(), scheduleWake.WakeAt.Unix())
}

func TestThreadFSM_SnoozedToWake(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Snooze the message.
	wakeTime := time.Now().Add(2 * time.Hour)
	_, err := manager.Snooze(ctx, fsm, wakeTime)
	require.NoError(t, err)

	// Wake it.
	outbox, err := manager.Wake(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "unread", fsm.StateString())

	// Should emit persist, cancel wake, and notify.
	require.Len(t, outbox, 3)

	cancelWake, ok := outbox[1].(CancelScheduledWake)
	require.True(t, ok)
	require.Equal(t, int64(100), cancelWake.MessageID)
}

func TestThreadFSM_ReadToArchived(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Read the message.
	_, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)

	// Archive it.
	outbox, err := manager.Archive(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "archived", fsm.StateString())
	require.Len(t, outbox, 2)
}

func TestThreadFSM_ArchivedToUnarchived(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Read and archive.
	_, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)
	_, err = manager.Archive(ctx, fsm)
	require.NoError(t, err)

	// Unarchive.
	outbox, err := manager.Unarchive(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())
	require.Len(t, outbox, 2)
}

func TestThreadFSM_UnreadToTrash(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	outbox, err := manager.Trash(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "trash", fsm.StateString())
	require.True(t, fsm.IsTerminal())

	// Should emit persist, schedule purge, and notify.
	require.Len(t, outbox, 3)

	schedulePurge, ok := outbox[1].(SchedulePurge)
	require.True(t, ok)
	require.True(t, schedulePurge.PurgeAt.After(time.Now()))
}

func TestThreadFSM_TrashToRestore(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Trash the message.
	_, err := manager.Trash(ctx, fsm)
	require.NoError(t, err)

	// Restore it.
	outbox, err := manager.Restore(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "unread", fsm.StateString())
	require.False(t, fsm.IsTerminal())

	// Should emit persist, cancel wake (used for purge), and notify.
	require.Len(t, outbox, 3)
}

func TestThreadFSM_UnreadToAck(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	outbox, err := manager.Ack(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())

	// Should emit persist and notify.
	require.Len(t, outbox, 2)

	persist, ok := outbox[0].(PersistStateChange)
	require.True(t, ok)
	require.NotNil(t, persist.AckedAt)
	require.NotNil(t, persist.ReadAt)
}

func TestThreadFSM_ReadToAck(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Read first.
	_, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)

	// Then ack.
	outbox, err := manager.Ack(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())

	// Should emit only persist (no notify since state didn't change).
	require.Len(t, outbox, 1)

	persist, ok := outbox[0].(PersistStateChange)
	require.True(t, ok)
	require.NotNil(t, persist.AckedAt)
}

func TestThreadFSM_ReadAlreadyRead(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Read.
	_, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)

	// Read again.
	outbox, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())

	// Should be a no-op with no outbox events.
	require.Empty(t, outbox)
}

func TestThreadFSM_Resume_Snoozed(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)

	wakeTime := time.Now().Add(1 * time.Hour)
	fsm := manager.LoadFSM(1, 100, "thread-1", "snoozed", &wakeTime, nil, nil)

	// Resume should re-emit schedule wake.
	outbox, err := fsm.Resume(ctx)
	require.NoError(t, err)
	require.Len(t, outbox, 1)

	scheduleWake, ok := outbox[0].(ScheduleWake)
	require.True(t, ok)
	require.Equal(t, wakeTime.Unix(), scheduleWake.WakeAt.Unix())
}

func TestThreadFSM_Resume_Trash(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)

	fsm := manager.LoadFSM(1, 100, "thread-1", "trash", nil, nil, nil)

	// Resume should re-emit schedule purge.
	outbox, err := fsm.Resume(ctx)
	require.NoError(t, err)
	require.Len(t, outbox, 1)

	schedulePurge, ok := outbox[0].(SchedulePurge)
	require.True(t, ok)
	require.True(t, schedulePurge.PurgeAt.After(time.Now()))
}

func TestThreadFSM_Resume_Read(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)

	readAt := time.Now().Add(-1 * time.Hour)
	fsm := manager.LoadFSM(1, 100, "thread-1", "read", nil, &readAt, nil)

	// Resume should not emit any outbox events.
	outbox, err := fsm.Resume(ctx)
	require.NoError(t, err)
	require.Empty(t, outbox)
}

func TestThreadFSM_LoadFromDB(t *testing.T) {
	manager := NewThreadFSMManager(DefaultTrashRetention)

	// Test loading each state.
	testCases := []struct {
		state    string
		expected string
	}{
		{"unread", "unread"},
		{"read", "read"},
		{"starred", "starred"},
		{"snoozed", "snoozed"},
		{"archived", "archived"},
		{"trash", "trash"},
		{"unknown", "unread"}, // Unknown defaults to unread.
	}

	for _, tc := range testCases {
		t.Run(tc.state, func(t *testing.T) {
			fsm := manager.LoadFSM(1, 100, "thread-1", tc.state, nil, nil, nil)
			require.Equal(t, tc.expected, fsm.StateString())
		})
	}
}

func TestThreadFSM_UnexpectedEvent(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Unstar on unread should fail.
	_, err := manager.Unstar(ctx, fsm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected event")
}

func TestThreadFSM_SnoozedReSnooze(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Snooze.
	wakeTime1 := time.Now().Add(1 * time.Hour)
	_, err := manager.Snooze(ctx, fsm, wakeTime1)
	require.NoError(t, err)

	// Re-snooze with different time.
	wakeTime2 := time.Now().Add(2 * time.Hour)
	outbox, err := manager.Snooze(ctx, fsm, wakeTime2)
	require.NoError(t, err)
	require.Equal(t, "snoozed", fsm.StateString())

	// Should emit persist, cancel old wake, schedule new wake.
	require.Len(t, outbox, 3)

	cancelWake, ok := outbox[1].(CancelScheduledWake)
	require.True(t, ok)
	require.Equal(t, int64(100), cancelWake.MessageID)

	scheduleWake, ok := outbox[2].(ScheduleWake)
	require.True(t, ok)
	require.Equal(t, wakeTime2.Unix(), scheduleWake.WakeAt.Unix())
}

func TestThreadFSM_SnoozedToRead(t *testing.T) {
	ctx := context.Background()
	manager := NewThreadFSMManager(DefaultTrashRetention)
	fsm := manager.CreateFSM(1, 100, "thread-1")

	// Snooze.
	wakeTime := time.Now().Add(1 * time.Hour)
	_, err := manager.Snooze(ctx, fsm, wakeTime)
	require.NoError(t, err)

	// Read cancels snooze.
	outbox, err := manager.MarkRead(ctx, fsm)
	require.NoError(t, err)
	require.Equal(t, "read", fsm.StateString())

	// Should emit persist, cancel wake, and notify.
	require.Len(t, outbox, 3)

	cancelWake, ok := outbox[1].(CancelScheduledWake)
	require.True(t, ok)
	require.Equal(t, int64(100), cancelWake.MessageID)
}

func TestStateFromString(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"unread", "unread"},
		{"read", "read"},
		{"starred", "starred"},
		{"snoozed", "snoozed"},
		{"archived", "archived"},
		{"trash", "trash"},
		{"invalid", "unread"},
		{"", "unread"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			state := StateFromString(tc.input)
			require.Equal(t, tc.expected, state.String())
		})
	}
}
