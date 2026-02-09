package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/stretchr/testify/require"
)

// testTaskDB creates a fresh SQLite database with all migrations applied for
// task store testing.
func testTaskDB(t *testing.T) (*SqlcStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "subtrate-store-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	sqlDB, err := sql.Open(
		"sqlite3",
		dbPath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000",
	)
	require.NoError(t, err)

	store := NewSqlcStore(sqlDB)

	migrationsDir := findTaskMigrationsDir(t)
	err = db.RunMigrations(sqlDB, migrationsDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}
	return store, cleanup
}

// findTaskMigrationsDir locates the migrations directory relative to the
// test file.
func findTaskMigrationsDir(t *testing.T) string {
	t.Helper()

	paths := []string{
		"../db/migrations",
		"../../internal/db/migrations",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		p := filepath.Join(
			gopath,
			"src/github.com/roasbeef/subtrate/internal/db/migrations",
		)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatal("Could not find migrations directory")
	return ""
}

// createTestAgent creates a test agent and returns its database ID.
func createTestAgent(t *testing.T, s *SqlcStore, name string) int64 {
	t.Helper()

	ctx := context.Background()
	agent, err := s.CreateAgent(ctx, CreateAgentParams{Name: name})
	require.NoError(t, err)

	return agent.ID
}

// createTestTaskList creates a task list for testing and returns it.
func createTestTaskList(
	t *testing.T, s *SqlcStore, listID string, agentID int64,
) TaskList {
	t.Helper()

	ctx := context.Background()
	tl, err := s.CreateTaskList(ctx, CreateTaskListParams{
		ListID:    listID,
		AgentID:   agentID,
		WatchPath: "/tmp/tasks/" + listID,
	})
	require.NoError(t, err)

	return tl
}

// createTestTask creates a task with the given params and returns it.
func createTestTask(
	t *testing.T, s *SqlcStore, params CreateTaskParams,
) Task {
	t.Helper()

	ctx := context.Background()
	task, err := s.CreateTask(ctx, params)
	require.NoError(t, err)

	return task
}

// TestTaskList_CRUD verifies the full lifecycle of task list operations:
// create, get by listID, get by DB ID, list all, list by agent, update sync
// time, and delete.
func TestTaskList_CRUD(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()

	agentA := createTestAgent(t, store, "AgentAlpha")
	agentB := createTestAgent(t, store, "AgentBeta")

	// Create task lists.
	tlA, err := store.CreateTaskList(ctx, CreateTaskListParams{
		ListID:    "list-alpha-1",
		AgentID:   agentA,
		WatchPath: "/home/alpha/.claude/tasks/list-alpha-1",
	})
	require.NoError(t, err)
	require.Equal(t, "list-alpha-1", tlA.ListID)
	require.Equal(t, agentA, tlA.AgentID)
	require.Nil(t, tlA.LastSyncedAt)

	tlB, err := store.CreateTaskList(ctx, CreateTaskListParams{
		ListID:    "list-beta-1",
		AgentID:   agentB,
		WatchPath: "/home/beta/.claude/tasks/list-beta-1",
	})
	require.NoError(t, err)
	require.Equal(t, "list-beta-1", tlB.ListID)

	// Get by list ID.
	got, err := store.GetTaskList(ctx, "list-alpha-1")
	require.NoError(t, err)
	require.Equal(t, tlA.ID, got.ID)
	require.Equal(t, "list-alpha-1", got.ListID)
	require.Equal(t, agentA, got.AgentID)

	// Get by database ID.
	gotByID, err := store.GetTaskListByID(ctx, tlA.ID)
	require.NoError(t, err)
	require.Equal(t, tlA.ListID, gotByID.ListID)

	// Get nonexistent list returns error.
	_, err = store.GetTaskList(ctx, "nonexistent-list")
	require.Error(t, err)

	// List all task lists.
	allLists, err := store.ListTaskLists(ctx)
	require.NoError(t, err)
	require.Len(t, allLists, 2)

	// List by agent.
	agentALists, err := store.ListTaskListsByAgent(ctx, agentA)
	require.NoError(t, err)
	require.Len(t, agentALists, 1)
	require.Equal(t, "list-alpha-1", agentALists[0].ListID)

	agentBLists, err := store.ListTaskListsByAgent(ctx, agentB)
	require.NoError(t, err)
	require.Len(t, agentBLists, 1)
	require.Equal(t, "list-beta-1", agentBLists[0].ListID)

	// Update sync time.
	syncTime := time.Now().Truncate(time.Second)
	err = store.UpdateTaskListSyncTime(ctx, "list-alpha-1", syncTime)
	require.NoError(t, err)

	updated, err := store.GetTaskList(ctx, "list-alpha-1")
	require.NoError(t, err)
	require.NotNil(t, updated.LastSyncedAt)
	require.Equal(t, syncTime.Unix(), updated.LastSyncedAt.Unix())

	// Delete task list.
	err = store.DeleteTaskList(ctx, "list-alpha-1")
	require.NoError(t, err)

	_, err = store.GetTaskList(ctx, "list-alpha-1")
	require.Error(t, err)

	// Only list-beta-1 remains.
	remaining, err := store.ListTaskLists(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, "list-beta-1", remaining[0].ListID)
}

// TestTask_CreateAndGet verifies task creation and retrieval by both database
// ID and Claude task ID.
func TestTask_CreateAndGet(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "TestAgent")
	createTestTaskList(t, store, "list-1", agentID)

	// Create a task.
	task := createTestTask(t, store, CreateTaskParams{
		AgentID:      agentID,
		ListID:       "list-1",
		ClaudeTaskID: "task-abc-123",
		Subject:      "Implement feature X",
		Description:  "Detailed description of feature X.",
		ActiveForm:   "Implementing feature X",
		Metadata:     `{"priority": "high"}`,
		Status:       "pending",
		Owner:        "",
		BlockedBy:    "[]",
		Blocks:       "[]",
		FilePath:     "/tmp/tasks/list-1/task-abc-123.json",
		FileMtime:    1700000000,
	})

	require.NotZero(t, task.ID)
	require.Equal(t, agentID, task.AgentID)
	require.Equal(t, "list-1", task.ListID)
	require.Equal(t, "task-abc-123", task.ClaudeTaskID)
	require.Equal(t, "Implement feature X", task.Subject)
	require.Equal(t, "Detailed description of feature X.", task.Description)
	require.Equal(t, "Implementing feature X", task.ActiveForm)
	require.Equal(t, `{"priority": "high"}`, task.Metadata)
	require.Equal(t, "pending", task.Status)
	require.Empty(t, task.Owner)
	require.Equal(t, "[]", task.BlockedBy)
	require.Equal(t, "[]", task.Blocks)
	require.Nil(t, task.StartedAt)
	require.Nil(t, task.CompletedAt)
	require.Equal(t, "/tmp/tasks/list-1/task-abc-123.json", task.FilePath)
	require.Equal(t, int64(1700000000), task.FileMtime)

	// Get by database ID.
	got, err := store.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, task.ID, got.ID)
	require.Equal(t, task.Subject, got.Subject)

	// Get by Claude task ID.
	gotByClaude, err := store.GetTaskByClaudeID(ctx, "list-1", "task-abc-123")
	require.NoError(t, err)
	require.Equal(t, task.ID, gotByClaude.ID)
	require.Equal(t, task.ClaudeTaskID, gotByClaude.ClaudeTaskID)

	// Get nonexistent task returns error.
	_, err = store.GetTask(ctx, 99999)
	require.Error(t, err)

	_, err = store.GetTaskByClaudeID(ctx, "list-1", "nonexistent")
	require.Error(t, err)
}

