package task

import (
	"context"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
)

// newTestService creates a task service backed by a fresh mock store and
// returns both the service and the mock for pre-populating test data.
func newTestService(t *testing.T) (*Service, *store.MockStore) {
	t.Helper()

	ms := store.NewMockStore()
	svc := NewService(ServiceConfig{Store: ms})

	return svc, ms
}

// createAgent is a helper that creates a named agent in the mock store.
func createAgent(
	t *testing.T, ms *store.MockStore, name string,
) store.Agent {
	t.Helper()

	agent, err := ms.CreateAgent(
		context.Background(),
		store.CreateAgentParams{Name: name},
	)
	require.NoError(t, err)

	return agent
}

// registerList is a helper that registers a task list via the mock store.
func registerList(
	t *testing.T, ms *store.MockStore,
	listID string, agentID int64, watchPath string,
) store.TaskList {
	t.Helper()

	tl, err := ms.CreateTaskList(
		context.Background(),
		store.CreateTaskListParams{
			ListID:    listID,
			AgentID:   agentID,
			WatchPath: watchPath,
		},
	)
	require.NoError(t, err)

	return tl
}

// upsertTask is a helper that creates a task via the service and returns the
// response task.
func upsertTask(
	t *testing.T, svc *Service, agentID int64,
	listID, claudeID, subject, status string,
) Task {
	t.Helper()

	result := svc.Receive(context.Background(), UpsertTaskRequest{
		AgentID:      agentID,
		ListID:       listID,
		ClaudeTaskID: claudeID,
		Subject:      subject,
		Status:       status,
	})
	resp, err := result.Unpack()
	require.NoError(t, err)

	uResp := resp.(UpsertTaskResponse)
	require.NoError(t, uResp.Error)

	return uResp.Task
}

// TestService_RegisterTaskList verifies that registering a task list through
// the service actor returns the created list with correct fields.
func TestService_RegisterTaskList(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "register-agent")

	result := svc.Receive(ctx, RegisterTaskListRequest{
		ListID:    "list-reg-1",
		AgentID:   agent.ID,
		WatchPath: "/tmp/tasks/reg",
	})
	resp, err := result.Unpack()
	require.NoError(t, err)

	regResp := resp.(RegisterTaskListResponse)
	require.NoError(t, regResp.Error)
	require.Equal(t, "list-reg-1", regResp.TaskList.ListID)
	require.Equal(t, agent.ID, regResp.TaskList.AgentID)
	require.Equal(t, "/tmp/tasks/reg", regResp.TaskList.WatchPath)
	require.False(t, regResp.TaskList.CreatedAt.IsZero())
}

// TestService_GetTaskList verifies that a previously registered task list
// can be retrieved via the GetTaskListRequest message.
func TestService_GetTaskList(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "get-list-agent")

	// Register a list first.
	regResult := svc.Receive(ctx, RegisterTaskListRequest{
		ListID:    "list-get-1",
		AgentID:   agent.ID,
		WatchPath: "/tmp/tasks/get",
	})
	regResp, err := regResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, regResp.(RegisterTaskListResponse).Error)

	// Retrieve it.
	getResult := svc.Receive(ctx, GetTaskListRequest{
		ListID: "list-get-1",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)

	got := getResp.(GetTaskListResponse)
	require.NoError(t, got.Error)
	require.Equal(t, "list-get-1", got.TaskList.ListID)
	require.Equal(t, agent.ID, got.TaskList.AgentID)
	require.Equal(t, "/tmp/tasks/get", got.TaskList.WatchPath)
}

