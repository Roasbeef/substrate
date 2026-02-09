package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// tasksCmd is the parent command for task operations.
var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage Claude Code agent tasks",
	Long:  "View, filter, and manage tasks tracked from Claude Code agents.",
}

// tasksListCmd lists tasks with optional filters.
var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks with optional filters",
	RunE:  runTasksList,
}

// tasksStatsCmd shows task statistics.
var tasksStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show task statistics",
	RunE:  runTasksStats,
}

// tasksRegisterCmd registers a task list.
var tasksRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a task list for tracking",
	RunE:  runTasksRegister,
}

// tasksSyncCmd syncs a task list from disk.
var tasksSyncCmd = &cobra.Command{
	Use:   "sync <list-id>",
	Short: "Sync a task list from disk",
	Args:  cobra.ExactArgs(1),
	RunE:  runTasksSync,
}

// tasksListsCmd lists registered task lists.
var tasksListsCmd = &cobra.Command{
	Use:   "lists",
	Short: "List registered task lists",
	RunE:  runTasksLists,
}

// tasksHookSyncCmd processes task tool output from PostToolUse hooks.
var tasksHookSyncCmd = &cobra.Command{
	Use:    "hook-sync",
	Short:  "Process task tool output from hooks (internal)",
	Hidden: true,
	RunE:   runTasksHookSync,
}

// Task command flags.
var (
	// List filters.
	tasksFilterStatus string
	tasksFilterAgent  int64
	tasksFilterList   string
	tasksActiveOnly   bool
	tasksAvailOnly    bool
	tasksListLimit    int

	// Register flags.
	tasksRegisterList  string
	tasksRegisterPath  string
	tasksRegisterAgent int64

	// Stats flags.
	tasksStatsAgent int64

	// Hook-sync flags.
	hookSyncTool string
	hookSyncList string
)

func init() {
	// List filters.
	tasksListCmd.Flags().StringVar(
		&tasksFilterStatus, "status", "",
		"Filter by status: pending, in_progress, completed, deleted",
	)
	tasksListCmd.Flags().Int64Var(
		&tasksFilterAgent, "agent-id", 0,
		"Filter by agent ID",
	)
	tasksListCmd.Flags().StringVar(
		&tasksFilterList, "list", "",
		"Filter by task list ID",
	)
	tasksListCmd.Flags().BoolVar(
		&tasksActiveOnly, "active", false,
		"Show only active tasks (pending or in_progress)",
	)
	tasksListCmd.Flags().BoolVar(
		&tasksAvailOnly, "available", false,
		"Show only available tasks (pending, not blocked)",
	)
	tasksListCmd.Flags().IntVar(
		&tasksListLimit, "limit", 50,
		"Maximum number of tasks to show",
	)

	// Register flags.
	tasksRegisterCmd.Flags().StringVar(
		&tasksRegisterList, "list-id", "",
		"Task list ID (from CLAUDE_CODE_TASK_LIST_ID)",
	)
	tasksRegisterCmd.Flags().StringVar(
		&tasksRegisterPath, "path", "",
		"Path to task list directory",
	)
	tasksRegisterCmd.Flags().Int64Var(
		&tasksRegisterAgent, "agent-id", 0,
		"Agent ID to associate with this list",
	)

	// Stats flags.
	tasksStatsCmd.Flags().Int64Var(
		&tasksStatsAgent, "agent-id", 0,
		"Show stats for specific agent",
	)

	// Hook-sync flags.
	tasksHookSyncCmd.Flags().StringVar(
		&hookSyncTool, "tool", "",
		"Tool type: create, update, list, get",
	)
	tasksHookSyncCmd.Flags().StringVar(
		&hookSyncList, "list", "",
		"Task list ID",
	)

	// Register subcommands.
	tasksCmd.AddCommand(tasksListCmd)
	tasksCmd.AddCommand(tasksStatsCmd)
	tasksCmd.AddCommand(tasksRegisterCmd)
	tasksCmd.AddCommand(tasksSyncCmd)
	tasksCmd.AddCommand(tasksListsCmd)
	tasksCmd.AddCommand(tasksHookSyncCmd)
}