// TestTask_Upsert verifies that upsert creates a new task on first call and
// updates the existing task on subsequent calls with the same list+claude ID.
func TestTask_Upsert(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "UpsertAgent")
	createTestTaskList(t, store, "upsert-list", agentID)

	// First upsert creates the task.
	created, err := store.UpsertTask(ctx, UpsertTaskParams{
		AgentID:      agentID,
		ListID:       "upsert-list",
		ClaudeTaskID: "task-ups-1",
		Subject:      "Original subject",
		Description:  "Original description",
		Status:       "pending",
		BlockedBy:    "[]",
		Blocks:       "[]",
		FilePath:     "/path/to/task.json",
		FileMtime:    1000,
	})
	require.NoError(t, err)
	require.Equal(t, "Original subject", created.Subject)
	require.Equal(t, "pending", created.Status)

	originalID := created.ID

	// Second upsert updates the existing task.
	updated, err := store.UpsertTask(ctx, UpsertTaskParams{
		AgentID:      agentID,
		ListID:       "upsert-list",
		ClaudeTaskID: "task-ups-1",
		Subject:      "Updated subject",
		Description:  "Updated description",
		Status:       "in_progress",
		BlockedBy:    "[]",
		Blocks:       "[]",
		FilePath:     "/path/to/task.json",
		FileMtime:    2000,
	})
	require.NoError(t, err)
	require.Equal(t, originalID, updated.ID)
	require.Equal(t, "Updated subject", updated.Subject)
	require.Equal(t, "Updated description", updated.Description)
	require.Equal(t, "in_progress", updated.Status)
	require.Equal(t, int64(2000), updated.FileMtime)

	// The upsert to in_progress should set started_at because the original
	// task had no started_at.
	require.NotNil(t, updated.StartedAt)

	// Verify only one task exists.
	tasks, err := store.ListTasksByAgent(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Third upsert to completed should set completed_at.
	completed, err := store.UpsertTask(ctx, UpsertTaskParams{
		AgentID:      agentID,
		ListID:       "upsert-list",
		ClaudeTaskID: "task-ups-1",
		Subject:      "Completed subject",
		Description:  "Done",
		Status:       "completed",
		BlockedBy:    "[]",
		Blocks:       "[]",
		FilePath:     "/path/to/task.json",
		FileMtime:    3000,
	})
	require.NoError(t, err)
	require.Equal(t, originalID, completed.ID)
	require.Equal(t, "completed", completed.Status)
	require.NotNil(t, completed.CompletedAt)
	// started_at should be preserved from the previous upsert.
	require.NotNil(t, completed.StartedAt)
}

