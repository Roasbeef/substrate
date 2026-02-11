package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockStore provides an in-memory implementation of the Store interface for
// testing purposes. All data is stored in maps and protected by a mutex.
type MockStore struct {
	mu sync.RWMutex

	// Data stores.
	messages          map[int64]Message
	messageRecipients map[int64]map[int64]MessageRecipient // [messageID][agentID]
	agents            map[int64]Agent
	agentsByName      map[string]int64
	agentsBySession   map[string]int64
	topics            map[int64]Topic
	topicsByName      map[string]int64
	subscriptions     map[int64]map[int64]bool // [topicID][agentID]
	activities        []Activity
	sessionIdentities map[string]SessionIdentity

	// Task data stores.
	taskLists  map[string]TaskList
	tasks      map[int64]Task
	tasksByKey map[string]int64 // "listID:claudeTaskID" -> task ID

	// Counters for auto-incrementing IDs.
	nextMessageID  int64
	nextAgentID    int64
	nextTopicID    int64
	nextActivityID int64
	nextTaskListID int64
	nextTaskID     int64
}

// NewMockStore creates a new in-memory mock store.
func NewMockStore() *MockStore {
	return &MockStore{
		messages:          make(map[int64]Message),
		messageRecipients: make(map[int64]map[int64]MessageRecipient),
		agents:            make(map[int64]Agent),
		agentsByName:      make(map[string]int64),
		agentsBySession:   make(map[string]int64),
		topics:            make(map[int64]Topic),
		topicsByName:      make(map[string]int64),
		subscriptions:     make(map[int64]map[int64]bool),
		activities:        make([]Activity, 0),
		sessionIdentities: make(map[string]SessionIdentity),
		taskLists:         make(map[string]TaskList),
		tasks:             make(map[int64]Task),
		tasksByKey:        make(map[string]int64),
		nextMessageID:     1,
		nextAgentID:       1,
		nextTopicID:       1,
		nextActivityID:    1,
		nextTaskListID:    1,
		nextTaskID:        1,
	}
}

// Close is a no-op for the mock store.
func (m *MockStore) Close() error {
	return nil
}

// WithTx executes the function within a "transaction" (just runs the function
// for the mock).
func (m *MockStore) WithTx(
	ctx context.Context,
	fn func(ctx context.Context, s Storage) error,
) error {
	return fn(ctx, m)
}

// WithReadTx executes the function within a read-only "transaction" (just runs
// the function for the mock).
func (m *MockStore) WithReadTx(
	ctx context.Context,
	fn func(ctx context.Context, s Storage) error,
) error {
	return fn(ctx, m)
}

// IsConsistent verifies that the store's internal state is consistent.
// Used for property-based testing.
func (m *MockStore) IsConsistent() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check that all message recipients reference valid messages.
	for msgID := range m.messageRecipients {
		if _, ok := m.messages[msgID]; !ok {
			return false
		}
	}

	// Check that all message recipients reference valid agents.
	for _, recipients := range m.messageRecipients {
		for agentID := range recipients {
			if _, ok := m.agents[agentID]; !ok {
				return false
			}
		}
	}

	// Check that all subscriptions reference valid topics and agents.
	for topicID, subscribers := range m.subscriptions {
		if _, ok := m.topics[topicID]; !ok {
			return false
		}
		for agentID := range subscribers {
			if _, ok := m.agents[agentID]; !ok {
				return false
			}
		}
	}

	return true
}

// MessageStore implementation.

func (m *MockStore) CreateMessage(
	ctx context.Context, params CreateMessageParams,
) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := Message{
		ID:              m.nextMessageID,
		ThreadID:        params.ThreadID,
		TopicID:         params.TopicID,
		LogOffset:       params.LogOffset,
		SenderID:        params.SenderID,
		Subject:         params.Subject,
		Body:            params.Body,
		Priority:        params.Priority,
		DeadlineAt:      params.DeadlineAt,
		Attachments:     params.Attachments,
		CreatedAt:       time.Now(),
		DeletedBySender: false,
		IdempotencyKey:  params.IdempotencyKey,
	}

	m.nextMessageID++
	m.messages[msg.ID] = msg
	m.messageRecipients[msg.ID] = make(map[int64]MessageRecipient)

	return msg, nil
}