// runTasksList handles the `substrate tasks list` command.
func runTasksList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Build request.
	req := &subtraterpc.ListTasksRequest{
		Limit:         int32(tasksListLimit),
		ActiveOnly:    tasksActiveOnly,
		AvailableOnly: tasksAvailOnly,
	}

	if tasksFilterAgent > 0 {
		req.AgentId = tasksFilterAgent
	}
	if tasksFilterList != "" {
		req.ListId = tasksFilterList
	}
	if tasksFilterStatus != "" {
		req.Status = parseTaskStatus(tasksFilterStatus)
	}

	resp, err := client.ListTasks(ctx, req)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("list tasks: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		if len(resp.Tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		// Print table header.
		fmt.Printf(
			"%-10s %-12s %-30s %-12s %s\n",
			"STATUS", "OWNER", "SUBJECT", "LIST", "UPDATED",
		)
		fmt.Println(strings.Repeat("-", 80))

		for _, t := range resp.Tasks {
			// Format status with icon.
			status := formatTaskStatus(t.Status)

			// Truncate subject.
			subject := t.Subject
			if len(subject) > 30 {
				subject = subject[:27] + "..."
			}

			// Truncate list ID.
			listID := t.ListId
			if len(listID) > 12 {
				listID = listID[:9] + "..."
			}

			// Format owner.
			owner := t.Owner
			if owner == "" {
				owner = "-"
			}
			if len(owner) > 12 {
				owner = owner[:9] + "..."
			}

			// Format updated time.
			updated := "-"
			if t.UpdatedAt != nil && t.UpdatedAt.IsValid() {
				updated = formatRelativeTimeProto(t.UpdatedAt)
			}

			fmt.Printf(
				"%-10s %-12s %-30s %-12s %s\n",
				status, owner, subject, listID, updated,
			)
		}
	}

	return nil
}