// TestTask_ListByAgent verifies that listing tasks by agent correctly filters
// results to only the specified agent's tasks.
func TestTask_ListByAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentA := createTestAgent(t, store, "AgentA")
	agentB := createTestAgent(t, store, "AgentB")
	createTestTaskList(t, store, "list-a", agentA)
	createTestTaskList(t, store, "list-b", agentB)

	// Create tasks for agent A.
	for i := 0; i < 3; i++ {
		createTestTask(t, store, CreateTaskParams{
			AgentID:      agentA,
			ListID:       "list-a",
			ClaudeTaskID: "task-a-" + string(rune('0'+i)),
			Subject:      "Agent A task",
			Status:       "pending",
			BlockedBy:    "[]",
			Blocks:       "[]",
		})
	}

	// Create tasks for agent B.
	for i := 0; i < 2; i++ {
		createTestTask(t, store, CreateTaskParams{
			AgentID:      agentB,
			ListID:       "list-b",
			ClaudeTaskID: "task-b-" + string(rune('0'+i)),
			Subject:      "Agent B task",
			Status:       "pending",
			BlockedBy:    "[]",
			Blocks:       "[]",
		})
	}

	// List by agent A should return 3 tasks.
	tasksA, err := store.ListTasksByAgent(ctx, agentA)
	require.NoError(t, err)
	require.Len(t, tasksA, 3)
	for _, task := range tasksA {
		require.Equal(t, agentA, task.AgentID)
	}

	// List by agent B should return 2 tasks.
	tasksB, err := store.ListTasksByAgent(ctx, agentB)
	require.NoError(t, err)
	require.Len(t, tasksB, 2)
	for _, task := range tasksB {
		require.Equal(t, agentB, task.AgentID)
	}

	// List by nonexistent agent should return empty.
	tasksNone, err := store.ListTasksByAgent(ctx, 99999)
	require.NoError(t, err)
	require.Empty(t, tasksNone)

	// Test ListTasksByAgentWithLimit.
	limited, err := store.ListTasksByAgentWithLimit(ctx, agentA, 2, 0)
	require.NoError(t, err)
	require.Len(t, limited, 2)

	offset, err := store.ListTasksByAgentWithLimit(ctx, agentA, 2, 2)
	require.NoError(t, err)
	require.Len(t, offset, 1)
}

// TestTask_ListActiveByAgent verifies that ListActiveTasksByAgent returns only
// tasks with pending or in_progress status.
func TestTask_ListActiveByAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "ActiveAgent")
	createTestTaskList(t, store, "active-list", agentID)

	// Create tasks with different statuses.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "active-list",
		ClaudeTaskID: "task-pending", Subject: "Pending",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "active-list",
		ClaudeTaskID: "task-progress", Subject: "In Progress",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "active-list",
		ClaudeTaskID: "task-completed", Subject: "Completed",
		Status: "completed", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "active-list",
		ClaudeTaskID: "task-deleted", Subject: "Deleted",
		Status: "deleted", BlockedBy: "[]", Blocks: "[]",
	})

	active, err := store.ListActiveTasksByAgent(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, active, 2)

	statuses := make(map[string]bool)
	for _, task := range active {
		statuses[task.Status] = true
	}
	require.True(t, statuses["pending"])
	require.True(t, statuses["in_progress"])
	require.False(t, statuses["completed"])
	require.False(t, statuses["deleted"])
}

// TestTask_ListByList verifies that listing tasks by list ID correctly filters
// results to only tasks in the specified list.
func TestTask_ListByList(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "ListAgent")
	createTestTaskList(t, store, "list-x", agentID)
	createTestTaskList(t, store, "list-y", agentID)

	// Create tasks in list X.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "list-x",
		ClaudeTaskID: "task-x-1", Subject: "List X task 1",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "list-x",
		ClaudeTaskID: "task-x-2", Subject: "List X task 2",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})

	// Create task in list Y.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "list-y",
		ClaudeTaskID: "task-y-1", Subject: "List Y task 1",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	tasksX, err := store.ListTasksByList(ctx, "list-x")
	require.NoError(t, err)
	require.Len(t, tasksX, 2)
	for _, task := range tasksX {
		require.Equal(t, "list-x", task.ListID)
	}

	tasksY, err := store.ListTasksByList(ctx, "list-y")
	require.NoError(t, err)
	require.Len(t, tasksY, 1)
	require.Equal(t, "list-y", tasksY[0].ListID)

	// Nonexistent list returns empty.
	tasksZ, err := store.ListTasksByList(ctx, "list-z")
	require.NoError(t, err)
	require.Empty(t, tasksZ)
}