func (m *MockStore) GetMessage(ctx context.Context, id int64) (Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msg, ok := m.messages[id]
	if !ok {
		return Message{}, sql.ErrNoRows
	}
	return msg, nil
}

func (m *MockStore) GetMessagesByThread(
	ctx context.Context, threadID string,
) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Message
	for _, msg := range m.messages {
		if msg.ThreadID == threadID {
			result = append(result, msg)
		}
	}
	return result, nil
}

// GetMessagesByThreadWithSender retrieves all messages in a thread with sender
// information (name, project, branch).
func (m *MockStore) GetMessagesByThreadWithSender(
	ctx context.Context, threadID string,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []InboxMessage
	for _, msg := range m.messages {
		if msg.ThreadID == threadID {
			// Look up sender info.
			var senderName, senderProjectKey, senderGitBranch string
			if agent, ok := m.agents[msg.SenderID]; ok {
				senderName = agent.Name
				senderProjectKey = agent.ProjectKey
				senderGitBranch = agent.GitBranch
			}
			result = append(result, InboxMessage{
				Message:          msg,
				SenderName:       senderName,
				SenderProjectKey: senderProjectKey,
				SenderGitBranch:  senderGitBranch,
			})
		}
	}
	return result, nil
}

func (m *MockStore) GetInboxMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []InboxMessage
	for msgID, recipients := range m.messageRecipients {
		if recip, ok := recipients[agentID]; ok {
			if recip.State != "archived" && recip.State != "trash" {
				msg := m.messages[msgID]
				sender := m.agents[msg.SenderID]
				im := InboxMessage{
					Message:    msg,
					SenderName: sender.Name,
					State:      recip.State,
					ReadAt:     recip.ReadAt,
					AckedAt:    recip.AckedAt,
				}
				result = append(result, im)
			}
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockStore) GetUnreadMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []InboxMessage
	for msgID, recipients := range m.messageRecipients {
		if recip, ok := recipients[agentID]; ok {
			if recip.State == "unread" {
				msg := m.messages[msgID]
				sender := m.agents[msg.SenderID]
				im := InboxMessage{
					Message:    msg,
					SenderName: sender.Name,
					State:      recip.State,
				}
				result = append(result, im)
			}
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockStore) GetArchivedMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []InboxMessage
	for msgID, recipients := range m.messageRecipients {
		if recip, ok := recipients[agentID]; ok {
			if recip.State == "archived" {
				msg := m.messages[msgID]
				im := InboxMessage{
					Message: msg,
					State:   recip.State,
				}
				result = append(result, im)
			}
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockStore) UpdateRecipientState(
	ctx context.Context, messageID, agentID int64, state string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	recipients, ok := m.messageRecipients[messageID]
	if !ok {
		return sql.ErrNoRows
	}
	recip, ok := recipients[agentID]
	if !ok {
		return sql.ErrNoRows
	}

	recip.State = state
	now := time.Now()
	recip.ReadAt = &now
	recipients[agentID] = recip

	return nil
}

func (m *MockStore) MarkMessageRead(
	ctx context.Context, messageID, agentID int64,
) error {
	return m.UpdateRecipientState(ctx, messageID, agentID, "read")
}

func (m *MockStore) AckMessage(
	ctx context.Context, messageID, agentID int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	recipients, ok := m.messageRecipients[messageID]
	if !ok {
		return sql.ErrNoRows
	}
	recip, ok := recipients[agentID]
	if !ok {
		return sql.ErrNoRows
	}

	now := time.Now()
	recip.AckedAt = &now
	recipients[agentID] = recip

	return nil
}

func (m *MockStore) SnoozeMessage(
	ctx context.Context, messageID, agentID int64, until time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	recipients, ok := m.messageRecipients[messageID]
	if !ok {
		return sql.ErrNoRows
	}
	recip, ok := recipients[agentID]
	if !ok {
		return sql.ErrNoRows
	}

	recip.State = "snoozed"
	recip.SnoozedUntil = &until
	recipients[agentID] = recip

	return nil
}

func (m *MockStore) CreateMessageRecipient(
	ctx context.Context, messageID, agentID int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.messages[messageID]; !ok {
		return fmt.Errorf("message %d not found", messageID)
	}
	if _, ok := m.agents[agentID]; !ok {
		return fmt.Errorf("agent %d not found", agentID)
	}

	if m.messageRecipients[messageID] == nil {
		m.messageRecipients[messageID] = make(map[int64]MessageRecipient)
	}

	m.messageRecipients[messageID][agentID] = MessageRecipient{
		MessageID: messageID,
		AgentID:   agentID,
		State:     "unread",
	}

	return nil
}

func (m *MockStore) GetMessageRecipient(
	ctx context.Context, messageID, agentID int64,
) (MessageRecipient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipients, ok := m.messageRecipients[messageID]
	if !ok {
		return MessageRecipient{}, sql.ErrNoRows
	}
	recip, ok := recipients[agentID]
	if !ok {
		return MessageRecipient{}, sql.ErrNoRows
	}
	return recip, nil
}

func (m *MockStore) CountUnreadByAgent(
	ctx context.Context, agentID int64,
) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, recipients := range m.messageRecipients {
		if recip, ok := recipients[agentID]; ok && recip.State == "unread" {
			count++
		}
	}
	return count, nil
}

func (m *MockStore) CountUnreadUrgentByAgent(
	ctx context.Context, agentID int64,
) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for msgID, recipients := range m.messageRecipients {
		if recip, ok := recipients[agentID]; ok && recip.State == "unread" {
			if msg, ok := m.messages[msgID]; ok && msg.Priority == "urgent" {
				count++
			}
		}
	}
	return count, nil
}

func (m *MockStore) GetMessagesSinceOffset(
	ctx context.Context, topicID, offset int64, limit int,
) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Message
	for _, msg := range m.messages {
		if msg.TopicID == topicID && msg.LogOffset > offset {
			result = append(result, msg)
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *MockStore) NextLogOffset(
	ctx context.Context, topicID int64,
) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var maxOffset int64
	for _, msg := range m.messages {
		if msg.TopicID == topicID && msg.LogOffset > maxOffset {
			maxOffset = msg.LogOffset
		}
	}
	return maxOffset + 1, nil
}

// SearchMessagesForAgent performs a simple substring search for messages
// visible to a specific agent.
func (m *MockStore) SearchMessagesForAgent(
	ctx context.Context, query string, agentID int64, limit int,
) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Message
	for msgID, recipients := range m.messageRecipients {
		if _, ok := recipients[agentID]; !ok {
			continue
		}

		msg, ok := m.messages[msgID]
		if !ok {
			continue
		}

		// Simple substring match on subject and body.
		if contains(msg.Subject, query) || contains(msg.Body, query) {
			results = append(results, msg)
		}

		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// GetAllInboxMessages retrieves inbox messages across all agents (global view).
func (m *MockStore) GetAllInboxMessages(
	ctx context.Context, limit, offset int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []InboxMessage
	count := 0
	skipped := 0

	for msgID, msg := range m.messages {
		recipients, ok := m.messageRecipients[msgID]
		if !ok {
			continue
		}

		// Find first non-archived recipient.
		for agentID, recip := range recipients {
			if recip.State == "archived" || recip.State == "trash" {
				continue
			}

			if skipped < offset {
				skipped++
				continue
			}

			// Get sender name.
			var senderName string
			if sender, ok := m.agents[msg.SenderID]; ok {
				senderName = sender.Name
			}

			results = append(results, InboxMessage{
				Message:      msg,
				SenderName:   senderName,
				State:        recip.State,
				SnoozedUntil: recip.SnoozedUntil,
				ReadAt:       recip.ReadAt,
				AckedAt:      recip.AckedAt,
			})
			_ = agentID
			count++
			break
		}

		if count >= limit {
			break
		}
	}

	return results, nil
}

// GetMessageRecipients retrieves all recipients for a message.
func (m *MockStore) GetMessageRecipients(
	ctx context.Context, messageID int64,
) ([]MessageRecipientWithAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipients, ok := m.messageRecipients[messageID]
	if !ok {
		return nil, nil
	}

	var results []MessageRecipientWithAgent
	for agentID, recip := range recipients {
		var agentName string
		if agent, ok := m.agents[agentID]; ok {
			agentName = agent.Name
		}
		results = append(results, MessageRecipientWithAgent{
			MessageRecipient: recip,
			AgentName:        agentName,
		})
	}
	return results, nil
}

// GetMessageRecipientsBulk retrieves recipients for multiple messages.
func (m *MockStore) GetMessageRecipientsBulk(
	ctx context.Context, messageIDs []int64,
) (map[int64][]MessageRecipientWithAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int64][]MessageRecipientWithAgent)
	for _, msgID := range messageIDs {
		recipients, ok := m.messageRecipients[msgID]
		if !ok {
			continue
		}
		for agentID, recip := range recipients {
			var agentName string
			if agent, ok := m.agents[agentID]; ok {
				agentName = agent.Name
			}
			result[msgID] = append(result[msgID], MessageRecipientWithAgent{
				MessageRecipient: recip,
				AgentName:        agentName,
			})
		}
	}
	return result, nil
}

