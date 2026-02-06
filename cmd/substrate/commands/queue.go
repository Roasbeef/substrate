package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/roasbeef/subtrate/internal/queue"
)

// queueCmd is the parent command for queue management subcommands.
var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage the local offline operation queue",
	Long: `Manage the local queue used for store-and-forward when the daemon
and database are unavailable. Operations queued here are automatically
delivered when connectivity is restored.`,
}

// queueListCmd lists all pending operations in the queue.
var queueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending queued operations",
	RunE:  runQueueList,
}

// queueDrainCmd connects to the daemon and delivers all pending operations.
var queueDrainCmd = &cobra.Command{
	Use:   "drain",
	Short: "Connect and deliver all pending queued operations",
	RunE:  runQueueDrain,
}

// queueClearCmd deletes all operations from the queue.
var queueClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all pending queued operations",
	RunE:  runQueueClear,
}

// queueStatsCmd shows aggregate counts for the queue.
var queueStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show queue statistics",
	RunE:  runQueueStats,
}

func init() {
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueDrainCmd)
	queueCmd.AddCommand(queueClearCmd)
	queueCmd.AddCommand(queueStatsCmd)
}

// openQueueStore opens the queue database for subcommand use.
func openQueueStore() (*queue.QueueStore, error) {
	root, err := queue.FindProjectRoot(projectDir)
	if err != nil {
		return nil, fmt.Errorf("find project root: %w", err)
	}

	cfg := queue.DefaultQueueConfig()
	qs, err := queue.OpenQueueStore(queue.QueueDBPath(root), cfg)
	if err != nil {
		return nil, fmt.Errorf("open queue: %w", err)
	}

	return qs, nil
}

// runQueueList lists all pending operations in the queue.
func runQueueList(cmd *cobra.Command, args []string) error {
	qs, err := openQueueStore()
	if err != nil {
		return err
	}
	defer qs.Close()

	ctx := context.Background()
	ops, err := qs.List(ctx)
	if err != nil {
		return fmt.Errorf("list queue: %w", err)
	}

	if outputFormat == "json" {
		return outputJSON(ops)
	}

	if len(ops) == 0 {
		fmt.Println("Queue is empty.")
		return nil
	}

	fmt.Printf("Pending operations: %d\n", len(ops))
	fmt.Println(strings.Repeat("-", 60))

	for _, op := range ops {
		age := time.Since(op.CreatedAt).Truncate(time.Second)
		fmt.Printf(
			"  %s  %-15s  agent=%-12s  age=%s",
			op.IdempotencyKey[:8], op.OperationType,
			op.AgentName, age,
		)
		if op.Attempts > 0 {
			fmt.Printf("  attempts=%d", op.Attempts)
		}
		if op.LastError != "" {
			fmt.Printf("  err=%s", op.LastError)
		}
		fmt.Println()
	}

	return nil
}

// runQueueDrain connects to the daemon and delivers all pending operations.
func runQueueDrain(cmd *cobra.Command, args []string) error {
	qs, err := openQueueStore()
	if err != nil {
		return err
	}
	defer qs.Close()

	ctx := context.Background()

	// Purge expired first.
	purged, err := qs.PurgeExpired(ctx)
	if err != nil {
		return fmt.Errorf("purge expired: %w", err)
	}
	if purged > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Purged %d expired operation(s)\n", purged,
		)
	}

	// Get a connected client (not queued).
	addr := grpcAddr
	if addr == "" {
		addr = defaultGRPCAddr
	}

	client, err := tryGRPCConnection(addr)
	if err != nil {
		client, err = getDirectClient()
		if err != nil {
			return fmt.Errorf("no connectivity for drain: " +
				"daemon not running and database unavailable")
		}
	}
	defer client.Close()

	// Drain and deliver.
	ops, err := qs.Drain(ctx)
	if err != nil {
		return fmt.Errorf("drain queue: %w", err)
	}

	if len(ops) == 0 {
		fmt.Println("Queue is empty, nothing to drain.")
		return nil
	}

	delivered := 0
	for _, op := range ops {
		err := deliverOperation(ctx, client, op)
		if err != nil {
			_ = qs.MarkFailed(ctx, op.ID, err.Error())
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Failed: %s (%s): %v\n",
				op.IdempotencyKey[:8], op.OperationType, err,
			)
			continue
		}

		_ = qs.MarkDelivered(ctx, op.ID)
		delivered++
	}

	fmt.Printf("Delivered %d of %d operation(s)\n",
		delivered, len(ops),
	)

	return nil
}

// runQueueClear deletes all operations from the queue.
func runQueueClear(cmd *cobra.Command, args []string) error {
	qs, err := openQueueStore()
	if err != nil {
		return err
	}
	defer qs.Close()

	ctx := context.Background()
	if err := qs.Clear(ctx); err != nil {
		return fmt.Errorf("clear queue: %w", err)
	}

	fmt.Println("Queue cleared.")
	return nil
}

// runQueueStats shows aggregate counts for the queue.
func runQueueStats(cmd *cobra.Command, args []string) error {
	qs, err := openQueueStore()
	if err != nil {
		return err
	}
	defer qs.Close()

	ctx := context.Background()
	stats, err := qs.Stats(ctx)
	if err != nil {
		return fmt.Errorf("get queue stats: %w", err)
	}

	if outputFormat == "json" {
		return outputJSON(stats)
	}

	fmt.Println("Queue Statistics")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Pending:   %d\n", stats.PendingCount)
	fmt.Printf("Delivered: %d\n", stats.DeliveredCount)
	fmt.Printf("Expired:   %d\n", stats.ExpiredCount)
	fmt.Printf("Failed:    %d\n", stats.FailedCount)

	if stats.OldestPending != nil {
		age := time.Since(*stats.OldestPending).Truncate(time.Second)
		fmt.Printf("Oldest:    %s ago\n", age)
	}

	return nil
}