// TestTask_ListInProgress verifies that ListInProgressTasks returns only tasks
// with in_progress status for the given agent.
func TestTask_ListInProgress(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "ProgressAgent")
	createTestTaskList(t, store, "progress-list", agentID)

	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "progress-list",
		ClaudeTaskID: "task-1", Subject: "Pending task",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "progress-list",
		ClaudeTaskID: "task-2", Subject: "In progress 1",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "progress-list",
		ClaudeTaskID: "task-3", Subject: "In progress 2",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "progress-list",
		ClaudeTaskID: "task-4", Subject: "Completed task",
		Status: "completed", BlockedBy: "[]", Blocks: "[]",
	})

	inProgress, err := store.ListInProgressTasks(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, inProgress, 2)
	for _, task := range inProgress {
		require.Equal(t, "in_progress", task.Status)
	}
}

// TestTask_ListPending verifies that ListPendingTasks returns only tasks with
// pending status for the given agent.
func TestTask_ListPending(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "PendingAgent")
	createTestTaskList(t, store, "pending-list", agentID)

	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "pending-list",
		ClaudeTaskID: "task-p1", Subject: "Pending 1",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "pending-list",
		ClaudeTaskID: "task-p2", Subject: "Pending 2",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "pending-list",
		ClaudeTaskID: "task-ip", Subject: "In Progress",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})

	pending, err := store.ListPendingTasks(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	for _, task := range pending {
		require.Equal(t, "pending", task.Status)
	}
}

// TestTask_ListBlocked verifies that ListBlockedTasks returns only pending
// tasks that have a non-empty blocked_by field.
func TestTask_ListBlocked(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "BlockedAgent")
	createTestTaskList(t, store, "blocked-list", agentID)

	// Pending with no blockers.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "blocked-list",
		ClaudeTaskID: "task-free", Subject: "Free task",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	// Pending with blockers.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "blocked-list",
		ClaudeTaskID: "task-blocked-1", Subject: "Blocked 1",
		Status: "pending", BlockedBy: `["task-free"]`, Blocks: "[]",
	})

	// Pending with blockers.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "blocked-list",
		ClaudeTaskID: "task-blocked-2", Subject: "Blocked 2",
		Status:    "pending",
		BlockedBy: `["task-free","task-other"]`,
		Blocks:    "[]",
	})

	// In-progress (not pending, should be excluded even if blocked_by set).
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "blocked-list",
		ClaudeTaskID: "task-ip-blocked", Subject: "In Progress Blocked",
		Status: "in_progress", BlockedBy: `["task-free"]`, Blocks: "[]",
	})

	blocked, err := store.ListBlockedTasks(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, blocked, 2)
	for _, task := range blocked {
		require.Equal(t, "pending", task.Status)
		require.NotEqual(t, "[]", task.BlockedBy)
	}
}

// TestTask_ListAvailable verifies that ListAvailableTasks returns only pending
// tasks with no owner and no blockers.
func TestTask_ListAvailable(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "AvailAgent")
	createTestTaskList(t, store, "avail-list", agentID)

	// Available: pending, no owner, no blockers.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "avail-list",
		ClaudeTaskID: "task-avail-1", Subject: "Available 1",
		Status: "pending", Owner: "", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "avail-list",
		ClaudeTaskID: "task-avail-2", Subject: "Available 2",
		Status: "pending", Owner: "", BlockedBy: "", Blocks: "[]",
	})

	// Not available: has owner.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "avail-list",
		ClaudeTaskID: "task-owned", Subject: "Owned",
		Status: "pending", Owner: "SomeAgent",
		BlockedBy: "[]", Blocks: "[]",
	})

	// Not available: has blockers.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "avail-list",
		ClaudeTaskID: "task-blocked", Subject: "Blocked",
		Status: "pending", Owner: "",
		BlockedBy: `["task-other"]`, Blocks: "[]",
	})

	// Not available: not pending.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "avail-list",
		ClaudeTaskID: "task-ip", Subject: "In Progress",
		Status: "in_progress", Owner: "",
		BlockedBy: "[]", Blocks: "[]",
	})

	available, err := store.ListAvailableTasks(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, available, 2)
	for _, task := range available {
		require.Equal(t, "pending", task.Status)
		require.Empty(t, task.Owner)
		require.True(
			t, task.BlockedBy == "[]" || task.BlockedBy == "",
			"expected no blockers, got: %s", task.BlockedBy,
		)
	}
}