// SearchMessages performs global search across all messages.
func (m *MockStore) SearchMessages(
	ctx context.Context, query string, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []InboxMessage
	for _, msg := range m.messages {
		if !contains(msg.Subject, query) && !contains(msg.Body, query) {
			continue
		}

		var senderName string
		if sender, ok := m.agents[msg.SenderID]; ok {
			senderName = sender.Name
		}

		results = append(results, InboxMessage{
			Message:    msg,
			SenderName: senderName,
		})

		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// GetMessagesByTopic retrieves all messages for a topic.
func (m *MockStore) GetMessagesByTopic(
	ctx context.Context, topicID int64,
) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Message
	for _, msg := range m.messages {
		if msg.TopicID == topicID {
			results = append(results, msg)
		}
	}
	return results, nil
}

// GetSentMessages retrieves messages sent by a specific agent.
func (m *MockStore) GetSentMessages(
	ctx context.Context, senderID int64, limit int,
) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Message
	for _, msg := range m.messages {
		if msg.SenderID == senderID {
			results = append(results, msg)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// GetAllSentMessages retrieves all sent messages across all agents.
func (m *MockStore) GetAllSentMessages(
	ctx context.Context, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []InboxMessage
	for _, msg := range m.messages {
		var senderName, senderProjectKey, senderGitBranch string
		if sender, ok := m.agents[msg.SenderID]; ok {
			senderName = sender.Name
			senderProjectKey = sender.ProjectKey
			senderGitBranch = sender.GitBranch
		}

		results = append(results, InboxMessage{
			Message:          msg,
			SenderName:       senderName,
			SenderProjectKey: senderProjectKey,
			SenderGitBranch:  senderGitBranch,
		})

		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// GetMessagesBySenderNamePrefix retrieves messages from agents whose name
// starts with the given prefix.
func (m *MockStore) GetMessagesBySenderNamePrefix(
	ctx context.Context, prefix string, limit int,
) ([]InboxMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []InboxMessage
	for _, msg := range m.messages {
		var senderName, senderProjectKey, senderGitBranch string
		if sender, ok := m.agents[msg.SenderID]; ok {
			senderName = sender.Name
			senderProjectKey = sender.ProjectKey
			senderGitBranch = sender.GitBranch
		}

		// Check if sender name starts with prefix.
		if len(senderName) >= len(prefix) && senderName[:len(prefix)] == prefix {
			// Look up recipient state — skip messages where
			// all recipients are archived or trashed.
			var found bool

			inbox := InboxMessage{
				Message:          msg,
				SenderName:       senderName,
				SenderProjectKey: senderProjectKey,
				SenderGitBranch:  senderGitBranch,
			}

			if recipients, ok := m.messageRecipients[msg.ID]; ok {
				for _, recip := range recipients {
					if recip.State == "archived" ||
						recip.State == "trash" {

						continue
					}

					inbox.State = recip.State
					inbox.ReadAt = recip.ReadAt
					inbox.AckedAt = recip.AckedAt
					inbox.SnoozedUntil = recip.SnoozedUntil
					found = true

					break
				}
			}

			if !found {
				continue
			}

			results = append(results, inbox)

			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			(len(s) > 0 && len(substr) > 0 &&
				(s[0:len(substr)] == substr ||
					contains(s[1:], substr))))
}

// GetMessageByIdempotencyKey retrieves a message by its idempotency key.
// Returns sql.ErrNoRows if no matching message is found.
func (m *MockStore) GetMessageByIdempotencyKey(
	ctx context.Context, key string,
) (Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, msg := range m.messages {
		if msg.IdempotencyKey == key {
			return msg, nil
		}
	}

	return Message{}, sql.ErrNoRows
}

// AgentStore implementation.

func (m *MockStore) CreateAgent(
	ctx context.Context, params CreateAgentParams,
) (Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agentsByName[params.Name]; exists {
		return Agent{}, fmt.Errorf("agent %s already exists", params.Name)
	}

	agent := Agent{
		ID:           m.nextAgentID,
		Name:         params.Name,
		ProjectKey:   params.ProjectKey,
		GitBranch:    params.GitBranch,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	m.nextAgentID++
	m.agents[agent.ID] = agent
	m.agentsByName[agent.Name] = agent.ID

	return agent, nil
}

func (m *MockStore) GetAgent(ctx context.Context, id int64) (Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, ok := m.agents[id]
	if !ok {
		return Agent{}, sql.ErrNoRows
	}
	return agent, nil
}

func (m *MockStore) GetAgentByName(
	ctx context.Context, name string,
) (Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.agentsByName[name]
	if !ok {
		return Agent{}, sql.ErrNoRows
	}
	return m.agents[id], nil
}

func (m *MockStore) GetAgentBySessionID(
	ctx context.Context, sessionID string,
) (Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.agentsBySession[sessionID]
	if !ok {
		return Agent{}, sql.ErrNoRows
	}
	return m.agents[id], nil
}

func (m *MockStore) ListAgents(ctx context.Context) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		result = append(result, agent)
	}
	return result, nil
}

func (m *MockStore) ListAgentsByProject(
	ctx context.Context, projectKey string,
) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Agent
	for _, agent := range m.agents {
		if agent.ProjectKey == projectKey {
			result = append(result, agent)
		}
	}
	return result, nil
}

func (m *MockStore) UpdateLastActive(
	ctx context.Context, id int64, ts time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}
	agent.LastActiveAt = ts
	m.agents[id] = agent
	return nil
}

func (m *MockStore) UpdateSession(
	ctx context.Context, id int64, sessionID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}

	// Remove old session mapping.
	if agent.CurrentSessionID != "" {
		delete(m.agentsBySession, agent.CurrentSessionID)
	}

	// Update agent.
	agent.CurrentSessionID = sessionID
	m.agents[id] = agent

	// Add new session mapping.
	if sessionID != "" {
		m.agentsBySession[sessionID] = id
	}

	return nil
}

// UpdateAgentName updates an agent's display name.
func (m *MockStore) UpdateAgentName(
	ctx context.Context, id int64, name string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}

	// Remove old name mapping.
	delete(m.agentsByName, agent.Name)

	// Update agent name.
	agent.Name = name
	m.agents[id] = agent
	m.agentsByName[name] = id

	return nil
}

// SearchAgents searches agents by name, project_key, or git_branch.
func (m *MockStore) SearchAgents(ctx context.Context,
	query string, limit int,
) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lowerQuery := strings.ToLower(query)

	var results []Agent
	for _, agent := range m.agents {
		nameMatch := strings.Contains(
			strings.ToLower(agent.Name), lowerQuery,
		)
		projectMatch := strings.Contains(
			strings.ToLower(agent.ProjectKey), lowerQuery,
		)
		branchMatch := strings.Contains(
			strings.ToLower(agent.GitBranch), lowerQuery,
		)

		if nameMatch || projectMatch || branchMatch {
			results = append(results, agent)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (m *MockStore) DeleteAgent(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}

	delete(m.agentsByName, agent.Name)
	if agent.CurrentSessionID != "" {
		delete(m.agentsBySession, agent.CurrentSessionID)
	}
	delete(m.agents, id)

	return nil
}

// TopicStore implementation.

func (m *MockStore) CreateTopic(
	ctx context.Context, params CreateTopicParams,
) (Topic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.topicsByName[params.Name]; exists {
		return Topic{}, fmt.Errorf("topic %s already exists", params.Name)
	}

	topic := Topic{
		ID:               m.nextTopicID,
		Name:             params.Name,
		TopicType:        params.TopicType,
		RetentionSeconds: params.RetentionSeconds,
		CreatedAt:        time.Now(),
	}

	m.nextTopicID++
	m.topics[topic.ID] = topic
	m.topicsByName[topic.Name] = topic.ID

	return topic, nil
}

func (m *MockStore) GetTopic(ctx context.Context, id int64) (Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topic, ok := m.topics[id]
	if !ok {
		return Topic{}, sql.ErrNoRows
	}
	return topic, nil
}

func (m *MockStore) GetTopicByName(
	ctx context.Context, name string,
) (Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.topicsByName[name]
	if !ok {
		return Topic{}, sql.ErrNoRows
	}
	return m.topics[id], nil
}

func (m *MockStore) GetOrCreateAgentInboxTopic(
	ctx context.Context, agentName string,
) (Topic, error) {
	topicName := fmt.Sprintf("inbox-%s", agentName)
	return m.GetOrCreateTopic(ctx, topicName, "inbox")
}

func (m *MockStore) GetOrCreateTopic(
	ctx context.Context, name, topicType string,
) (Topic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, exists := m.topicsByName[name]; exists {
		return m.topics[id], nil
	}

	topic := Topic{
		ID:        m.nextTopicID,
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now(),
	}

	m.nextTopicID++
	m.topics[topic.ID] = topic
	m.topicsByName[topic.Name] = topic.ID

	return topic, nil
}

func (m *MockStore) ListTopics(ctx context.Context) ([]Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Topic, 0, len(m.topics))
	for _, topic := range m.topics {
		result = append(result, topic)
	}
	return result, nil
}

func (m *MockStore) ListTopicsByType(
	ctx context.Context, topicType string,
) ([]Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Topic
	for _, topic := range m.topics {
		if topic.TopicType == topicType {
			result = append(result, topic)
		}
	}
	return result, nil
}

func (m *MockStore) CreateSubscription(
	ctx context.Context, agentID, topicID int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscriptions[topicID] == nil {
		m.subscriptions[topicID] = make(map[int64]bool)
	}
	m.subscriptions[topicID][agentID] = true

	return nil
}

func (m *MockStore) DeleteSubscription(
	ctx context.Context, agentID, topicID int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if subs, ok := m.subscriptions[topicID]; ok {
		delete(subs, agentID)
	}
	return nil
}

func (m *MockStore) ListSubscriptionsByAgent(
	ctx context.Context, agentID int64,
) ([]Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Topic
	for topicID, subscribers := range m.subscriptions {
		if subscribers[agentID] {
			if topic, ok := m.topics[topicID]; ok {
				result = append(result, topic)
			}
		}
	}
	return result, nil
}

func (m *MockStore) ListSubscriptionsByTopic(
	ctx context.Context, topicID int64,
) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Agent
	if subscribers, ok := m.subscriptions[topicID]; ok {
		for agentID := range subscribers {
			if agent, ok := m.agents[agentID]; ok {
				result = append(result, agent)
			}
		}
	}
	return result, nil
}

