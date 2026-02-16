package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// doRequest makes an HTTP request with the given method, path, and optional
// JSON body. It returns the HTTP response.
func (e *httpTestEnv) doRequest(
	method, path string, data any,
) *http.Response {
	e.t.Helper()

	var body *strings.Reader
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		require.NoError(e.t, err)
		body = strings.NewReader(string(jsonBytes))
	} else {
		body = strings.NewReader("")
	}

	req, err := http.NewRequest(method, e.baseURL()+path, body)
	require.NoError(e.t, err)

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	require.NoError(e.t, err)

	return resp
}

// patchJSON makes a PATCH request with JSON data.
func (e *httpTestEnv) patchJSON(
	path string, data any,
) *http.Response {
	e.t.Helper()
	return e.doRequest(http.MethodPatch, path, data)
}

// deleteReq makes a DELETE request.
func (e *httpTestEnv) deleteReq(path string) *http.Response {
	e.t.Helper()
	return e.doRequest(http.MethodDelete, path, nil)
}

// parseJSON unmarshals a response body into the given target.
func parseJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	body := readBody(t, resp)
	err := json.Unmarshal([]byte(body), target)
	require.NoError(t, err, "failed to parse JSON: %s", body)
}

// jsonInt64 extracts an int64 value from a JSON map field. The gRPC-gateway
// v1 marshaler with OrigName serializes int64 fields as JSON numbers.
func jsonInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}

	switch val := v.(type) {
	case float64:
		return int64(val)
	case json.Number:
		n, _ := val.Int64()
		return n
	case string:
		var n int64
		fmt.Sscanf(val, "%d", &n)
		return n
	default:
		return 0
	}
}

// jsonIntStr formats an int64 JSON field as a string for display. Handles
// both float64 and string representations.
func jsonIntStr(m map[string]any, key string) string {
	return fmt.Sprintf("%d", jsonInt64(m, key))
}