// TestTask_UpdateStatus verifies that updating a task's status correctly sets
// the started_at timestamp when transitioning to in_progress and the
// completed_at timestamp when transitioning to completed.
func TestTask_UpdateStatus(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "StatusAgent")
	createTestTaskList(t, store, "status-list", agentID)

	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "status-list",
		ClaudeTaskID: "task-status", Subject: "Status test",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	// Verify initial state.
	task, err := store.GetTaskByClaudeID(ctx, "status-list", "task-status")
	require.NoError(t, err)
	require.Equal(t, "pending", task.Status)
	require.Nil(t, task.StartedAt)
	require.Nil(t, task.CompletedAt)

	// Transition to in_progress.
	now := time.Now().Truncate(time.Second)
	err = store.UpdateTaskStatus(
		ctx, "status-list", "task-status", "in_progress", now,
	)
	require.NoError(t, err)

	task, err = store.GetTaskByClaudeID(ctx, "status-list", "task-status")
	require.NoError(t, err)
	require.Equal(t, "in_progress", task.Status)
	require.NotNil(t, task.StartedAt)
	require.Equal(t, now.Unix(), task.StartedAt.Unix())
	require.Nil(t, task.CompletedAt)

	// Transition to completed.
	completedNow := now.Add(10 * time.Second)
	err = store.UpdateTaskStatus(
		ctx, "status-list", "task-status", "completed", completedNow,
	)
	require.NoError(t, err)

	task, err = store.GetTaskByClaudeID(ctx, "status-list", "task-status")
	require.NoError(t, err)
	require.Equal(t, "completed", task.Status)
	// started_at should be preserved.
	require.NotNil(t, task.StartedAt)
	require.Equal(t, now.Unix(), task.StartedAt.Unix())
	// completed_at should be set.
	require.NotNil(t, task.CompletedAt)
	require.Equal(t, completedNow.Unix(), task.CompletedAt.Unix())

	// Re-setting to in_progress should NOT overwrite started_at since
	// it is already set.
	laterTime := completedNow.Add(10 * time.Second)
	err = store.UpdateTaskStatus(
		ctx, "status-list", "task-status", "in_progress", laterTime,
	)
	require.NoError(t, err)

	task, err = store.GetTaskByClaudeID(ctx, "status-list", "task-status")
	require.NoError(t, err)
	require.Equal(t, "in_progress", task.Status)
	require.NotNil(t, task.StartedAt)
	// started_at should remain the original value.
	require.Equal(t, now.Unix(), task.StartedAt.Unix())
}

// TestTask_UpdateOwner verifies that assigning an owner to a task works
// correctly.
func TestTask_UpdateOwner(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "OwnerAgent")
	createTestTaskList(t, store, "owner-list", agentID)

	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "owner-list",
		ClaudeTaskID: "task-owner", Subject: "Owner test",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	// Initially no owner.
	task, err := store.GetTaskByClaudeID(ctx, "owner-list", "task-owner")
	require.NoError(t, err)
	require.Empty(t, task.Owner)

	// Assign owner.
	now := time.Now().Truncate(time.Second)
	err = store.UpdateTaskOwner(
		ctx, "owner-list", "task-owner", "AssignedAgent", now,
	)
	require.NoError(t, err)

	task, err = store.GetTaskByClaudeID(ctx, "owner-list", "task-owner")
	require.NoError(t, err)
	require.Equal(t, "AssignedAgent", task.Owner)

	// Clear owner.
	laterNow := now.Add(5 * time.Second)
	err = store.UpdateTaskOwner(
		ctx, "owner-list", "task-owner", "", laterNow,
	)
	require.NoError(t, err)

	task, err = store.GetTaskByClaudeID(ctx, "owner-list", "task-owner")
	require.NoError(t, err)
	require.Empty(t, task.Owner)
}

