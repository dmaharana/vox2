package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go-harness/pkg/config"

	"github.com/sashabaranov/go-openai"
)

func TestCopilotPromptFormatting(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "copilot",
		LLMModel:    "gpt-4o",
	}

	copilotClient := NewCopilotClient(cfg)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are Vox2 AI assistant.",
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "What is the status of the project?",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "Let me check the files.",
			ToolCalls: []openai.ToolCall{
				{
					ID:   "call-1",
					Type: "function",
					Function: openai.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				},
			},
		},
		{
			Role:       openai.ChatMessageRoleTool,
			Content:    "package main\nfunc main() {}",
			ToolCallID: "call-1",
		},
	}

	tools := []openai.Tool{
		{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        "read_file",
				Description: "Read file contents from disk",
			},
		},
	}

	prompt := copilotClient.FormatPrompt(messages, tools)

	if !strings.Contains(prompt, "=== SYSTEM INSTRUCTIONS ===") || !strings.Contains(prompt, "You are Vox2 AI assistant.") {
		t.Errorf("expected system prompt in formatted output, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "=== USER MESSAGE ===") || !strings.Contains(prompt, "What is the status of the project?") {
		t.Errorf("expected user message in formatted output, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "read_file({\"path\":\"main.go\"})") {
		t.Errorf("expected executed tool call in formatted output, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "=== TOOL RESPONSE (Call ID: call-1) ===") {
		t.Errorf("expected tool response in formatted output, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "=== AVAILABLE AGENT TOOLS ===") || !strings.Contains(prompt, "Tool: `read_file`") {
		t.Errorf("expected available tools in formatted output, got:\n%s", prompt)
	}
}

func TestCopilotStreamParsing(t *testing.T) {
	// Sample JSONL output emitted by official Copilot CLI
	jsonLines := `{"type":"session.skills_loaded","data":{"skills":[]}}
{"type":"user.message","data":{"content":"hello"}}
{"type":"assistant.message_start","data":{"messageId":"msg-1"}}
{"type":"assistant.message_delta","data":{"messageId":"msg-1","deltaContent":"Hello"}}
{"type":"assistant.message_delta","data":{"messageId":"msg-1","deltaContent":" world"}}
{"type":"assistant.message_delta","data":{"messageId":"msg-1","deltaContent":"!"}}
{"type":"model.model_call_success","data":{"usage":{"prompt_tokens":150,"completion_tokens":25,"total_tokens":175}}}
{"type":"assistant.message","data":{"messageId":"msg-1","content":"Hello world!"}}
{"type":"result","exitCode":0}
`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	readCloser := io.NopCloser(strings.NewReader(jsonLines))
	var stderrBuf bytes.Buffer

	stream := &copilotStream{
		cmd:       exec.Command("true"),
		stdout:    readCloser,
		stderrBuf: &stderrBuf,
		cancel:    cancel,
		chunks:    make(chan streamItem, 100),
		done:      make(chan struct{}),
		model:     "gpt-4o",
		startTime: time.Now(),
		ctx:       ctx,
	}

	// Start reading stdout simulated JSON lines
	_ = stream.cmd.Start()
	go stream.readStdout()

	var assembled strings.Builder
	toolCallMap := make(map[int]*openai.ToolCall)

	for {
		delta, err := ReadStreamChunk(stream, toolCallMap)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("unexpected read error: %v", err)
		}
		if delta.Content != "" {
			assembled.WriteString(delta.Content)
		}
	}

	if assembled.String() != "Hello world!" {
		t.Errorf("expected assembled 'Hello world!', got '%s'", assembled.String())
	}

	if stream.totalPrompt != 150 || stream.totalComp != 25 {
		t.Errorf("expected tokens 150 prompt / 25 comp, got %d / %d", stream.totalPrompt, stream.totalComp)
	}

	_ = stream.Close()
}

func TestCopilotStreamAuthFailureHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	readCloser := io.NopCloser(strings.NewReader(""))
	var stderrBuf bytes.Buffer
	stderrBuf.WriteString("Error: No authentication information found.\nTo authenticate, run 'copilot login'")

	// Use false command so exit code is non-zero
	cmd := exec.Command("false")
	_ = cmd.Start()

	stream := &copilotStream{
		cmd:       cmd,
		stdout:    readCloser,
		stderrBuf: &stderrBuf,
		cancel:    cancel,
		chunks:    make(chan streamItem, 100),
		done:      make(chan struct{}),
		model:     "gpt-4o",
		startTime: time.Now(),
		ctx:       ctx,
	}

	go stream.readStdout()

	_, err := ReadStreamChunk(stream, make(map[int]*openai.ToolCall))
	if err == nil {
		t.Fatal("expected authentication error, got nil")
	}

	if !strings.Contains(err.Error(), "copilot login") {
		t.Errorf("expected error mentioning 'copilot login', got: %v", err)
	}

	_ = stream.Close()
}

func TestCopilotClientRouting(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "copilot",
		LLMModel:    "gpt-4o",
	}

	client := NewClient(cfg)
	if !client.IsCopilotProvider() {
		t.Error("expected IsCopilotProvider to be true for 'copilot'")
	}

	cfg.LLMProvider = "github-copilot"
	if !client.IsCopilotProvider() {
		t.Error("expected IsCopilotProvider to be true for 'github-copilot'")
	}

	cfg.LLMProvider = "copilot-cli"
	if !client.IsCopilotProvider() {
		t.Error("expected IsCopilotProvider to be true for 'copilot-cli'")
	}

	cfg.LLMProvider = "openai"
	if client.IsCopilotProvider() {
		t.Error("expected IsCopilotProvider to be false for 'openai'")
	}
}

func TestCopilotLiveIntegration(t *testing.T) {
	cfg := &config.Config{
		LLMProvider:   "copilot",
		LLMModel:      "",
		CopilotBinary: "copilot",
	}

	client := NewCopilotClient(cfg)
	ok, binPath, versionStr, err := client.CheckAuth(context.Background())
	if err != nil || !ok {
		t.Skipf("Skipping live Copilot CLI integration test: copilot not available or not authenticated (%v)", err)
		return
	}

	t.Logf("Copilot binary found at %s: %s", binPath, versionStr)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Reply with the single word: INTEGRATION_OK",
		},
	}

	stream, err := client.StreamChat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	defer stream.Close()

	var response strings.Builder
	toolCallMap := make(map[int]*openai.ToolCall)

	for {
		delta, err := ReadStreamChunk(stream, toolCallMap)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("ReadStreamChunk error: %v", err)
		}
		if delta.Content != "" {
			response.WriteString(delta.Content)
		}
	}

	t.Logf("Live Copilot response: %s", response.String())
	if !strings.Contains(response.String(), "INTEGRATION_OK") {
		t.Errorf("expected response to contain 'INTEGRATION_OK', got '%s'", response.String())
	}
}