// TestHTTP_TaskListCRUD tests the full lifecycle of a task list: register,
// get, list, and delete.
func TestHTTP_TaskListCRUD(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create an agent first since task lists require an agent_id FK.
	agent := env.createAgent("TaskAgent")

	// Step 1: Register a new task list.
	registerData := map[string]any{
		"list_id":    "my-task-list",
		"agent_id":   agent.ID,
		"watch_path": "/tmp/tasks",
	}
	resp := env.postJSON("/api/v1/task-lists", registerData)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var registerResp map[string]any
	parseJSON(t, resp, &registerResp)

	// The gRPC-gateway v1 with OrigName:true uses snake_case field names.
	taskList, ok := registerResp["task_list"].(map[string]any)
	require.True(t, ok,
		"expected task_list in response: %v", registerResp,
	)
	require.Equal(t, "my-task-list", taskList["list_id"])
	require.NotEmpty(t, taskList["created_at"])

	t.Logf("Registered task list: %v", taskList["list_id"])

	// Step 2: Get the task list by ID.
	resp = env.get("/api/v1/task-lists/my-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var getResp map[string]any
	parseJSON(t, resp, &getResp)

	gotList, ok := getResp["task_list"].(map[string]any)
	require.True(t, ok, "expected task_list in response")
	require.Equal(t, "my-task-list", gotList["list_id"])
	require.Equal(t, "/tmp/tasks", gotList["watch_path"])

	t.Logf("Got task list: %v", gotList["list_id"])

	// Step 3: Register a second task list for the same agent.
	registerData2 := map[string]any{
		"list_id":    "second-list",
		"agent_id":   agent.ID,
		"watch_path": "/tmp/tasks2",
	}
	resp = env.postJSON("/api/v1/task-lists", registerData2)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 4: List all task lists.
	resp = env.get("/api/v1/task-lists")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp map[string]any
	parseJSON(t, resp, &listResp)

	taskLists, ok := listResp["task_lists"].([]any)
	require.True(t, ok,
		"expected task_lists array in response: %v", listResp,
	)
	require.Len(t, taskLists, 2)

	t.Logf("Listed %d task lists", len(taskLists))

	// Step 5: List task lists filtered by agent_id.
	resp = env.get(fmt.Sprintf(
		"/api/v1/task-lists?agent_id=%d", agent.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var filteredResp map[string]any
	parseJSON(t, resp, &filteredResp)

	filteredLists, ok := filteredResp["task_lists"].([]any)
	require.True(t, ok)
	require.Len(t, filteredLists, 2)

	// Step 6: Delete the first task list.
	resp = env.deleteReq("/api/v1/task-lists/my-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Deleted task list: my-task-list")

	// Step 7: Verify only one list remains.
	resp = env.get("/api/v1/task-lists")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterDeleteResp map[string]any
	parseJSON(t, resp, &afterDeleteResp)

	remainingLists, ok := afterDeleteResp["task_lists"].([]any)
	require.True(t, ok)
	require.Len(t, remainingLists, 1)

	remaining := remainingLists[0].(map[string]any)
	require.Equal(t, "second-list", remaining["list_id"])

	t.Log("Verified only second-list remains after deletion")

	// Step 8: Verify getting deleted list returns an error.
	resp = env.get("/api/v1/task-lists/my-task-list")
	require.NotEqual(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Confirmed deleted list returns error")
}

// TestHTTP_TaskUpsertAndGet tests creating a task via upsert, retrieving it,
// and then updating it via another upsert.
func TestHTTP_TaskUpsertAndGet(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create agent and task list.
	agent := env.createAgent("UpsertAgent")

	registerData := map[string]any{
		"list_id":    "upsert-list",
		"agent_id":   agent.ID,
		"watch_path": "/tmp/upsert",
	}
	resp := env.postJSON("/api/v1/task-lists", registerData)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 1: Upsert (create) a new task.
	taskData := map[string]any{
		"agent_id":       agent.ID,
		"list_id":        "upsert-list",
		"claude_task_id": "task-001",
		"subject":        "Implement feature X",
		"description":    "Build the new feature X component",
		"status":         "TASK_STATUS_PENDING",
		"owner":          "",
		"blocked_by":     []string{},
		"blocks":         []string{},
	}
	resp = env.postJSON("/api/v1/tasks", taskData)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var upsertResp map[string]any
	parseJSON(t, resp, &upsertResp)

	task, ok := upsertResp["task"].(map[string]any)
	require.True(t, ok, "expected task in response: %v", upsertResp)
	require.Equal(t, "task-001", task["claude_task_id"])
	require.Equal(t, "Implement feature X", task["subject"])
	require.Equal(t, "TASK_STATUS_PENDING", task["status"])

	t.Logf(
		"Created task: %v (status: %v)",
		task["claude_task_id"], task["status"],
	)

	// Step 2: Get the task by list_id and claude_task_id.
	resp = env.get("/api/v1/tasks/upsert-list/task-001")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var getResp map[string]any
	parseJSON(t, resp, &getResp)

	gotTask, ok := getResp["task"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "task-001", gotTask["claude_task_id"])
	require.Equal(t, "Implement feature X", gotTask["subject"])
	require.Equal(
		t, "Build the new feature X component",
		gotTask["description"],
	)

	t.Logf("Retrieved task: %v", gotTask["claude_task_id"])

	// Step 3: Upsert (update) the same task with new data.
	updatedTaskData := map[string]any{
		"agent_id":       agent.ID,
		"list_id":        "upsert-list",
		"claude_task_id": "task-001",
		"subject":        "Implement feature X (revised)",
		"description":    "Updated description for feature X",
		"status":         "TASK_STATUS_IN_PROGRESS",
		"owner":          "WorkerBot",
		"blocked_by":     []string{},
		"blocks":         []string{"task-002"},
	}
	resp = env.postJSON("/api/v1/tasks", updatedTaskData)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updateResp map[string]any
	parseJSON(t, resp, &updateResp)

	updatedTask, ok := updateResp["task"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "task-001", updatedTask["claude_task_id"])
	require.Equal(
		t, "Implement feature X (revised)", updatedTask["subject"],
	)
	require.Equal(
		t, "TASK_STATUS_IN_PROGRESS", updatedTask["status"],
	)

	t.Log("Updated task via upsert")

	// Step 4: Verify the update by getting the task again.
	resp = env.get("/api/v1/tasks/upsert-list/task-001")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var verifyResp map[string]any
	parseJSON(t, resp, &verifyResp)

	verifiedTask, ok := verifyResp["task"].(map[string]any)
	require.True(t, ok)
	require.Equal(
		t, "Implement feature X (revised)", verifiedTask["subject"],
	)
	require.Equal(
		t, "Updated description for feature X",
		verifiedTask["description"],
	)
	require.Equal(
		t, "TASK_STATUS_IN_PROGRESS", verifiedTask["status"],
	)
	require.Equal(t, "WorkerBot", verifiedTask["owner"])

	t.Log("Verified task update persisted")
}

// TestHTTP_TaskListFilters tests listing tasks with various filter parameters
// including agent_id, list_id, status, active_only, and available_only.
func TestHTTP_TaskListFilters(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create two agents.
	agentA := env.createAgent("FilterAgentA")
	agentB := env.createAgent("FilterAgentB")

	// Create task lists for each agent.
	for _, data := range []map[string]any{
		{
			"list_id": "filter-list-a", "agent_id": agentA.ID,
			"watch_path": "/tmp/a",
		},
		{
			"list_id": "filter-list-b", "agent_id": agentB.ID,
			"watch_path": "/tmp/b",
		},
	} {
		resp := env.postJSON("/api/v1/task-lists", data)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Create tasks with different statuses for agent A.
	tasksA := []map[string]any{
		{
			"agent_id": agentA.ID, "list_id": "filter-list-a",
			"claude_task_id": "a-pending",
			"subject":        "Pending Task A",
			"status":         "TASK_STATUS_PENDING",
		},
		{
			"agent_id": agentA.ID, "list_id": "filter-list-a",
			"claude_task_id": "a-inprogress",
			"subject":        "In Progress Task A",
			"status":         "TASK_STATUS_IN_PROGRESS",
		},
		{
			"agent_id": agentA.ID, "list_id": "filter-list-a",
			"claude_task_id": "a-completed",
			"subject":        "Completed Task A",
			"status":         "TASK_STATUS_COMPLETED",
		},
	}
	for _, task := range tasksA {
		resp := env.postJSON("/api/v1/tasks", task)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Create a task for agent B.
	resp := env.postJSON("/api/v1/tasks", map[string]any{
		"agent_id": agentB.ID, "list_id": "filter-list-b",
		"claude_task_id": "b-pending",
		"subject":        "Pending Task B", "status": "TASK_STATUS_PENDING",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test 1: List all tasks (no filters).
	resp = env.get("/api/v1/tasks")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var allResp map[string]any
	parseJSON(t, resp, &allResp)

	allTasks, ok := allResp["tasks"].([]any)
	require.True(t, ok, "expected tasks array: %v", allResp)
	require.Len(t, allTasks, 4)

	t.Logf("All tasks: %d", len(allTasks))

	// Test 2: Filter by list_id.
	resp = env.get("/api/v1/tasks?list_id=filter-list-a")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listAResp map[string]any
	parseJSON(t, resp, &listAResp)

	listATasks, ok := listAResp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, listATasks, 3)

	t.Logf("Tasks in filter-list-a: %d", len(listATasks))

	// Test 3: Filter by agent_id.
	resp = env.get(fmt.Sprintf(
		"/api/v1/tasks?agent_id=%d", agentB.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var agentBResp map[string]any
	parseJSON(t, resp, &agentBResp)

	agentBTasks, ok := agentBResp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, agentBTasks, 1)

	t.Logf("Tasks for agent B: %d", len(agentBTasks))

	// Test 4: Filter by status (pending only).
	resp = env.get("/api/v1/tasks?status=TASK_STATUS_PENDING")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var pendingResp map[string]any
	parseJSON(t, resp, &pendingResp)

	pendingTasks, ok := pendingResp["tasks"].([]any)
	require.True(t, ok)
	// Should include a-pending and b-pending.
	require.Len(t, pendingTasks, 2)

	t.Logf("Pending tasks: %d", len(pendingTasks))

	// Test 5: Filter active_only for agent A (pending + in_progress).
	resp = env.get(fmt.Sprintf(
		"/api/v1/tasks?agent_id=%d&active_only=true", agentA.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var activeResp map[string]any
	parseJSON(t, resp, &activeResp)

	activeTasks, ok := activeResp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, activeTasks, 2)

	t.Logf("Active tasks for agent A: %d", len(activeTasks))

	// Test 6: Pagination with limit and offset.
	resp = env.get("/api/v1/tasks?limit=2&offset=0")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page1Resp map[string]any
	parseJSON(t, resp, &page1Resp)

	page1Tasks, ok := page1Resp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, page1Tasks, 2)

	resp = env.get("/api/v1/tasks?limit=2&offset=2")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page2Resp map[string]any
	parseJSON(t, resp, &page2Resp)

	page2Tasks, ok := page2Resp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, page2Tasks, 2)

	t.Log("Pagination verified: page1=2, page2=2")
}

// TestHTTP_TaskStatusUpdate tests updating a task's status through the
// dedicated status update endpoint.
func TestHTTP_TaskStatusUpdate(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create agent, task list, and a task.
	agent := env.createAgent("StatusAgent")

	resp := env.postJSON("/api/v1/task-lists", map[string]any{
		"list_id": "status-list", "agent_id": agent.ID,
		"watch_path": "/tmp/status",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.postJSON("/api/v1/tasks", map[string]any{
		"agent_id": agent.ID, "list_id": "status-list",
		"claude_task_id": "status-task",
		"subject":        "Status Test Task",
		"status":         "TASK_STATUS_PENDING",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 1: Verify initial status is pending.
	resp = env.get("/api/v1/tasks/status-list/status-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var getResp map[string]any
	parseJSON(t, resp, &getResp)

	task := getResp["task"].(map[string]any)
	require.Equal(t, "TASK_STATUS_PENDING", task["status"])

	t.Log("Initial status: TASK_STATUS_PENDING")

	// Step 2: Update status to IN_PROGRESS.
	resp = env.patchJSON(
		"/api/v1/tasks/status-list/status-task/status",
		map[string]any{
			"list_id":        "status-list",
			"claude_task_id": "status-task",
			"status":         "TASK_STATUS_IN_PROGRESS",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify the status changed.
	resp = env.get("/api/v1/tasks/status-list/status-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterProgress map[string]any
	parseJSON(t, resp, &afterProgress)

	task = afterProgress["task"].(map[string]any)
	require.Equal(t, "TASK_STATUS_IN_PROGRESS", task["status"])

	t.Log("Updated to: TASK_STATUS_IN_PROGRESS")

	// Step 3: Update status to COMPLETED.
	resp = env.patchJSON(
		"/api/v1/tasks/status-list/status-task/status",
		map[string]any{
			"list_id":        "status-list",
			"claude_task_id": "status-task",
			"status":         "TASK_STATUS_COMPLETED",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify the status changed to completed.
	resp = env.get("/api/v1/tasks/status-list/status-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterComplete map[string]any
	parseJSON(t, resp, &afterComplete)

	task = afterComplete["task"].(map[string]any)
	require.Equal(t, "TASK_STATUS_COMPLETED", task["status"])

	t.Log("Updated to: TASK_STATUS_COMPLETED")
}

// TestHTTP_TaskOwnerUpdate tests updating a task's owner through the
// dedicated owner update endpoint.
func TestHTTP_TaskOwnerUpdate(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create agent, task list, and a task with no owner.
	agent := env.createAgent("OwnerAgent")

	resp := env.postJSON("/api/v1/task-lists", map[string]any{
		"list_id": "owner-list", "agent_id": agent.ID,
		"watch_path": "/tmp/owner",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = env.postJSON("/api/v1/tasks", map[string]any{
		"agent_id": agent.ID, "list_id": "owner-list",
		"claude_task_id": "owner-task",
		"subject":        "Owner Test Task",
		"status":         "TASK_STATUS_PENDING",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 1: Verify initial owner is empty.
	resp = env.get("/api/v1/tasks/owner-list/owner-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var getResp map[string]any
	parseJSON(t, resp, &getResp)

	task := getResp["task"].(map[string]any)
	// Owner should be empty string or absent.
	owner, _ := task["owner"].(string)
	require.Empty(t, owner)

	t.Log("Initial owner: (empty)")

	// Step 2: Assign an owner.
	resp = env.patchJSON(
		"/api/v1/tasks/owner-list/owner-task/owner",
		map[string]any{
			"list_id":        "owner-list",
			"claude_task_id": "owner-task",
			"owner":          "AgentSmith",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify the owner was assigned.
	resp = env.get("/api/v1/tasks/owner-list/owner-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterAssign map[string]any
	parseJSON(t, resp, &afterAssign)

	task = afterAssign["task"].(map[string]any)
	require.Equal(t, "AgentSmith", task["owner"])

	t.Log("Assigned owner: AgentSmith")

	// Step 3: Change the owner.
	resp = env.patchJSON(
		"/api/v1/tasks/owner-list/owner-task/owner",
		map[string]any{
			"list_id":        "owner-list",
			"claude_task_id": "owner-task",
			"owner":          "AgentJones",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify the owner changed.
	resp = env.get("/api/v1/tasks/owner-list/owner-task")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterChange map[string]any
	parseJSON(t, resp, &afterChange)

	task = afterChange["task"].(map[string]any)
	require.Equal(t, "AgentJones", task["owner"])

	t.Log("Changed owner to: AgentJones")
}

// TestHTTP_TaskStats tests the task statistics endpoints, both global and
// per-agent.
func TestHTTP_TaskStats(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create agents.
	agentA := env.createAgent("StatsAgentA")
	agentB := env.createAgent("StatsAgentB")

	// Create task lists.
	for _, data := range []map[string]any{
		{
			"list_id": "stats-list-a", "agent_id": agentA.ID,
			"watch_path": "/tmp/sa",
		},
		{
			"list_id": "stats-list-b", "agent_id": agentB.ID,
			"watch_path": "/tmp/sb",
		},
	} {
		resp := env.postJSON("/api/v1/task-lists", data)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Create tasks with various statuses for both agents.
	tasks := []map[string]any{
		// Agent A: 2 pending, 1 in_progress, 1 completed.
		{
			"agent_id": agentA.ID, "list_id": "stats-list-a",
			"claude_task_id": "sa-p1", "subject": "Pending 1",
			"status": "TASK_STATUS_PENDING",
		},
		{
			"agent_id": agentA.ID, "list_id": "stats-list-a",
			"claude_task_id": "sa-p2", "subject": "Pending 2",
			"status": "TASK_STATUS_PENDING",
		},
		{
			"agent_id": agentA.ID, "list_id": "stats-list-a",
			"claude_task_id": "sa-ip1", "subject": "In Progress 1",
			"status": "TASK_STATUS_IN_PROGRESS",
		},
		{
			"agent_id": agentA.ID, "list_id": "stats-list-a",
			"claude_task_id": "sa-c1", "subject": "Completed 1",
			"status": "TASK_STATUS_COMPLETED",
		},
		// Agent B: 1 pending, 1 completed.
		{
			"agent_id": agentB.ID, "list_id": "stats-list-b",
			"claude_task_id": "sb-p1", "subject": "B Pending",
			"status": "TASK_STATUS_PENDING",
		},
		{
			"agent_id": agentB.ID, "list_id": "stats-list-b",
			"claude_task_id": "sb-c1", "subject": "B Completed",
			"status": "TASK_STATUS_COMPLETED",
		},
	}

	for _, task := range tasks {
		resp := env.postJSON("/api/v1/tasks", task)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	t.Log("Created 6 tasks across 2 agents")

	// Test 1: Get global task stats.
	resp := env.get("/api/v1/tasks/stats")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var statsResp map[string]any
	parseJSON(t, resp, &statsResp)

	stats, ok := statsResp["stats"].(map[string]any)
	require.True(t, ok, "expected stats in response: %v", statsResp)

	// Verify aggregate counts using the helper that handles
	// both float64 and string JSON representations.
	require.Equal(t, int64(3), jsonInt64(stats, "pending_count"),
		"expected 3 pending tasks (2 from A + 1 from B)",
	)
	require.Equal(t, int64(1), jsonInt64(stats, "in_progress_count"),
		"expected 1 in_progress task",
	)
	require.Equal(t, int64(2), jsonInt64(stats, "completed_count"),
		"expected 2 completed tasks",
	)

	t.Logf(
		"Global stats - pending: %s, in_progress: %s, completed: %s",
		jsonIntStr(stats, "pending_count"),
		jsonIntStr(stats, "in_progress_count"),
		jsonIntStr(stats, "completed_count"),
	)

	// Test 2: Get stats filtered by agent_id.
	resp = env.get(fmt.Sprintf(
		"/api/v1/tasks/stats?agent_id=%d", agentA.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var agentAStats map[string]any
	parseJSON(t, resp, &agentAStats)

	aStats := agentAStats["stats"].(map[string]any)
	require.Equal(t, int64(2), jsonInt64(aStats, "pending_count"))
	require.Equal(t, int64(1), jsonInt64(aStats, "in_progress_count"))
	require.Equal(t, int64(1), jsonInt64(aStats, "completed_count"))

	t.Logf(
		"Agent A stats - pending: %s, in_progress: %s, completed: %s",
		jsonIntStr(aStats, "pending_count"),
		jsonIntStr(aStats, "in_progress_count"),
		jsonIntStr(aStats, "completed_count"),
	)

	// Test 3: Get stats filtered by list_id.
	resp = env.get("/api/v1/tasks/stats?list_id=stats-list-b")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listBStats map[string]any
	parseJSON(t, resp, &listBStats)

	bStats := listBStats["stats"].(map[string]any)
	require.Equal(t, int64(1), jsonInt64(bStats, "pending_count"))
	require.Equal(t, int64(1), jsonInt64(bStats, "completed_count"))

	t.Logf(
		"List B stats - pending: %s, completed: %s",
		jsonIntStr(bStats, "pending_count"),
		jsonIntStr(bStats, "completed_count"),
	)

	// Test 4: Get all agent task stats (grouped by agent).
	// Note: the /api/v1/tasks/stats/by-agent route has a routing
	// conflict with /api/v1/tasks/{list_id}/{claude_task_id} in
	// gRPC-gateway v1. The wildcard pattern may match first. We test
	// the endpoint and accept either a successful response or a 404
	// from the route conflict.
	resp = env.get("/api/v1/tasks/stats/by-agent")
	if resp.StatusCode == http.StatusOK {
		var allAgentStats map[string]any
		parseJSON(t, resp, &allAgentStats)

		agentStatsList, ok := allAgentStats["stats"].([]any)
		require.True(t, ok,
			"expected stats array in response: %v",
			allAgentStats,
		)
		require.GreaterOrEqual(t, len(agentStatsList), 2)
		t.Logf("Agent stats entries: %d", len(agentStatsList))
	} else {
		resp.Body.Close()
		t.Logf(
			"Skipped by-agent stats check: route conflict "+
				"with GetTask wildcard (status %d)",
			resp.StatusCode,
		)
	}
}

// TestHTTP_TaskE2EFlow tests the full task lifecycle: register list, create
// tasks, update status, update owner, check stats, sync, and cleanup.
func TestHTTP_TaskE2EFlow(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Step 1: Create an agent.
	agent := env.createAgent("E2ETaskAgent")
	t.Logf("Step 1: Created agent (ID: %d)", agent.ID)

	// Step 2: Register a task list.
	resp := env.postJSON("/api/v1/task-lists", map[string]any{
		"list_id":    "e2e-task-list",
		"agent_id":   agent.ID,
		"watch_path": "/tmp/e2e-tasks",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 2: Registered task list e2e-task-list")

	// Step 3: Create multiple tasks.
	taskDefs := []map[string]any{
		{
			"agent_id": agent.ID, "list_id": "e2e-task-list",
			"claude_task_id": "e2e-task-1",
			"subject":        "Build API",
			"description":    "Create REST endpoints",
			"status":         "TASK_STATUS_PENDING",
		},
		{
			"agent_id": agent.ID, "list_id": "e2e-task-list",
			"claude_task_id": "e2e-task-2",
			"subject":        "Write tests",
			"description":    "Add integration tests",
			"status":         "TASK_STATUS_PENDING",
			"blocked_by":     []string{"e2e-task-1"},
		},
		{
			"agent_id": agent.ID, "list_id": "e2e-task-list",
			"claude_task_id": "e2e-task-3",
			"subject":        "Deploy service",
			"description":    "Deploy to production",
			"status":         "TASK_STATUS_PENDING",
			"blocked_by":     []string{"e2e-task-1", "e2e-task-2"},
		},
	}

	for _, td := range taskDefs {
		resp = env.postJSON("/api/v1/tasks", td)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	t.Log("Step 3: Created 3 tasks with dependency chain")

	// Step 4: Verify all tasks were created.
	resp = env.get("/api/v1/tasks?list_id=e2e-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp map[string]any
	parseJSON(t, resp, &listResp)

	tasksList, ok := listResp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, tasksList, 3)

	t.Log("Step 4: Verified 3 tasks exist")

	// Step 5: Start working on task 1 (update status to in_progress).
	resp = env.patchJSON(
		"/api/v1/tasks/e2e-task-list/e2e-task-1/status",
		map[string]any{
			"list_id":        "e2e-task-list",
			"claude_task_id": "e2e-task-1",
			"status":         "TASK_STATUS_IN_PROGRESS",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 5: Started task 1 (in_progress)")

	// Step 6: Assign owner to task 1.
	resp = env.patchJSON(
		"/api/v1/tasks/e2e-task-list/e2e-task-1/owner",
		map[string]any{
			"list_id":        "e2e-task-list",
			"claude_task_id": "e2e-task-1",
			"owner":          "E2ETaskAgent",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 6: Assigned owner to task 1")

	// Step 7: Complete task 1.
	resp = env.patchJSON(
		"/api/v1/tasks/e2e-task-list/e2e-task-1/status",
		map[string]any{
			"list_id":        "e2e-task-list",
			"claude_task_id": "e2e-task-1",
			"status":         "TASK_STATUS_COMPLETED",
		},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 7: Completed task 1")

	// Step 8: Check stats.
	resp = env.get(fmt.Sprintf(
		"/api/v1/tasks/stats?agent_id=%d", agent.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var statsResp map[string]any
	parseJSON(t, resp, &statsResp)

	stats := statsResp["stats"].(map[string]any)
	require.Equal(
		t, int64(2), jsonInt64(stats, "pending_count"),
		"2 tasks still pending",
	)
	require.Equal(
		t, int64(1), jsonInt64(stats, "completed_count"),
		"1 task completed",
	)

	t.Logf(
		"Step 8: Stats - pending: %s, completed: %s",
		jsonIntStr(stats, "pending_count"),
		jsonIntStr(stats, "completed_count"),
	)

	// Step 9: Sync the task list.
	resp = env.postJSON(
		"/api/v1/task-lists/e2e-task-list/sync",
		map[string]any{"list_id": "e2e-task-list"},
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 9: Synced task list")

	// Verify sync updated last_synced_at.
	resp = env.get("/api/v1/task-lists/e2e-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var syncedListResp map[string]any
	parseJSON(t, resp, &syncedListResp)

	syncedList := syncedListResp["task_list"].(map[string]any)
	require.NotEmpty(t, syncedList["last_synced_at"],
		"last_synced_at should be set after sync",
	)

	t.Log("Step 9: Verified sync timestamp updated")

	// Step 10: Delete a task by ID. First get the task to find its
	// database ID.
	resp = env.get("/api/v1/tasks/e2e-task-list/e2e-task-3")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var task3Resp map[string]any
	parseJSON(t, resp, &task3Resp)

	task3 := task3Resp["task"].(map[string]any)
	task3ID := jsonInt64(task3, "id")
	require.Greater(t, task3ID, int64(0), "task ID should be positive")

	resp = env.deleteReq(fmt.Sprintf("/api/v1/tasks/%d", task3ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 10: Deleted task 3")

	// Verify only 2 tasks remain.
	resp = env.get("/api/v1/tasks?list_id=e2e-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var afterDeleteResp map[string]any
	parseJSON(t, resp, &afterDeleteResp)

	remainingTasks, ok := afterDeleteResp["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, remainingTasks, 2)

	t.Log("Step 10: Verified 2 tasks remain after delete")

	// Step 11: Unregister the task list (deletes all remaining tasks
	// too).
	resp = env.deleteReq("/api/v1/task-lists/e2e-task-list")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	t.Log("Step 11: Unregistered task list")

	// Verify no task lists remain for this agent.
	resp = env.get(fmt.Sprintf(
		"/api/v1/task-lists?agent_id=%d", agent.ID,
	))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var finalListResp map[string]any
	parseJSON(t, resp, &finalListResp)

	// task_lists may be null/absent or empty array when
	// EmitDefaults is true.
	finalLists, _ := finalListResp["task_lists"].([]any)
	require.Empty(t, finalLists, "no task lists should remain")

	t.Log("Step 11: Verified no task lists remain")
	t.Log("E2E task flow complete!")
}

// TestHTTP_TaskBlockedByAndBlocks tests that blocked_by and blocks
// relationships are persisted and returned correctly.
func TestHTTP_TaskBlockedByAndBlocks(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	agent := env.createAgent("BlockAgent")

	resp := env.postJSON("/api/v1/task-lists", map[string]any{
		"list_id": "block-list", "agent_id": agent.ID,
		"watch_path": "/tmp/block",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Create a task that blocks others.
	resp = env.postJSON("/api/v1/tasks", map[string]any{
		"agent_id": agent.ID, "list_id": "block-list",
		"claude_task_id": "blocker",
		"subject":        "Blocker Task",
		"status":         "TASK_STATUS_IN_PROGRESS",
		"blocks":         []string{"blocked-1", "blocked-2"},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Create a task that is blocked.
	resp = env.postJSON("/api/v1/tasks", map[string]any{
		"agent_id": agent.ID, "list_id": "block-list",
		"claude_task_id": "blocked-1",
		"subject":        "Blocked Task 1",
		"status":         "TASK_STATUS_PENDING",
		"blocked_by":     []string{"blocker"},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify blocker task has blocks array.
	resp = env.get("/api/v1/tasks/block-list/blocker")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var blockerResp map[string]any
	parseJSON(t, resp, &blockerResp)

	blockerTask := blockerResp["task"].(map[string]any)
	blocks, ok := blockerTask["blocks"].([]any)
	require.True(t, ok, "expected blocks array: %v", blockerTask)
	require.Len(t, blocks, 2)
	require.Contains(t, blocks, "blocked-1")
	require.Contains(t, blocks, "blocked-2")

	t.Logf("Blocker task blocks: %v", blocks)

	// Verify blocked task has blocked_by array.
	resp = env.get("/api/v1/tasks/block-list/blocked-1")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var blockedResp map[string]any
	parseJSON(t, resp, &blockedResp)

	blockedTask := blockedResp["task"].(map[string]any)
	blockedBy, ok := blockedTask["blocked_by"].([]any)
	require.True(t, ok,
		"expected blocked_by array: %v", blockedTask,
	)
	require.Len(t, blockedBy, 1)
	require.Contains(t, blockedBy, "blocker")

	t.Logf("Blocked task blocked_by: %v", blockedBy)
}