// TestService_ListTaskLists verifies listing all task lists and filtering
// by agent ID.
func TestService_ListTaskLists(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent1 := createAgent(t, ms, "list-agent-1")
	agent2 := createAgent(t, ms, "list-agent-2")

	// Register lists for both agents.
	registerList(t, ms, "list-a1-1", agent1.ID, "/tmp/a1/1")
	registerList(t, ms, "list-a1-2", agent1.ID, "/tmp/a1/2")
	registerList(t, ms, "list-a2-1", agent2.ID, "/tmp/a2/1")

	// List all task lists.
	allResult := svc.Receive(ctx, ListTaskListsRequest{})
	allResp, err := allResult.Unpack()
	require.NoError(t, err)

	allLists := allResp.(ListTaskListsResponse)
	require.NoError(t, allLists.Error)
	require.Len(t, allLists.TaskLists, 3)

	// List by agent1.
	a1Result := svc.Receive(ctx, ListTaskListsRequest{
		AgentID: agent1.ID,
	})
	a1Resp, err := a1Result.Unpack()
	require.NoError(t, err)

	a1Lists := a1Resp.(ListTaskListsResponse)
	require.NoError(t, a1Lists.Error)
	require.Len(t, a1Lists.TaskLists, 2)

	for _, tl := range a1Lists.TaskLists {
		require.Equal(t, agent1.ID, tl.AgentID)
	}

	// List by agent2.
	a2Result := svc.Receive(ctx, ListTaskListsRequest{
		AgentID: agent2.ID,
	})
	a2Resp, err := a2Result.Unpack()
	require.NoError(t, err)

	a2Lists := a2Resp.(ListTaskListsResponse)
	require.NoError(t, a2Lists.Error)
	require.Len(t, a2Lists.TaskLists, 1)
	require.Equal(t, agent2.ID, a2Lists.TaskLists[0].AgentID)
}

// TestService_UnregisterTaskList verifies that unregistering a task list
// removes both the list and its associated tasks.
func TestService_UnregisterTaskList(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "unreg-agent")
	registerList(t, ms, "list-unreg", agent.ID, "/tmp/unreg")

	// Create a task in the list.
	upsertTask(t, svc, agent.ID, "list-unreg", "task-1", "Do something", "pending")

	// Unregister the list.
	unregResult := svc.Receive(ctx, UnregisterTaskListRequest{
		ListID: "list-unreg",
	})
	unregResp, err := unregResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, unregResp.(UnregisterTaskListResponse).Error)

	// Verify the list is gone.
	getResult := svc.Receive(ctx, GetTaskListRequest{
		ListID: "list-unreg",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)
	require.Error(t, getResp.(GetTaskListResponse).Error)

	// Verify tasks in the list are gone.
	listResult := svc.Receive(ctx, ListTasksRequest{
		ListID: "list-unreg",
	})
	listResp, err := listResult.Unpack()
	require.NoError(t, err)

	tasksResp := listResp.(ListTasksResponse)
	require.NoError(t, tasksResp.Error)
	require.Empty(t, tasksResp.Tasks)
}

// TestService_UpsertTask verifies creating a new task via upsert and then
// updating it via a second upsert with the same claude task ID.
func TestService_UpsertTask(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "upsert-agent")
	registerList(t, ms, "list-upsert", agent.ID, "/tmp/upsert")

	// Create a task.
	createResult := svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent.ID,
		ListID:       "list-upsert",
		ClaudeTaskID: "claude-1",
		Subject:      "Original subject",
		Description:  "Original description",
		Status:       "pending",
		BlockedBy:    []string{"dep-1"},
		Blocks:       []string{"dep-2"},
	})
	createResp, err := createResult.Unpack()
	require.NoError(t, err)

	created := createResp.(UpsertTaskResponse)
	require.NoError(t, created.Error)
	require.Equal(t, "Original subject", created.Task.Subject)
	require.Equal(t, "Original description", created.Task.Description)
	require.Equal(t, "pending", created.Task.Status)
	require.Equal(t, []string{"dep-1"}, created.Task.BlockedBy)
	require.Equal(t, []string{"dep-2"}, created.Task.Blocks)
	require.NotZero(t, created.Task.ID)

	originalID := created.Task.ID

	// Update the same task via upsert (same ListID + ClaudeTaskID).
	updateResult := svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent.ID,
		ListID:       "list-upsert",
		ClaudeTaskID: "claude-1",
		Subject:      "Updated subject",
		Description:  "Updated description",
		Status:       "in_progress",
	})
	updateResp, err := updateResult.Unpack()
	require.NoError(t, err)

	updated := updateResp.(UpsertTaskResponse)
	require.NoError(t, updated.Error)
	require.Equal(t, originalID, updated.Task.ID)
	require.Equal(t, "Updated subject", updated.Task.Subject)
	require.Equal(t, "Updated description", updated.Task.Description)
	require.Equal(t, "in_progress", updated.Task.Status)
}

