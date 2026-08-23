package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/db"
	"go-harness/pkg/memory"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/ws"
)

func TestLLMOrchestratorMock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "llm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock OpenAI streaming server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// SSE chunk 1
		chunk1 := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello! "},"finish_reason":null}]}`
		fmt.Fprintf(w, "%s\n\n", chunk1)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)

		// SSE chunk 2
		chunk2 := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"I am your Go Agent."},"finish_reason":"stop"}]}`
		fmt.Fprintf(w, "%s\n\n", chunk2)
		flusher.Flush()

		// End
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		LLMBaseURL: mockServer.URL + "/v1",
		LLMModel:   "gpt-4o",
		LLMAPIKey:  "test-api-key",
	}

	database, _ := db.Open(filepath.Join(tempDir, "test.db"))
	defer database.Close()

	memMgr := memory.NewManager(database)
	skillsLoader := skills.NewLoader(filepath.Join(tempDir, "skills"))
	toolsReg := tools.NewRegistry()

	orchestrator := NewOrchestrator(cfg, database, memMgr, skillsLoader, toolsReg, nil)

	// Test Client instantiation
	if orchestrator.client == nil {
		t.Fatal("expected non-nil LLM client")
	}

	// Test chat handling with memory & DB save
	inbound := ws.InboundMessage{
		Type:           ws.TypeChatMessage,
		ConversationID: "conv-123",
		Content:        "Hello there",
	}

	orchestrator.HandleChatMessage(context.Background(), nil, inbound)

	// Verify conversation was stored in SQLite
	history, err := database.GetMessages("conv-123")
	if err != nil || len(history) < 2 {
		t.Fatalf("expected stored user & assistant messages, got %d messages: %v", len(history), err)
	}

	if history[0].Content != "Hello there" {
		t.Errorf("expected user message 'Hello there', got '%s'", history[0].Content)
	}

	if history[1].Content != "Hello! I am your Go Agent." {
		t.Errorf("expected assistant message 'Hello! I am your Go Agent.', got '%s'", history[1].Content)
	}
}

func TestReadStreamChunk(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := `data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Test token"},"finish_reason":null}]}`
		fmt.Fprintf(w, "%s\n\n", chunk)
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		LLMBaseURL: mockServer.URL + "/v1",
		LLMModel:   "gpt-4o",
	}

	client := NewClient(cfg)
	stream, err := client.StreamChat(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	defer stream.Close()

	toolCallMap := make(map[int]*json.RawMessage)
	_ = toolCallMap
}