// TestTask_Stats verifies the task statistics functions:
// GetTaskStatsByAgent, GetTaskStatsByList, GetAllTaskStats, and
// GetAllAgentTaskStats.
func TestTask_Stats(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentA := createTestAgent(t, store, "StatsAgentA")
	agentB := createTestAgent(t, store, "StatsAgentB")
	createTestTaskList(t, store, "stats-list-a", agentA)
	createTestTaskList(t, store, "stats-list-b", agentB)

	todaySince := time.Now().Add(-24 * time.Hour)

	// Agent A tasks: 2 pending (1 blocked, 1 available), 1 in_progress,
	// 1 completed today.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentA, ListID: "stats-list-a",
		ClaudeTaskID: "a-pending-avail", Subject: "Pending available",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentA, ListID: "stats-list-a",
		ClaudeTaskID: "a-pending-blocked", Subject: "Pending blocked",
		Status: "pending", BlockedBy: `["some-task"]`, Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentA, ListID: "stats-list-a",
		ClaudeTaskID: "a-in-progress", Subject: "In progress",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})

	// Create a completed task and set its completed_at via UpdateTaskStatus.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentA, ListID: "stats-list-a",
		ClaudeTaskID: "a-completed", Subject: "Completed today",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	err := store.UpdateTaskStatus(
		ctx, "stats-list-a", "a-completed", "completed", time.Now(),
	)
	require.NoError(t, err)

	// Agent B tasks: 1 pending available.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentB, ListID: "stats-list-b",
		ClaudeTaskID: "b-pending", Subject: "B pending",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	// GetTaskStatsByAgent for agent A.
	statsA, err := store.GetTaskStatsByAgent(ctx, agentA, todaySince)
	require.NoError(t, err)
	require.Equal(t, int64(2), statsA.PendingCount)
	require.Equal(t, int64(1), statsA.InProgressCount)
	require.Equal(t, int64(1), statsA.CompletedCount)
	require.Equal(t, int64(1), statsA.BlockedCount)
	require.Equal(t, int64(1), statsA.AvailableCount)
	require.Equal(t, int64(1), statsA.CompletedToday)

	// GetTaskStatsByAgent for agent B.
	statsB, err := store.GetTaskStatsByAgent(ctx, agentB, todaySince)
	require.NoError(t, err)
	require.Equal(t, int64(1), statsB.PendingCount)
	require.Equal(t, int64(0), statsB.InProgressCount)
	require.Equal(t, int64(0), statsB.CompletedCount)

	// GetTaskStatsByList.
	statsListA, err := store.GetTaskStatsByList(
		ctx, "stats-list-a", todaySince,
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), statsListA.PendingCount)
	require.Equal(t, int64(1), statsListA.InProgressCount)
	require.Equal(t, int64(1), statsListA.CompletedCount)

	// GetAllTaskStats (global).
	globalStats, err := store.GetAllTaskStats(ctx, todaySince)
	require.NoError(t, err)
	require.Equal(t, int64(3), globalStats.PendingCount)
	require.Equal(t, int64(1), globalStats.InProgressCount)
	require.Equal(t, int64(1), globalStats.CompletedCount)
	require.Equal(t, int64(1), globalStats.BlockedCount)
	require.Equal(t, int64(1), globalStats.CompletedToday)

	// GetAllAgentTaskStats.
	agentStats, err := store.GetAllAgentTaskStats(ctx, todaySince)
	require.NoError(t, err)
	require.Len(t, agentStats, 2)

	// Build map for easier assertion.
	statsMap := make(map[int64]AgentTaskStats)
	for _, s := range agentStats {
		statsMap[s.AgentID] = s
	}

	require.Equal(t, int64(2), statsMap[agentA].PendingCount)
	require.Equal(t, int64(1), statsMap[agentA].InProgressCount)
	require.Equal(t, int64(1), statsMap[agentA].BlockedCount)
	require.Equal(t, int64(1), statsMap[agentA].CompletedToday)

	require.Equal(t, int64(1), statsMap[agentB].PendingCount)
	require.Equal(t, int64(0), statsMap[agentB].InProgressCount)
	require.Equal(t, int64(0), statsMap[agentB].BlockedCount)
	require.Equal(t, int64(0), statsMap[agentB].CompletedToday)
}

// TestTask_CountByList verifies that CountTasksByList returns the correct
// count of tasks in a given list.
func TestTask_CountByList(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "CountAgent")
	createTestTaskList(t, store, "count-list", agentID)

	// Initially empty.
	count, err := store.CountTasksByList(ctx, "count-list")
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// Add some tasks.
	for i := 0; i < 5; i++ {
		createTestTask(t, store, CreateTaskParams{
			AgentID: agentID, ListID: "count-list",
			ClaudeTaskID: "task-count-" + string(rune('a'+i)),
			Subject:      "Count task", Status: "pending",
			BlockedBy: "[]", Blocks: "[]",
		})
	}

	count, err = store.CountTasksByList(ctx, "count-list")
	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	// Nonexistent list has zero count.
	count, err = store.CountTasksByList(ctx, "no-such-list")
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

