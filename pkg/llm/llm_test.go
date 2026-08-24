package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/db"
	"go-harness/pkg/memory"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/ws"

	"github.com/sashabaranov/go-openai"
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

func TestLLMOAuth2Client(t *testing.T) {
	var tokenCalls int64
	var llmCalls int64
	var receivedAuthHeader string

	// 1. Mock OAuth2 Token Server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&tokenCalls, 1)
		clientID, clientSecret, ok := r.BasicAuth()
		if !ok {
			clientID = r.FormValue("client_id")
			clientSecret = r.FormValue("client_secret")
		}

		if clientID != "test-client-id" || clientSecret != "test-client-secret" {
			http.Error(w, "invalid client credentials", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oauth2-generated-access-token-999",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	// 2. Mock LLM Server checking Bearer token
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&llmCalls, 1)
		receivedAuthHeader = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "text/event-stream")
		chunk := `data: {"id":"chatcmpl-oauth","choices":[{"index":0,"delta":{"content":"OAuth2 works!"},"finish_reason":"stop"}]}`
		fmt.Fprintf(w, "%s\n\n", chunk)
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer llmServer.Close()

	cfg := &config.Config{
		LLMBaseURL:           llmServer.URL + "/v1",
		LLMModel:             "gpt-4o",
		LLMAuthType:          "oauth2",
		LLMOAuthClientID:     "test-client-id",
		LLMOAuthClientSecret: "test-client-secret",
		LLMOAuthTokenURL:     tokenServer.URL,
		LLMOAuthScopes:       "https://www.googleapis.com/auth/cloud-platform",
	}

	client := NewClient(cfg)
	stream, err := client.StreamChat(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("StreamChat with OAuth2 failed: %v", err)
	}
	defer stream.Close()

	delta, err := ReadStreamChunk(stream, make(map[int]*openai.ToolCall))
	if err != nil {
		t.Fatalf("ReadStreamChunk failed: %v", err)
	}

	if delta.Content != "OAuth2 works!" {
		t.Errorf("expected 'OAuth2 works!', got '%s'", delta.Content)
	}

	if tokenCalls == 0 {
		t.Errorf("expected OAuth2 token endpoint to be called")
	}

	if receivedAuthHeader != "Bearer oauth2-generated-access-token-999" {
		t.Errorf("expected Authorization 'Bearer oauth2-generated-access-token-999', got '%s'", receivedAuthHeader)
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

	toolCallMap := make(map[int]*openai.ToolCall)
	delta, err := ReadStreamChunk(stream, toolCallMap)
	if err != nil {
		t.Fatalf("ReadStreamChunk failed: %v", err)
	}
	if delta.Content != "Test token" {
		t.Errorf("expected 'Test token', got '%s'", delta.Content)
	}
}

func TestSlashCommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "slash-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	database, _ := db.Open(filepath.Join(tempDir, "test.db"))
	defer database.Close()

	skillsDir := filepath.Join(tempDir, "skills")
	_ = os.MkdirAll(filepath.Join(skillsDir, "deploy-k8s"), 0755)
	_ = os.WriteFile(filepath.Join(skillsDir, "deploy-k8s", "SKILL.md"), []byte(`---
name: deploy-k8s
description: Kubernetes deployment runbook
---
Step 1: Check kubectl cluster info.
Step 2: Apply k8s manifests.`), 0644)

	skillsLoader := skills.NewLoader(skillsDir)
	_ = skillsLoader.Load()

	toolsReg := tools.NewRegistry()
	skillsLoader.RegisterSkillTools(toolsReg)

	cfg := &config.Config{
		LLMBaseURL: "http://mock/v1",
		LLMModel:   "gpt-4o",
	}

	orchestrator := NewOrchestrator(cfg, database, nil, skillsLoader, toolsReg, nil)

	// 1. Test /help
	helpMsg := ws.InboundMessage{
		Type:           ws.TypeChatMessage,
		ConversationID: "conv-help",
		Content:        "/help",
	}
	orchestrator.HandleChatMessage(context.Background(), nil, helpMsg)
	history, err := database.GetMessages("conv-help")
	if err != nil || len(history) < 2 {
		t.Fatalf("expected /help response saved in DB, got %d messages", len(history))
	}
	if history[0].Content != "/help" || history[1].Role != "assistant" {
		t.Errorf("unexpected help message history: %+v", history)
	}

	// 2. Test /skills
	skillsMsg := ws.InboundMessage{
		Type:           ws.TypeChatMessage,
		ConversationID: "conv-skills",
		Content:        "/skills",
	}
	orchestrator.HandleChatMessage(context.Background(), nil, skillsMsg)
	historySkills, err := database.GetMessages("conv-skills")
	if err != nil || len(historySkills) < 2 {
		t.Fatalf("expected /skills response saved in DB, got %d messages", len(historySkills))
	}
	if historySkills[1].Role != "assistant" {
		t.Errorf("unexpected skills message history: %+v", historySkills)
	}

	// 3. Test /tools
	toolsMsg := ws.InboundMessage{
		Type:           ws.TypeChatMessage,
		ConversationID: "conv-tools",
		Content:        "/tools",
	}
	orchestrator.HandleChatMessage(context.Background(), nil, toolsMsg)
	historyTools, err := database.GetMessages("conv-tools")
	if err != nil || len(historyTools) < 2 {
		t.Fatalf("expected /tools response saved in DB, got %d messages", len(historyTools))
	}
	if historyTools[1].Role != "assistant" {
		t.Errorf("unexpected tools message history: %+v", historyTools)
	}

	// 4. Test unknown command
	unknownMsg := ws.InboundMessage{
		Type:           ws.TypeChatMessage,
		ConversationID: "conv-unknown",
		Content:        "/unknown_cmd",
	}
	orchestrator.HandleChatMessage(context.Background(), nil, unknownMsg)
	historyUnknown, err := database.GetMessages("conv-unknown")
	if err != nil || len(historyUnknown) < 2 {
		t.Fatalf("expected unknown response saved in DB, got %d messages", len(historyUnknown))
	}
}
