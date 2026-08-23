package db

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a saved chat session.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents an individual turn in a conversation.
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"` // "user", "assistant", "system", "tool"
	Content        string    `json:"content"`
	ToolCalls      string    `json:"tool_calls,omitempty"` // JSON serialized tool calls if any
	CreatedAt      time.Time `json:"created_at"`
}

// CreateConversation creates a new conversation with a title.
func (d *DB) CreateConversation(title string) (*Conversation, error) {
	if title == "" {
		title = "New Conversation"
	}
	c := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	query := `INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`
	_, err := d.db.Exec(query, c.ID, c.Title, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert conversation: %w", err)
	}
	return c, nil
}

// GetConversation fetches conversation metadata by ID.
func (d *DB) GetConversation(id string) (*Conversation, error) {
	query := `SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?`
	row := d.db.QueryRow(query, id)

	var c Conversation
	if err := row.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListConversations returns all conversations ordered by recent update.
func (d *DB) ListConversations() ([]Conversation, error) {
	query := `SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

// DeleteConversation removes a conversation and its messages.
func (d *DB) DeleteConversation(id string) error {
	query := `DELETE FROM conversations WHERE id = ?`
	_, err := d.db.Exec(query, id)
	return err
}

// UpdateConversationTitle updates the title of a conversation.
func (d *DB) UpdateConversationTitle(id, title string) error {
	query := `UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, title, time.Now().UTC(), id)
	return err
}

// AddMessage appends a message to a conversation.
func (d *DB) AddMessage(m Message) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}

	// Ensure conversation exists
	_, err := d.GetConversation(m.ConversationID)
	if err != nil {
		// Auto create if missing
		title := m.Content
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		if title == "" {
			title = "New Conversation"
		}
		now := time.Now().UTC()
		_, _ = d.db.Exec(`INSERT OR IGNORE INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			m.ConversationID, title, now, now)
	}

	query := `INSERT INTO messages (id, conversation_id, role, content, tool_calls, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = d.db.Exec(query, m.ID, m.ConversationID, m.Role, m.Content, m.ToolCalls, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Touch updated_at
	_, _ = d.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, time.Now().UTC(), m.ConversationID)
	return nil
}

// GetMessages retrieves all messages for a conversation ordered chronologically.
func (d *DB) GetMessages(conversationID string) ([]Message, error) {
	query := `SELECT id, conversation_id, role, content, tool_calls, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`
	rows, err := d.db.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCalls, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// ExportConversationCSV generates a CSV representation of the conversation messages.
func (d *DB) ExportConversationCSV(conversationID string) (string, error) {
	messages, err := d.GetMessages(conversationID)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write CSV Header
	if err := writer.Write([]string{"ID", "ConversationID", "Role", "Content", "ToolCalls", "CreatedAt"}); err != nil {
		return "", err
	}

	for _, m := range messages {
		record := []string{
			m.ID,
			m.ConversationID,
			m.Role,
			m.Content,
			m.ToolCalls,
			m.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}