// ActivityStore implementation.

func (m *MockStore) CreateActivity(
	ctx context.Context, params CreateActivityParams,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	activity := Activity{
		ID:           m.nextActivityID,
		AgentID:      params.AgentID,
		ActivityType: params.ActivityType,
		Description:  params.Description,
		Metadata:     params.Metadata,
		CreatedAt:    time.Now(),
	}

	m.nextActivityID++
	m.activities = append(m.activities, activity)

	return nil
}

func (m *MockStore) ListRecentActivities(
	ctx context.Context, limit int,
) ([]Activity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit > len(m.activities) {
		limit = len(m.activities)
	}

	// Return most recent first.
	result := make([]Activity, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.activities[len(m.activities)-1-i]
	}
	return result, nil
}

func (m *MockStore) ListActivitiesByAgent(
	ctx context.Context, agentID int64, limit int,
) ([]Activity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Activity
	// Iterate in reverse for most recent first.
	for i := len(m.activities) - 1; i >= 0 && len(result) < limit; i-- {
		if m.activities[i].AgentID == agentID {
			result = append(result, m.activities[i])
		}
	}
	return result, nil
}

func (m *MockStore) ListActivitiesSince(
	ctx context.Context, since time.Time, limit int,
) ([]Activity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Activity
	for i := len(m.activities) - 1; i >= 0 && len(result) < limit; i-- {
		if m.activities[i].CreatedAt.After(since) {
			result = append(result, m.activities[i])
		}
	}
	return result, nil
}

