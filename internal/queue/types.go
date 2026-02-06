package queue

import (
	"database/sql"
	"errors"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// OperationType defines the type of operation stored in the queue.
type OperationType string

const (
	// OpSend represents a mail send operation.
	OpSend OperationType = "send"

	// OpPublish represents a topic publish operation.
	OpPublish OperationType = "publish"

	// OpHeartbeat represents an agent heartbeat operation.
	OpHeartbeat OperationType = "heartbeat"

	// OpStatusUpdate represents a status update operation.
	OpStatusUpdate OperationType = "status_update"
)

// PendingOperation is the domain type for a queued operation awaiting
// delivery.
type PendingOperation struct {
	ID             int64
	IdempotencyKey string
	OperationType  OperationType
	PayloadJSON    string
	AgentName      string
	SessionID      string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	Attempts       int
	LastError      string
	Status         string
}

// QueueStats holds aggregate counts for queued operations.
type QueueStats struct {
	PendingCount   int64
	DeliveredCount int64
	ExpiredCount   int64
	FailedCount    int64
	OldestPending  *time.Time
}

// QueueConfig holds configuration for the local queue.
type QueueConfig struct {
	// MaxPending is the maximum number of pending operations allowed.
	MaxPending int

	// DefaultTTL is the default time-to-live for queued operations.
	DefaultTTL time.Duration
}

// DefaultQueueConfig returns sensible defaults for the queue.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		MaxPending: 100,
		DefaultTTL: 7 * 24 * time.Hour,
	}
}

// ErrQueueFull is returned when the queue has reached its maximum pending
// capacity.
var ErrQueueFull = errors.New("queue is full")

// PendingOperationFromSqlc converts a sqlc PendingOperation to the domain
// type.
func PendingOperationFromSqlc(op sqlc.PendingOperation) PendingOperation {
	result := PendingOperation{
		ID:             op.ID,
		IdempotencyKey: op.IdempotencyKey,
		OperationType:  OperationType(op.OperationType),
		PayloadJSON:    op.PayloadJson,
		AgentName:      op.AgentName,
		CreatedAt:      time.Unix(op.CreatedAt, 0),
		ExpiresAt:      time.Unix(op.ExpiresAt, 0),
		Attempts:       int(op.Attempts),
		Status:         op.Status,
	}

	if op.SessionID.Valid {
		result.SessionID = op.SessionID.String
	}
	if op.LastError.Valid {
		result.LastError = op.LastError.String
	}

	return result
}

// QueueStatsFromSqlc converts a sqlc GetQueueStatsRow to the domain type.
func QueueStatsFromSqlc(row sqlc.GetQueueStatsRow) QueueStats {
	stats := QueueStats{
		PendingCount:   row.PendingCount,
		DeliveredCount: row.DeliveredCount,
		ExpiredCount:   row.ExpiredCount,
		FailedCount:    row.FailedCount,
	}

	// OldestPending comes as interface{} from the MIN aggregate.
	if row.OldestPending != nil {
		switch v := row.OldestPending.(type) {
		case int64:
			t := time.Unix(v, 0)
			stats.OldestPending = &t
		case float64:
			t := time.Unix(int64(v), 0)
			stats.OldestPending = &t
		}
	}

	return stats
}

// toSqlcNullString converts a string to sql.NullString, treating empty
// strings as NULL.
func toSqlcNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: s, Valid: true}
}
