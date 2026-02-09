package queue

import (
	"encoding/json"
	"fmt"
	"time"
)

// SendPayload stores the data for a queued send operation. It uses agent
// names instead of IDs so they can be resolved at drain time when the
// database is available.
type SendPayload struct {
	SenderName     string     `json:"sender_name"`
	RecipientNames []string   `json:"recipient_names"`
	Subject        string     `json:"subject"`
	Body           string     `json:"body"`
	Priority       string     `json:"priority"`
	TopicName      string     `json:"topic_name,omitempty"`
	ThreadID       string     `json:"thread_id,omitempty"`
	DeadlineAt     *time.Time `json:"deadline_at,omitempty"`
	Attachments    string     `json:"attachments,omitempty"`
}

// PublishPayload stores the data for a queued publish operation.
type PublishPayload struct {
	SenderName string `json:"sender_name"`
	TopicName  string `json:"topic_name"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	Priority   string `json:"priority"`
}

// HeartbeatPayload stores the data for a queued heartbeat operation.
type HeartbeatPayload struct {
	AgentName    string `json:"agent_name"`
	SessionStart bool   `json:"session_start,omitempty"`
}

// StatusUpdatePayload stores the data for a queued status update operation.
type StatusUpdatePayload struct {
	SenderName     string   `json:"sender_name"`
	RecipientNames []string `json:"recipient_names"`
	Subject        string   `json:"subject"`
	Body           string   `json:"body"`
}

// MarshalPayload serializes a payload struct to JSON for queue storage.
func MarshalPayload(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	return string(data), nil
}

// UnmarshalPayload deserializes a JSON payload string back into the
// appropriate type based on the operation type.
func UnmarshalPayload(
	opType OperationType, jsonStr string,
) (any, error) {
	switch opType {
	case OpSend:
		var p SendPayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			return nil, fmt.Errorf("unmarshal send: %w", err)
		}
		return &p, nil

	case OpPublish:
		var p PublishPayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			return nil, fmt.Errorf("unmarshal publish: %w", err)
		}
		return &p, nil

	case OpHeartbeat:
		var p HeartbeatPayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			return nil, fmt.Errorf("unmarshal heartbeat: %w", err)
		}
		return &p, nil

	case OpStatusUpdate:
		var p StatusUpdatePayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			return nil, fmt.Errorf("unmarshal status: %w", err)
		}
		return &p, nil

	default:
		return nil, fmt.Errorf("unknown operation type: %s", opType)
	}
}
