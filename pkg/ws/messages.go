package ws

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// Inbound types
	TypeChatMessage MessageType = "chat_message"
	TypeCancel      MessageType = "cancel"
	TypePing        MessageType = "ping"

	// Outbound types
	TypeToken        MessageType = "token"
	TypeToolCall     MessageType = "tool_call"
	TypeSubflowEvent MessageType = "subflow_event"
	TypeMemoryEvent  MessageType = "memory_event"
	TypeDone         MessageType = "done"
	TypeError        MessageType = "error"
	TypePong         MessageType = "pong"
	TypeStatus       MessageType = "status"
)

// InboundMessage represents a message received from a connected frontend client.
type InboundMessage struct {
	Type           MessageType     `json:"type"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Content        string          `json:"content,omitempty"`
	ToolsEnabled   []string        `json:"tools_enabled,omitempty"`
	SkillsEnabled  []string        `json:"skills_enabled,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
}

// OutboundMessage represents a message sent to connected clients.
type OutboundMessage struct {
	Type           MessageType `json:"type"`
	ConversationID string      `json:"conversation_id,omitempty"`
	Timestamp      time.Time   `json:"timestamp"`
	Payload        any         `json:"payload,omitempty"`
	Error          string      `json:"error,omitempty"`
}

// TokenPayload represents streaming token text chunks.
type TokenPayload struct {
	Delta    string `json:"delta"`
	FullText string `json:"full_text,omitempty"`
}

// ToolCallPayload represents status and details of a tool call execution.
type ToolCallPayload struct {
	ID        string `json:"id"`
	Tool      string `json:"tool"`
	Arguments any    `json:"arguments,omitempty"`
	Status    string `json:"status"` // "started", "completed", "failed"
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SubflowPayload represents progress of a parallel flow execution.
type SubflowPayload struct {
	FlowID      string `json:"flow_id"`
	TaskIndex   int    `json:"task_index"`
	TotalTasks  int    `json:"total_tasks"`
	TaskName    string `json:"task_name"`
	Status      string `json:"status"` // "started", "running", "completed", "failed"
	Summary     string `json:"summary,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
}

// MemoryPayload represents memory retrieval or storage event details.
type MemoryPayload struct {
	Action string `json:"action"` // "searched", "saved", "promoted"
	Query  string `json:"query,omitempty"`
	Count  int    `json:"count"`
	Items  any    `json:"items,omitempty"`
}