// TestTask_Delete verifies single task deletion, bulk deletion by list, and
// marking tasks as deleted by list.
func TestTask_Delete(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "DeleteAgent")
	createTestTaskList(t, store, "del-list", agentID)

	// Create tasks.
	task1 := createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "del-list",
		ClaudeTaskID: "task-del-1", Subject: "Delete me",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "del-list",
		ClaudeTaskID: "task-del-2", Subject: "Delete by list",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "del-list",
		ClaudeTaskID: "task-del-3", Subject: "Also delete by list",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})

	// Delete single task.
	err := store.DeleteTask(ctx, task1.ID)
	require.NoError(t, err)

	_, err = store.GetTask(ctx, task1.ID)
	require.Error(t, err)

	// Verify 2 remain.
	count, err := store.CountTasksByList(ctx, "del-list")
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	// Delete all tasks by list.
	err = store.DeleteTasksByList(ctx, "del-list")
	require.NoError(t, err)

	count, err = store.CountTasksByList(ctx, "del-list")
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// Test MarkTasksDeletedByList.
	createTestTaskList(t, store, "mark-list", agentID)

	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "mark-list",
		ClaudeTaskID: "task-keep", Subject: "Keep",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "mark-list",
		ClaudeTaskID: "task-remove", Subject: "Remove",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "mark-list",
		ClaudeTaskID: "task-completed", Subject: "Completed",
		Status: "completed", BlockedBy: "[]", Blocks: "[]",
	})

	// Mark tasks not in activeIDs as deleted. Completed tasks should be
	// excluded from deletion.
	now := time.Now()
	err = store.MarkTasksDeletedByList(
		ctx, "mark-list", []string{"task-keep"}, now,
	)
	require.NoError(t, err)

	// task-keep should remain pending.
	keepTask, err := store.GetTaskByClaudeID(
		ctx, "mark-list", "task-keep",
	)
	require.NoError(t, err)
	require.Equal(t, "pending", keepTask.Status)

	// task-remove should be marked as deleted.
	removeTask, err := store.GetTaskByClaudeID(
		ctx, "mark-list", "task-remove",
	)
	require.NoError(t, err)
	require.Equal(t, "deleted", removeTask.Status)

	// task-completed should remain completed.
	completedTask, err := store.GetTaskByClaudeID(
		ctx, "mark-list", "task-completed",
	)
	require.NoError(t, err)
	require.Equal(t, "completed", completedTask.Status)

	// Test MarkTasksDeletedByList with empty active IDs (all non-completed
	// and non-deleted tasks get deleted).
	createTestTaskList(t, store, "mark-all-list", agentID)
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "mark-all-list",
		ClaudeTaskID: "task-ma-1", Subject: "Pending to delete",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "mark-all-list",
		ClaudeTaskID: "task-ma-2", Subject: "In progress to delete",
		Status: "in_progress", BlockedBy: "[]", Blocks: "[]",
	})

	err = store.MarkTasksDeletedByList(ctx, "mark-all-list", nil, now)
	require.NoError(t, err)

	tasks, err := store.ListTasksByList(ctx, "mark-all-list")
	require.NoError(t, err)
	for _, task := range tasks {
		require.Equal(t, "deleted", task.Status)
	}
}