// runTasksStats handles the `substrate tasks stats` command.
func runTasksStats(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// If no agent specified, show per-agent stats.
	if tasksStatsAgent == 0 {
		resp, err := client.GetAllAgentTaskStats(
			ctx, &subtraterpc.GetAllAgentTaskStatsRequest{
				TodaySince: timestamppb.New(todayStart()),
			},
		)
		if err != nil {
			return fmt.Errorf("get agent stats: %w", err)
		}
		if resp.Error != "" {
			return fmt.Errorf("get agent stats: %s", resp.Error)
		}

		switch outputFormat {
		case "json":
			return outputJSON(resp)
		default:
			if len(resp.Stats) == 0 {
				fmt.Println("No task statistics found.")
				return nil
			}

			fmt.Printf(
				"%-20s %10s %12s %10s %12s\n",
				"AGENT", "IN_PROG", "PENDING", "BLOCKED",
				"DONE_TODAY",
			)
			fmt.Println(strings.Repeat("-", 70))

			for _, s := range resp.Stats {
				name := s.AgentName
				if name == "" {
					name = fmt.Sprintf("Agent %d", s.AgentId)
				}
				if len(name) > 20 {
					name = name[:17] + "..."
				}

				fmt.Printf(
					"%-20s %10d %12d %10d %12d\n",
					name, s.InProgressCount, s.PendingCount,
					s.BlockedCount, s.CompletedToday,
				)
			}
		}
		return nil
	}

	// Show stats for specific agent.
	resp, err := client.GetTaskStats(
		ctx, &subtraterpc.GetTaskStatsRequest{
			AgentId:    tasksStatsAgent,
			TodaySince: timestamppb.New(todayStart()),
		},
	)
	if err != nil {
		return fmt.Errorf("get stats: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("get stats: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		s := resp.Stats
		fmt.Printf("Task Statistics (Agent %d)\n", tasksStatsAgent)
		fmt.Println(strings.Repeat("-", 30))
		fmt.Printf("  In Progress: %d\n", s.InProgressCount)
		fmt.Printf("  Pending:     %d\n", s.PendingCount)
		fmt.Printf("  Available:   %d\n", s.AvailableCount)
		fmt.Printf("  Blocked:     %d\n", s.BlockedCount)
		fmt.Printf("  Completed:   %d\n", s.CompletedCount)
		fmt.Printf("  Done Today:  %d\n", s.CompletedToday)
	}

	return nil
}

// runTasksRegister handles the `substrate tasks register` command.
func runTasksRegister(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Get list ID from flag or environment.
	listID := tasksRegisterList
	if listID == "" {
		listID = os.Getenv("CLAUDE_CODE_TASK_LIST_ID")
	}
	if listID == "" {
		return fmt.Errorf(
			"task list ID required: use --list-id or " +
				"set CLAUDE_CODE_TASK_LIST_ID",
		)
	}

	// Get agent ID from flag or current identity.
	agentID := tasksRegisterAgent
	if agentID == 0 {
		var err error
		agentID, _, err = getCurrentAgentWithClient(ctx, client)
		if err != nil {
			return fmt.Errorf(
				"resolve identity: %w (use --agent-id to specify)",
				err,
			)
		}
	}

	// Get watch path from flag or default.
	watchPath := tasksRegisterPath
	if watchPath == "" {
		home, _ := os.UserHomeDir()
		watchPath = fmt.Sprintf("%s/.claude/tasks/%s", home, listID)
	}

	resp, err := client.RegisterTaskList(
		ctx, &subtraterpc.RegisterTaskListRequest{
			ListId:    listID,
			AgentId:   agentID,
			WatchPath: watchPath,
		},
	)
	if err != nil {
		return fmt.Errorf("register task list: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("register task list: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Task list registered:\n")
		fmt.Printf("  List ID:    %s\n", resp.TaskList.ListId)
		fmt.Printf("  Agent ID:   %d\n", resp.TaskList.AgentId)
		fmt.Printf("  Watch Path: %s\n", resp.TaskList.WatchPath)
	}

	return nil
}

// runTasksSync handles the `substrate tasks sync <list-id>` command.
func runTasksSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	listID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.SyncTaskList(
		ctx, &subtraterpc.SyncTaskListRequest{
			ListId: listID,
		},
	)
	if err != nil {
		return fmt.Errorf("sync task list: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("sync task list: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Sync complete:\n")
		fmt.Printf("  Updated: %d\n", resp.TasksUpdated)
		fmt.Printf("  Deleted: %d\n", resp.TasksDeleted)
	}

	return nil
}

// runTasksLists handles the `substrate tasks lists` command.
func runTasksLists(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.ListTaskLists(
		ctx, &subtraterpc.ListTaskListsRequest{
			AgentId: tasksFilterAgent,
		},
	)
	if err != nil {
		return fmt.Errorf("list task lists: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("list task lists: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		if len(resp.TaskLists) == 0 {
			fmt.Println("No task lists registered.")
			return nil
		}

		fmt.Printf(
			"%-20s %-10s %-30s %s\n",
			"LIST ID", "AGENT", "WATCH PATH", "LAST SYNC",
		)
		fmt.Println(strings.Repeat("-", 80))

		for _, tl := range resp.TaskLists {
			listID := tl.ListId
			if len(listID) > 20 {
				listID = listID[:17] + "..."
			}

			watchPath := tl.WatchPath
			if len(watchPath) > 30 {
				watchPath = "..." + watchPath[len(watchPath)-27:]
			}

			lastSync := "-"
			if tl.LastSyncedAt != nil && tl.LastSyncedAt.IsValid() {
				lastSync = formatRelativeTimeProto(tl.LastSyncedAt)
			}

			fmt.Printf(
				"%-20s %-10d %-30s %s\n",
				listID, tl.AgentId, watchPath, lastSync,
			)
		}
	}

	return nil
}

// Helper functions.

// parseTaskStatus converts a status string to proto enum.
func parseTaskStatus(s string) subtraterpc.TaskStatus {
	switch strings.ToLower(s) {
	case "pending":
		return subtraterpc.TaskStatus_TASK_STATUS_PENDING
	case "in_progress", "in-progress", "inprogress":
		return subtraterpc.TaskStatus_TASK_STATUS_IN_PROGRESS
	case "completed", "complete", "done":
		return subtraterpc.TaskStatus_TASK_STATUS_COMPLETED
	case "deleted":
		return subtraterpc.TaskStatus_TASK_STATUS_DELETED
	default:
		return subtraterpc.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

// formatTaskStatus returns a human-readable status with icon.
func formatTaskStatus(s subtraterpc.TaskStatus) string {
	switch s {
	case subtraterpc.TaskStatus_TASK_STATUS_PENDING:
		return "[.] pending"
	case subtraterpc.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return "[>] active"
	case subtraterpc.TaskStatus_TASK_STATUS_COMPLETED:
		return "[+] done"
	case subtraterpc.TaskStatus_TASK_STATUS_DELETED:
		return "[x] deleted"
	default:
		return "[?] unknown"
	}
}

// formatRelativeTimeProto formats a proto timestamp as relative time.
func formatRelativeTimeProto(ts *timestamppb.Timestamp) string {
	if ts == nil || !ts.IsValid() {
		return "-"
	}

	t := ts.AsTime()
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("Jan 2")
}

// todayStart returns the start of today (midnight).
func todayStart() time.Time {
	now := time.Now()
	return time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0, now.Location(),
	)
}

// runTasksHookSync handles the `substrate tasks hook-sync` command.
// This is called by the PostToolUse hook to sync task data from Claude Code.
func runTasksHookSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if hookSyncList == "" {
		return fmt.Errorf("--list is required")
	}
	if hookSyncTool == "" {
		return fmt.Errorf("--tool is required")
	}

	// Read JSON from stdin.
	reader := bufio.NewReader(os.Stdin)
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	if len(data) == 0 {
		return nil // No data to process.
	}

	// The list ID IS the session ID (Claude Code uses the session ID as
	// the task list ID). If no --session-id was provided and the env var
	// isn't set, use hookSyncList so identity resolution can find/create
	// the agent identity.
	if sessionID == "" && os.Getenv("CLAUDE_SESSION_ID") == "" {
		sessionID = hookSyncList
	}

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Get agent ID from current identity.
	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		// Log error but continue with agentID=0 so tasks still sync.
		// The agent association is secondary to getting tasks into the
		// database.
		fmt.Fprintf(os.Stderr, "warning: could not resolve agent "+
			"identity: %v\n", err)
		agentID = 0
	}

	switch hookSyncTool {
	case "create":
		return syncTaskCreate(ctx, client, agentID, hookSyncList, data)
	case "update":
		return syncTaskUpdate(ctx, client, agentID, hookSyncList, data)
	case "list":
		return syncTaskList(ctx, client, agentID, hookSyncList, data)
	case "get":
		return syncTaskGet(ctx, client, agentID, hookSyncList, data)
	case "reconcile":
		return syncTaskReconcile(ctx, client, agentID, hookSyncList)
	default:
		return fmt.Errorf("unknown tool: %s", hookSyncTool)
	}
}

// ClaudeTask represents the task structure from Claude Code tools.
type ClaudeTask struct {
	ID          string   `json:"id"`
	TaskID      string   `json:"taskId"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	ActiveForm  string   `json:"activeForm"`
	Owner       string   `json:"owner"`
	BlockedBy   []string `json:"blockedBy"`
	Blocks      []string `json:"blocks"`

	// Incremental dependency updates from TaskUpdate tool.
	AddBlockedBy []string `json:"addBlockedBy"`
	AddBlocks    []string `json:"addBlocks"`
}

// ClaudeTaskList represents the TaskList tool output.
type ClaudeTaskList struct {
	Tasks []ClaudeTask `json:"tasks"`
}

// ClaudeTaskWrapper represents a wrapped task response from Claude Code.
// TaskGet and TaskCreate responses may be wrapped in {"task": {...}}.
type ClaudeTaskWrapper struct {
	Task ClaudeTask `json:"task"`
}

// syncTaskCreate handles TaskCreate tool output.
func syncTaskCreate(
	ctx context.Context, client *Client, agentID int64,
	listID string, data []byte,
) error {
	var task ClaudeTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil // Ignore invalid JSON.
	}

	// If direct unmarshal didn't find an ID, try wrapped format.
	if task.ID == "" && task.TaskID == "" {
		var wrapper ClaudeTaskWrapper
		if err := json.Unmarshal(data, &wrapper); err == nil {
			task = wrapper.Task
		}
	}

	taskID := task.TaskID
	if taskID == "" {
		taskID = task.ID
	}
	if taskID == "" {
		return nil
	}

	_, err := client.UpsertTask(ctx, &subtraterpc.UpsertTaskRequest{
		AgentId:      agentID,
		ListId:       listID,
		ClaudeTaskId: taskID,
		Subject:      task.Subject,
		Description:  task.Description,
		ActiveForm:   task.ActiveForm,
		Status:       claudeStatusToProto(task.Status),
		Owner:        task.Owner,
		BlockedBy:    task.BlockedBy,
		Blocks:       task.Blocks,
	})
	return err
}

// syncTaskUpdate handles TaskUpdate tool input.
func syncTaskUpdate(
	ctx context.Context, client *Client, agentID int64,
	listID string, data []byte,
) error {
	var task ClaudeTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil
	}

	taskID := task.TaskID
	if taskID == "" {
		taskID = task.ID
	}
	if taskID == "" {
		return nil
	}

	blockedBy := task.BlockedBy
	blocks := task.Blocks

	// Handle incremental dependency updates (addBlockedBy/addBlocks).
	// These add to existing arrays rather than replacing them.
	if len(task.AddBlockedBy) > 0 || len(task.AddBlocks) > 0 {
		existing, err := client.GetTask(ctx, &subtraterpc.GetTaskProtoRequest{
			ListId:       listID,
			ClaudeTaskId: taskID,
		})
		if err == nil && existing.Task != nil {
			blockedBy = mergeStringSlices(
				existing.Task.BlockedBy, task.AddBlockedBy,
			)
			blocks = mergeStringSlices(
				existing.Task.Blocks, task.AddBlocks,
			)
		} else {
			// Task doesn't exist yet — use add* as initial values.
			blockedBy = task.AddBlockedBy
			blocks = task.AddBlocks
		}
	}

	_, err := client.UpsertTask(ctx, &subtraterpc.UpsertTaskRequest{
		AgentId:      agentID,
		ListId:       listID,
		ClaudeTaskId: taskID,
		Subject:      task.Subject,
		Description:  task.Description,
		ActiveForm:   task.ActiveForm,
		Status:       claudeStatusToProto(task.Status),
		Owner:        task.Owner,
		BlockedBy:    blockedBy,
		Blocks:       blocks,
	})
	return err
}

// mergeStringSlices merges two string slices, deduplicating entries.
func mergeStringSlices(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing))
	result := make([]string, 0, len(existing)+len(additions))
	for _, s := range existing {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range additions {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// syncTaskList handles TaskList tool output.
func syncTaskList(
	ctx context.Context, client *Client, agentID int64,
	listID string, data []byte,
) error {
	var taskList ClaudeTaskList
	if err := json.Unmarshal(data, &taskList); err != nil {
		// Try as array directly.
		var tasks []ClaudeTask
		if err := json.Unmarshal(data, &tasks); err != nil {
			return nil
		}
		taskList.Tasks = tasks
	}

	// Compute inverse "blocks" from "blockedBy" since Claude Code's
	// TaskList only provides blockedBy but not blocks.
	blocksMap := make(map[string][]string)
	for _, task := range taskList.Tasks {
		taskID := task.TaskID
		if taskID == "" {
			taskID = task.ID
		}
		for _, depID := range task.BlockedBy {
			blocksMap[depID] = append(blocksMap[depID], taskID)
		}
	}

	// Build a set of claude_task_ids present in the response for orphan
	// detection after upserting.
	responseTaskIDs := make(map[string]bool, len(taskList.Tasks))

	for _, task := range taskList.Tasks {
		taskID := task.TaskID
		if taskID == "" {
			taskID = task.ID
		}
		if taskID == "" {
			continue
		}

		responseTaskIDs[taskID] = true

		// Merge explicit blocks with computed inverse blocks.
		blocks := mergeStringSlices(task.Blocks, blocksMap[taskID])

		_, _ = client.UpsertTask(ctx, &subtraterpc.UpsertTaskRequest{
			AgentId:      agentID,
			ListId:       listID,
			ClaudeTaskId: taskID,
			Subject:      task.Subject,
			Description:  task.Description,
			ActiveForm:   task.ActiveForm,
			Status:       claudeStatusToProto(task.Status),
			Owner:        task.Owner,
			BlockedBy:    task.BlockedBy,
			Blocks:       blocks,
		})
	}

	// Remove orphan tasks: any task in the DB for this list that is not in
	// the TaskList response no longer exists in Claude Code's state.
	dbTasks, err := client.ListTasks(ctx, &subtraterpc.ListTasksRequest{
		ListId: listID,
	})
	if err != nil {
		return nil // Non-fatal: orphan cleanup is best-effort.
	}

	for _, dbTask := range dbTasks.GetTasks() {
		if !responseTaskIDs[dbTask.ClaudeTaskId] {
			_, _ = client.DeleteTask(
				ctx,
				&subtraterpc.DeleteTaskRequest{
					Id: dbTask.Id,
				},
			)
		}
	}

	return nil
}

// syncTaskGet handles TaskGet tool output.
func syncTaskGet(
	ctx context.Context, client *Client, agentID int64,
	listID string, data []byte,
) error {
	// TaskGet returns a single task, same as create.
	return syncTaskCreate(ctx, client, agentID, listID, data)
}

// syncTaskReconcile reads task JSON files from disk and upserts them into the
// database. Claude Code stores task state at ~/.claude/tasks/{listID}/*.json,
// which serves as a source of truth that can be reconciled periodically.
func syncTaskReconcile(
	ctx context.Context, client *Client, agentID int64,
	listID string,
) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	tasksDir := fmt.Sprintf(
		"%s/.claude/tasks/%s", homeDir, listID,
	)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No tasks directory for this list, nothing to reconcile.
			return nil
		}
		return fmt.Errorf("reading tasks dir %s: %w", tasksDir, err)
	}

	// First pass: read all tasks and build the inverse blocks map from
	// blockedBy, since on-disk files may not always have blocks populated.
	var tasks []ClaudeTask
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := fmt.Sprintf("%s/%s", tasksDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files.
		}

		var task ClaudeTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue // Skip malformed JSON.
		}

		// Normalize task ID: prefer "id" field.
		if task.ID == "" && task.TaskID != "" {
			task.ID = task.TaskID
		}
		if task.ID == "" {
			continue // Skip tasks without an ID.
		}

		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		return nil
	}

	// Compute inverse blocks from blockedBy across all tasks.
	blocksMap := make(map[string][]string)
	for _, task := range tasks {
		for _, depID := range task.BlockedBy {
			blocksMap[depID] = append(
				blocksMap[depID], task.ID,
			)
		}
	}

	// Second pass: upsert each task into the database.
	for _, task := range tasks {
		blocks := mergeStringSlices(task.Blocks, blocksMap[task.ID])

		_, _ = client.UpsertTask(ctx, &subtraterpc.UpsertTaskRequest{
			AgentId:      agentID,
			ListId:       listID,
			ClaudeTaskId: task.ID,
			Subject:      task.Subject,
			Description:  task.Description,
			ActiveForm:   task.ActiveForm,
			Status:       claudeStatusToProto(task.Status),
			Owner:        task.Owner,
			BlockedBy:    task.BlockedBy,
			Blocks:       blocks,
		})
	}

	return nil
}

// claudeStatusToProto converts Claude status strings to proto enum.
func claudeStatusToProto(status string) subtraterpc.TaskStatus {
	switch strings.ToLower(status) {
	case "pending":
		return subtraterpc.TaskStatus_TASK_STATUS_PENDING
	case "in_progress", "in-progress", "inprogress":
		return subtraterpc.TaskStatus_TASK_STATUS_IN_PROGRESS
	case "completed", "complete", "done":
		return subtraterpc.TaskStatus_TASK_STATUS_COMPLETED
	case "deleted":
		return subtraterpc.TaskStatus_TASK_STATUS_DELETED
	default:
		return subtraterpc.TaskStatus_TASK_STATUS_PENDING
	}
}