// TestService_GetTask verifies retrieving a task by its claude task ID.
func TestService_GetTask(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "get-task-agent")
	registerList(t, ms, "list-get-task", agent.ID, "/tmp/get-task")

	// Create a task.
	created := upsertTask(
		t, svc, agent.ID, "list-get-task",
		"claude-get", "Get me", "pending",
	)

	// Retrieve it.
	getResult := svc.Receive(ctx, GetTaskRequest{
		ListID:       "list-get-task",
		ClaudeTaskID: "claude-get",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)

	got := getResp.(GetTaskResponse)
	require.NoError(t, got.Error)
	require.Equal(t, created.ID, got.Task.ID)
	require.Equal(t, "Get me", got.Task.Subject)
	require.Equal(t, "pending", got.Task.Status)
}

// TestService_ListTasks verifies the various filter combinations on the
// ListTasksRequest: by agent, by list, by status, active only, and
// available only.
func TestService_ListTasks(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()

	agent1 := createAgent(t, ms, "list-tasks-agent-1")
	agent2 := createAgent(t, ms, "list-tasks-agent-2")
	registerList(t, ms, "lt-list-1", agent1.ID, "/tmp/lt1")
	registerList(t, ms, "lt-list-2", agent2.ID, "/tmp/lt2")

	// Agent 1 tasks in list-1.
	upsertTask(t, svc, agent1.ID, "lt-list-1", "t1", "Pending available", "pending")
	upsertTask(t, svc, agent1.ID, "lt-list-1", "t2", "In progress", "in_progress")
	upsertTask(t, svc, agent1.ID, "lt-list-1", "t3", "Completed", "completed")

	// Create a pending task with an owner (not available).
	ownedResult := svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent1.ID,
		ListID:       "lt-list-1",
		ClaudeTaskID: "t4",
		Subject:      "Owned pending",
		Status:       "pending",
		Owner:        "someone",
	})
	ownedResp, err := ownedResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, ownedResp.(UpsertTaskResponse).Error)

	// Create a blocked task (not available).
	blockedResult := svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent1.ID,
		ListID:       "lt-list-1",
		ClaudeTaskID: "t5",
		Subject:      "Blocked pending",
		Status:       "pending",
		BlockedBy:    []string{"t1"},
	})
	blockedResp, err := blockedResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, blockedResp.(UpsertTaskResponse).Error)

	// Agent 2 tasks in list-2.
	upsertTask(t, svc, agent2.ID, "lt-list-2", "t6", "Agent2 task", "pending")

	// Sub-test: list by agent.
	t.Run("by_agent", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{
			AgentID: agent1.ID,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)
		require.Len(t, tasks.Tasks, 5)
	})

	// Sub-test: list by list ID.
	t.Run("by_list", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{
			ListID: "lt-list-1",
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)
		require.Len(t, tasks.Tasks, 5)
	})

	// Sub-test: list by status.
	t.Run("by_status", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{
			Status: "pending",
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)

		// 3 pending from agent1 (t1, t4, t5) + 1 from agent2 (t6).
		require.Len(t, tasks.Tasks, 4)
	})

	// Sub-test: active only (pending + in_progress) for agent1.
	t.Run("active_only", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{
			AgentID:    agent1.ID,
			ActiveOnly: true,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)

		// t1 (pending), t2 (in_progress), t4 (pending), t5 (pending).
		require.Len(t, tasks.Tasks, 4)
		for _, task := range tasks.Tasks {
			require.Contains(
				t, []string{"pending", "in_progress"}, task.Status,
			)
		}
	})

	// Sub-test: available only (pending, no owner, no blockers).
	t.Run("available_only", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{
			AgentID:       agent1.ID,
			AvailableOnly: true,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)

		// Only t1 is pending with no owner and no blockers.
		require.Len(t, tasks.Tasks, 1)
		require.Equal(t, "Pending available", tasks.Tasks[0].Subject)
	})

	// Sub-test: default (list all) when no filters set.
	t.Run("list_all", func(t *testing.T) {
		result := svc.Receive(ctx, ListTasksRequest{})
		resp, err := result.Unpack()
		require.NoError(t, err)

		tasks := resp.(ListTasksResponse)
		require.NoError(t, tasks.Error)
		require.Len(t, tasks.Tasks, 6)
	})
}