// TestTask_PruneOldTasks verifies that PruneOldTasks removes completed and
// deleted tasks older than the specified time while leaving newer tasks
// and active tasks intact.
func TestTask_PruneOldTasks(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "PruneAgent")
	createTestTaskList(t, store, "prune-list", agentID)

	// Create a pending task (should not be pruned).
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "prune-list",
		ClaudeTaskID: "task-active", Subject: "Active task",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})

	// Create a task, complete it so it gets a completed_at timestamp.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "prune-list",
		ClaudeTaskID: "task-old-complete", Subject: "Old completed",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	oldTime := time.Now().Add(-48 * time.Hour)
	err := store.UpdateTaskStatus(
		ctx, "prune-list", "task-old-complete", "completed", oldTime,
	)
	require.NoError(t, err)

	// Create another completed task with recent timestamp.
	createTestTask(t, store, CreateTaskParams{
		AgentID: agentID, ListID: "prune-list",
		ClaudeTaskID: "task-recent-complete", Subject: "Recent completed",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	recentTime := time.Now()
	err = store.UpdateTaskStatus(
		ctx, "prune-list", "task-recent-complete", "completed",
		recentTime,
	)
	require.NoError(t, err)

	// Prune tasks completed before 24 hours ago.
	cutoff := time.Now().Add(-24 * time.Hour)
	err = store.PruneOldTasks(ctx, cutoff)
	require.NoError(t, err)

	// Active task should still exist.
	_, err = store.GetTaskByClaudeID(ctx, "prune-list", "task-active")
	require.NoError(t, err)

	// Old completed task should be pruned.
	_, err = store.GetTaskByClaudeID(
		ctx, "prune-list", "task-old-complete",
	)
	require.Error(t, err)

	// Recent completed task should still exist.
	_, err = store.GetTaskByClaudeID(
		ctx, "prune-list", "task-recent-complete",
	)
	require.NoError(t, err)

	// Total remaining should be 2.
	count, err := store.CountTasksByList(ctx, "prune-list")
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

// TestTask_ListAllAndByStatus verifies ListAllTasks pagination and
// ListTasksByStatus filtering.
func TestTask_ListAllAndByStatus(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "AllAgent")
	createTestTaskList(t, store, "all-list", agentID)

	statuses := []string{
		"pending", "pending", "in_progress",
		"completed", "completed", "deleted",
	}
	for i, status := range statuses {
		createTestTask(t, store, CreateTaskParams{
			AgentID: agentID, ListID: "all-list",
			ClaudeTaskID: "task-all-" + string(rune('a'+i)),
			Subject:      "Task " + status, Status: status,
			BlockedBy: "[]", Blocks: "[]",
		})
	}

	// ListAllTasks with pagination.
	page1, err := store.ListAllTasks(ctx, 3, 0)
	require.NoError(t, err)
	require.Len(t, page1, 3)

	page2, err := store.ListAllTasks(ctx, 3, 3)
	require.NoError(t, err)
	require.Len(t, page2, 3)

	page3, err := store.ListAllTasks(ctx, 3, 6)
	require.NoError(t, err)
	require.Empty(t, page3)

	// Verify all 6 tasks can be collected across pages.
	allTasks, err := store.ListAllTasks(ctx, 100, 0)
	require.NoError(t, err)
	require.Len(t, allTasks, 6)

	// Verify ordering: in_progress first, then pending, then others.
	require.Equal(t, "in_progress", allTasks[0].Status)
	require.Equal(t, "pending", allTasks[1].Status)
	require.Equal(t, "pending", allTasks[2].Status)

	// ListTasksByStatus for pending.
	pendingTasks, err := store.ListTasksByStatus(ctx, "pending", 100, 0)
	require.NoError(t, err)
	require.Len(t, pendingTasks, 2)
	for _, task := range pendingTasks {
		require.Equal(t, "pending", task.Status)
	}

	// ListTasksByStatus for completed.
	completedTasks, err := store.ListTasksByStatus(
		ctx, "completed", 100, 0,
	)
	require.NoError(t, err)
	require.Len(t, completedTasks, 2)

	// ListTasksByStatus for in_progress.
	ipTasks, err := store.ListTasksByStatus(ctx, "in_progress", 100, 0)
	require.NoError(t, err)
	require.Len(t, ipTasks, 1)

	// ListTasksByStatus with pagination.
	limitedPending, err := store.ListTasksByStatus(ctx, "pending", 1, 0)
	require.NoError(t, err)
	require.Len(t, limitedPending, 1)

	offsetPending, err := store.ListTasksByStatus(ctx, "pending", 1, 1)
	require.NoError(t, err)
	require.Len(t, offsetPending, 1)

	// No results for status with empty set.
	noResults, err := store.ListTasksByStatus(ctx, "nonexistent", 100, 0)
	require.NoError(t, err)
	require.Empty(t, noResults)
}

// TestTask_RecentCompleted verifies that ListRecentCompletedTasks returns
// only completed tasks after the given time with the proper limit.
func TestTask_RecentCompleted(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "RecentAgent")
	createTestTaskList(t, store, "recent-list", agentID)

	// Create and complete tasks at different times.
	for i := 0; i < 3; i++ {
		createTestTask(t, store, CreateTaskParams{
			AgentID: agentID, ListID: "recent-list",
			ClaudeTaskID: "task-r-" + string(rune('a'+i)),
			Subject:      "Recent task", Status: "pending",
			BlockedBy: "[]", Blocks: "[]",
		})
	}

	// Complete first task 2 hours ago.
	oldComplete := time.Now().Add(-2 * time.Hour)
	err := store.UpdateTaskStatus(
		ctx, "recent-list", "task-r-a", "completed", oldComplete,
	)
	require.NoError(t, err)

	// Complete second task just now.
	recentComplete := time.Now()
	err = store.UpdateTaskStatus(
		ctx, "recent-list", "task-r-b", "completed", recentComplete,
	)
	require.NoError(t, err)

	// Third task remains pending.

	// Get recently completed tasks since 1 hour ago.
	since := time.Now().Add(-1 * time.Hour)
	recent, err := store.ListRecentCompletedTasks(
		ctx, agentID, since, 10,
	)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	require.Equal(t, "task-r-b", recent[0].ClaudeTaskID)

	// Get all completed tasks since 3 hours ago.
	allSince := time.Now().Add(-3 * time.Hour)
	allRecent, err := store.ListRecentCompletedTasks(
		ctx, agentID, allSince, 10,
	)
	require.NoError(t, err)
	require.Len(t, allRecent, 2)

	// Test limit.
	limited, err := store.ListRecentCompletedTasks(
		ctx, agentID, allSince, 1,
	)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

// TestTask_UniqueConstraint verifies that creating two tasks with the same
// list_id and claude_task_id pair fails due to the UNIQUE constraint.
func TestTask_UniqueConstraint(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "UniqueAgent")
	createTestTaskList(t, store, "unique-list", agentID)

	_, err := store.CreateTask(ctx, CreateTaskParams{
		AgentID: agentID, ListID: "unique-list",
		ClaudeTaskID: "task-dup", Subject: "Original",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	require.NoError(t, err)

	// Creating a task with the same list+claude ID should fail.
	_, err = store.CreateTask(ctx, CreateTaskParams{
		AgentID: agentID, ListID: "unique-list",
		ClaudeTaskID: "task-dup", Subject: "Duplicate",
		Status: "pending", BlockedBy: "[]", Blocks: "[]",
	})
	require.Error(t, err)
}

// TestTask_TaskListUniqueListID verifies that creating two task lists with the
// same list_id fails due to the UNIQUE constraint on list_id.
func TestTask_TaskListUniqueListID(t *testing.T) {
	t.Parallel()

	store, cleanup := testTaskDB(t)
	defer cleanup()

	ctx := context.Background()
	agentID := createTestAgent(t, store, "DupListAgent")

	_, err := store.CreateTaskList(ctx, CreateTaskListParams{
		ListID: "dup-list", AgentID: agentID,
		WatchPath: "/path/1",
	})
	require.NoError(t, err)

	_, err = store.CreateTaskList(ctx, CreateTaskListParams{
		ListID: "dup-list", AgentID: agentID,
		WatchPath: "/path/2",
	})
	require.Error(t, err)
}