func (m *MockStore) DeleteOldActivities(
	ctx context.Context, olderThan time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []Activity
	for _, a := range m.activities {
		if a.CreatedAt.After(olderThan) {
			remaining = append(remaining, a)
		}
	}
	m.activities = remaining
	return nil
}

// SessionStore implementation.

func (m *MockStore) CreateSessionIdentity(
	ctx context.Context, params CreateSessionIdentityParams,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessionIdentities[params.SessionID] = SessionIdentity{
		SessionID:    params.SessionID,
		AgentID:      params.AgentID,
		ProjectKey:   params.ProjectKey,
		GitBranch:    params.GitBranch,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	// Also update the agent's session mapping.
	if agent, ok := m.agents[params.AgentID]; ok {
		m.agentsBySession[params.SessionID] = params.AgentID
		agent.CurrentSessionID = params.SessionID
		m.agents[params.AgentID] = agent
	}

	return nil
}

func (m *MockStore) GetSessionIdentity(
	ctx context.Context, sessionID string,
) (SessionIdentity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	si, ok := m.sessionIdentities[sessionID]
	if !ok {
		return SessionIdentity{}, sql.ErrNoRows
	}
	return si, nil
}

func (m *MockStore) DeleteSessionIdentity(
	ctx context.Context, sessionID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessionIdentities, sessionID)
	delete(m.agentsBySession, sessionID)
	return nil
}

func (m *MockStore) ListSessionIdentitiesByAgent(
	ctx context.Context, agentID int64,
) ([]SessionIdentity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []SessionIdentity
	for _, si := range m.sessionIdentities {
		if si.AgentID == agentID {
			result = append(result, si)
		}
	}
	return result, nil
}

func (m *MockStore) UpdateSessionIdentityLastActive(
	ctx context.Context, sessionID string, ts time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	si, ok := m.sessionIdentities[sessionID]
	if !ok {
		return sql.ErrNoRows
	}
	si.LastActiveAt = ts
	m.sessionIdentities[sessionID] = si
	return nil
}

// Ensure MockStore implements Storage.
var _ Storage = (*MockStore)(nil)