// TestService_UpdateTaskStatus verifies that updating a task's status
// through the service persists correctly and can be read back.
func TestService_UpdateTaskStatus(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "status-agent")
	registerList(t, ms, "list-status", agent.ID, "/tmp/status")

	// Create a pending task.
	upsertTask(t, svc, agent.ID, "list-status", "status-1", "Status task", "pending")

	// Update to in_progress.
	updateResult := svc.Receive(ctx, UpdateTaskStatusRequest{
		ListID:       "list-status",
		ClaudeTaskID: "status-1",
		Status:       "in_progress",
	})
	updateResp, err := updateResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, updateResp.(UpdateTaskStatusResponse).Error)

	// Verify via get.
	getResult := svc.Receive(ctx, GetTaskRequest{
		ListID:       "list-status",
		ClaudeTaskID: "status-1",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)

	got := getResp.(GetTaskResponse)
	require.NoError(t, got.Error)
	require.Equal(t, "in_progress", got.Task.Status)

	// Update to completed.
	compResult := svc.Receive(ctx, UpdateTaskStatusRequest{
		ListID:       "list-status",
		ClaudeTaskID: "status-1",
		Status:       "completed",
	})
	compResp, err := compResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, compResp.(UpdateTaskStatusResponse).Error)

	// Verify completed state.
	getResult2 := svc.Receive(ctx, GetTaskRequest{
		ListID:       "list-status",
		ClaudeTaskID: "status-1",
	})
	getResp2, err := getResult2.Unpack()
	require.NoError(t, err)

	got2 := getResp2.(GetTaskResponse)
	require.NoError(t, got2.Error)
	require.Equal(t, "completed", got2.Task.Status)
}

// TestService_UpdateTaskOwner verifies that assigning an owner to a task
// is persisted and can be read back.
func TestService_UpdateTaskOwner(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "owner-agent")
	registerList(t, ms, "list-owner", agent.ID, "/tmp/owner")

	// Create a task.
	upsertTask(t, svc, agent.ID, "list-owner", "owner-1", "Owner task", "pending")

	// Set owner.
	ownerResult := svc.Receive(ctx, UpdateTaskOwnerRequest{
		ListID:       "list-owner",
		ClaudeTaskID: "owner-1",
		Owner:        "alice",
	})
	ownerResp, err := ownerResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, ownerResp.(UpdateTaskOwnerResponse).Error)

	// Verify via get.
	getResult := svc.Receive(ctx, GetTaskRequest{
		ListID:       "list-owner",
		ClaudeTaskID: "owner-1",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)

	got := getResp.(GetTaskResponse)
	require.NoError(t, got.Error)
	require.Equal(t, "alice", got.Task.Owner)
}

// TestService_DeleteTask verifies that deleting a task removes it from
// the store.
func TestService_DeleteTask(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "delete-agent")
	registerList(t, ms, "list-delete", agent.ID, "/tmp/delete")

	// Create a task.
	created := upsertTask(
		t, svc, agent.ID, "list-delete",
		"del-1", "Delete me", "pending",
	)

	// Delete it.
	delResult := svc.Receive(ctx, DeleteTaskRequest{ID: created.ID})
	delResp, err := delResult.Unpack()
	require.NoError(t, err)
	require.NoError(t, delResp.(DeleteTaskResponse).Error)

	// Verify it is gone.
	getResult := svc.Receive(ctx, GetTaskRequest{
		ListID:       "list-delete",
		ClaudeTaskID: "del-1",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)
	require.Error(t, getResp.(GetTaskResponse).Error)
}

