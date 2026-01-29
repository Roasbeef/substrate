package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// SearchResult represents a message search result with its state.
type SearchResult struct {
	sqlc.Message
	State string
	Rank  float64
}

// SearchMessages performs a full-text search across all messages using FTS5.
// The query should use FTS5 query syntax (e.g., "word1 word2" for AND,
// "word1 OR word2" for OR).
func (s *Store) SearchMessages(ctx context.Context, query string,
	limit int) ([]SearchResult, error) {

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.thread_id, m.topic_id, m.log_offset, m.sender_id,
		       m.subject, m.body_md, m.priority, m.deadline_at,
		       m.attachments, m.created_at, fts.rank
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		WHERE messages_fts MATCH ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var deadlineAt sql.NullInt64
		var attachments sql.NullString

		err := rows.Scan(
			&r.ID, &r.ThreadID, &r.TopicID, &r.LogOffset, &r.SenderID,
			&r.Subject, &r.BodyMd, &r.Priority, &deadlineAt,
			&attachments, &r.CreatedAt, &r.Rank,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		r.DeadlineAt = deadlineAt
		r.Attachments = attachments
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// SearchMessagesForAgent performs a full-text search for messages visible to
// a specific agent.
func (s *Store) SearchMessagesForAgent(ctx context.Context, query string,
	agentID int64, limit int) ([]SearchResult, error) {

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.thread_id, m.topic_id, m.log_offset, m.sender_id,
		       m.subject, m.body_md, m.priority, m.deadline_at,
		       m.attachments, m.created_at, mr.state, fts.rank
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		JOIN message_recipients mr ON m.id = mr.message_id
		WHERE messages_fts MATCH ? AND mr.agent_id = ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages for agent: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var deadlineAt sql.NullInt64
		var attachments sql.NullString

		err := rows.Scan(
			&r.ID, &r.ThreadID, &r.TopicID, &r.LogOffset, &r.SenderID,
			&r.Subject, &r.BodyMd, &r.Priority, &deadlineAt,
			&attachments, &r.CreatedAt, &r.State, &r.Rank,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		r.DeadlineAt = deadlineAt
		r.Attachments = attachments
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}