// TestService_GetTaskStats verifies that task statistics are correctly
// computed by agent, by list, and globally.
func TestService_GetTaskStats(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()

	agent := createAgent(t, ms, "stats-agent")
	registerList(t, ms, "list-stats-1", agent.ID, "/tmp/stats1")
	registerList(t, ms, "list-stats-2", agent.ID, "/tmp/stats2")

	todaySince := time.Now().Add(-24 * time.Hour)

	// Create tasks in list-stats-1.
	upsertTask(t, svc, agent.ID, "list-stats-1", "s1", "Pending", "pending")
	upsertTask(t, svc, agent.ID, "list-stats-1", "s2", "In progress", "in_progress")

	// Create a completed task (need to set completed status via update so
	// CompletedAt gets set).
	upsertTask(t, svc, agent.ID, "list-stats-1", "s3", "Completed", "pending")
	svc.Receive(ctx, UpdateTaskStatusRequest{
		ListID:       "list-stats-1",
		ClaudeTaskID: "s3",
		Status:       "completed",
	})

	// Create a blocked task.
	svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent.ID,
		ListID:       "list-stats-1",
		ClaudeTaskID: "s4",
		Subject:      "Blocked",
		Status:       "pending",
		BlockedBy:    []string{"s1"},
	})

	// Tasks in list-stats-2.
	upsertTask(t, svc, agent.ID, "list-stats-2", "s5", "Another pending", "pending")

	// Sub-test: stats by agent.
	t.Run("by_agent", func(t *testing.T) {
		result := svc.Receive(ctx, GetTaskStatsRequest{
			AgentID:    agent.ID,
			TodaySince: todaySince,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		stats := resp.(GetTaskStatsResponse)
		require.NoError(t, stats.Error)

		// s1 (pending available), s4 (pending blocked), s5 (pending
		// available).
		require.Equal(t, int64(3), stats.Stats.PendingCount)
		require.Equal(t, int64(1), stats.Stats.InProgressCount)
		require.Equal(t, int64(1), stats.Stats.CompletedCount)
		require.Equal(t, int64(1), stats.Stats.BlockedCount)
		require.Equal(t, int64(2), stats.Stats.AvailableCount)
		require.Equal(t, int64(1), stats.Stats.CompletedToday)
	})

	// Sub-test: stats by list.
	t.Run("by_list", func(t *testing.T) {
		result := svc.Receive(ctx, GetTaskStatsRequest{
			ListID:     "list-stats-1",
			TodaySince: todaySince,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		stats := resp.(GetTaskStatsResponse)
		require.NoError(t, stats.Error)
		require.Equal(t, int64(2), stats.Stats.PendingCount)
		require.Equal(t, int64(1), stats.Stats.InProgressCount)
		require.Equal(t, int64(1), stats.Stats.CompletedCount)
	})

	// Sub-test: global stats.
	t.Run("global", func(t *testing.T) {
		result := svc.Receive(ctx, GetTaskStatsRequest{
			TodaySince: todaySince,
		})
		resp, err := result.Unpack()
		require.NoError(t, err)

		stats := resp.(GetTaskStatsResponse)
		require.NoError(t, stats.Error)
		require.Equal(t, int64(3), stats.Stats.PendingCount)
		require.Equal(t, int64(1), stats.Stats.InProgressCount)
		require.Equal(t, int64(1), stats.Stats.CompletedCount)
	})
}

// TestService_GetAllAgentTaskStats verifies that per-agent task statistics
// are returned for multiple agents.
func TestService_GetAllAgentTaskStats(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()

	agent1 := createAgent(t, ms, "multi-stats-1")
	agent2 := createAgent(t, ms, "multi-stats-2")
	registerList(t, ms, "ms-list-1", agent1.ID, "/tmp/ms1")
	registerList(t, ms, "ms-list-2", agent2.ID, "/tmp/ms2")

	todaySince := time.Now().Add(-24 * time.Hour)

	// Agent1 tasks.
	upsertTask(t, svc, agent1.ID, "ms-list-1", "m1", "Pending", "pending")
	upsertTask(t, svc, agent1.ID, "ms-list-1", "m2", "In progress", "in_progress")

	// Agent2 tasks.
	upsertTask(t, svc, agent2.ID, "ms-list-2", "m3", "Pending", "pending")

	// Create a blocked task for agent2.
	svc.Receive(ctx, UpsertTaskRequest{
		AgentID:      agent2.ID,
		ListID:       "ms-list-2",
		ClaudeTaskID: "m4",
		Subject:      "Blocked",
		Status:       "pending",
		BlockedBy:    []string{"m3"},
	})

	result := svc.Receive(ctx, GetAllAgentTaskStatsRequest{
		TodaySince: todaySince,
	})
	resp, err := result.Unpack()
	require.NoError(t, err)

	allStats := resp.(GetAllAgentTaskStatsResponse)
	require.NoError(t, allStats.Error)
	require.Len(t, allStats.Stats, 2)

	// Build a map for easier assertion.
	statsByAgent := make(map[int64]AgentTaskStats)
	for _, s := range allStats.Stats {
		statsByAgent[s.AgentID] = s
	}

	// Agent1: 1 pending, 1 in_progress.
	a1Stats := statsByAgent[agent1.ID]
	require.Equal(t, int64(1), a1Stats.PendingCount)
	require.Equal(t, int64(1), a1Stats.InProgressCount)

	// Agent2: 2 pending (one blocked).
	a2Stats := statsByAgent[agent2.ID]
	require.Equal(t, int64(2), a2Stats.PendingCount)
	require.Equal(t, int64(1), a2Stats.BlockedCount)
}

// TestService_SyncTaskList verifies that the sync operation completes
// without error and returns zero counters for the current stub
// implementation.
func TestService_SyncTaskList(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "sync-agent")
	registerList(t, ms, "list-sync", agent.ID, "/tmp/sync")

	result := svc.Receive(ctx, SyncTaskListRequest{
		ListID: "list-sync",
	})
	resp, err := result.Unpack()
	require.NoError(t, err)

	syncResp := resp.(SyncTaskListResponse)
	require.NoError(t, syncResp.Error)
	require.Equal(t, 0, syncResp.TasksUpdated)
	require.Equal(t, 0, syncResp.TasksDeleted)

	// Verify sync time was updated on the list.
	getResult := svc.Receive(ctx, GetTaskListRequest{
		ListID: "list-sync",
	})
	getResp, err := getResult.Unpack()
	require.NoError(t, err)

	got := getResp.(GetTaskListResponse)
	require.NoError(t, got.Error)
	require.NotNil(t, got.TaskList.LastSyncedAt)
}

// TestService_PruneOldTasks verifies that the prune operation completes
// without error.
func TestService_PruneOldTasks(t *testing.T) {
	t.Parallel()

	svc, ms := newTestService(t)
	ctx := context.Background()
	agent := createAgent(t, ms, "prune-agent")
	registerList(t, ms, "list-prune", agent.ID, "/tmp/prune")

	// Create a completed task.
	upsertTask(t, svc, agent.ID, "list-prune", "prune-1", "Old completed", "pending")
	svc.Receive(ctx, UpdateTaskStatusRequest{
		ListID:       "list-prune",
		ClaudeTaskID: "prune-1",
		Status:       "completed",
	})

	// Prune tasks older than far in the future (should prune everything
	// completed).
	result := svc.Receive(ctx, PruneOldTasksRequest{
		OlderThan: time.Now().Add(1 * time.Hour),
	})
	resp, err := result.Unpack()
	require.NoError(t, err)
	require.NoError(t, resp.(PruneOldTasksResponse).Error)

	// Verify the completed task was pruned.
	listResult := svc.Receive(ctx, ListTasksRequest{
		ListID: "list-prune",
	})
	listResp, err := listResult.Unpack()
	require.NoError(t, err)

	tasks := listResp.(ListTasksResponse)
	require.NoError(t, tasks.Error)
	require.Empty(t, tasks.Tasks)
}

// TestService_Receive_UnknownMessage verifies that the Receive method
// returns an error for an unrecognized message type.
func TestService_Receive_UnknownMessage(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t)
	ctx := context.Background()

	result := svc.Receive(ctx, unknownMessage{})
	_, err := result.Unpack()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown message type")
}

// unknownMessage is a message type not handled by the service, used to
// test the default branch in Receive. It embeds actor.BaseMessage to
// satisfy the unexported messageMarker method.
type unknownMessage struct {
	actor.BaseMessage
}

// MessageType implements actor.Message.
func (unknownMessage) MessageType() string { return "unknownMessage" }

// isTaskRequest satisfies the TaskRequest sealed interface.
func (unknownMessage) isTaskRequest() {}
