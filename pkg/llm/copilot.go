package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/tracing"

	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
)

// CopilotClient executes prompts via the GitHub Copilot CLI binary.
type CopilotClient struct {
	cfg *config.Config
}

// NewCopilotClient creates a new CopilotClient.
func NewCopilotClient(cfg *config.Config) *CopilotClient {
	return &CopilotClient{
		cfg: cfg,
	}
}

// GetBinaryPath resolves the copilot binary path from config or $PATH.
func (c *CopilotClient) GetBinaryPath() (string, error) {
	snap := c.cfg.Clone()
	bin := snap.CopilotBinary
	if bin == "" {
		bin = "copilot"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("GitHub Copilot CLI binary '%s' not found on PATH: %w. Please install the Copilot CLI (https://github.com/github/copilot-cli) or configure COPILOT_BINARY", bin, err)
	}
	return path, nil
}

// CheckAuth verifies that the copilot binary is available and authenticated.
func (c *CopilotClient) CheckAuth(ctx context.Context) (bool, string, string, error) {
	binPath, err := c.GetBinaryPath()
	if err != nil {
		return false, "", "", err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, binPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, binPath, "", fmt.Errorf("failed to run '%s --version': %w (%s)", binPath, err, strings.TrimSpace(string(out)))
	}

	versionStr := strings.TrimSpace(string(out))
	return true, binPath, versionStr, nil
}

// FormatPrompt converts OpenAI-style chat messages and tools into a clean single prompt for Copilot CLI.
func (c *CopilotClient) FormatPrompt(messages []openai.ChatCompletionMessage, tools []openai.Tool) string {
	var sb strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case openai.ChatMessageRoleSystem:
			sb.WriteString("=== SYSTEM INSTRUCTIONS ===\n")
			sb.WriteString(strings.TrimSpace(msg.Content))
			sb.WriteString("\n\n")
		case openai.ChatMessageRoleUser:
			sb.WriteString("=== USER MESSAGE ===\n")
			sb.WriteString(strings.TrimSpace(msg.Content))
			sb.WriteString("\n\n")
		case openai.ChatMessageRoleAssistant:
			sb.WriteString("=== ASSISTANT PREVIOUS RESPONSE ===\n")
			sb.WriteString(strings.TrimSpace(msg.Content))
			if len(msg.ToolCalls) > 0 {
				sb.WriteString("\n[Executed Tools:]\n")
				for _, tc := range msg.ToolCalls {
					sb.WriteString(fmt.Sprintf("- %s(%s)\n", tc.Function.Name, tc.Function.Arguments))
				}
			}
			sb.WriteString("\n\n")
		case openai.ChatMessageRoleTool:
			sb.WriteString(fmt.Sprintf("=== TOOL RESPONSE (Call ID: %s) ===\n", msg.ToolCallID))
			sb.WriteString(strings.TrimSpace(msg.Content))
			sb.WriteString("\n\n")
		}
	}

	if len(tools) > 0 {
		sb.WriteString("=== AVAILABLE AGENT TOOLS ===\n")
		sb.WriteString("You have access to the following agent tools in this harness if needed:\n")
		for _, t := range tools {
			sb.WriteString(fmt.Sprintf("- Tool: `%s` - %s\n", t.Function.Name, t.Function.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== INSTRUCTION ===\n")
	sb.WriteString("Provide a direct, thorough, and high-quality response to the user's latest message based on the context above.\n")

	return sb.String()
}

// StreamChat executes the Copilot CLI in non-interactive JSON streaming mode.
func (c *CopilotClient) StreamChat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (ChatStream, error) {
	binPath, err := c.GetBinaryPath()
	if err != nil {
		return nil, err
	}

	prompt := c.FormatPrompt(messages, tools)

	snap := c.cfg.Clone()
	model := snap.LLMModel

	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--no-custom-instructions",
		"--no-auto-update",
		"--available-tools", "",
	}

	// Add model flag if explicitly specified and not default/auto
	if model != "" && !strings.EqualFold(model, "default") && !strings.EqualFold(model, "auto") {
		args = append(args, "--model", model)
	}

	timeoutSec := snap.CopilotTimeout
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)

	cmd := exec.CommandContext(execCtx, binPath, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open stdout pipe for copilot CLI: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		cancel()
		tracing.TraceLLMCall(ctx, model, "copilot_start_error", time.Since(startTime), 0, 0, err)
		return nil, fmt.Errorf("failed to start copilot CLI process: %w", err)
	}

	stream := &copilotStream{
		cmd:       cmd,
		stdout:    stdoutPipe,
		stderrBuf: &stderrBuf,
		cancel:    cancel,
		chunks:    make(chan streamItem, 100),
		done:      make(chan struct{}),
		model:     model,
		startTime: startTime,
		ctx:       ctx,
	}

	go stream.readStdout()

	return stream, nil
}

type streamItem struct {
	resp openai.ChatCompletionStreamResponse
	err  error
}

type copilotStream struct {
	cmd       *exec.Cmd
	stdout    io.ReadCloser
	stderrBuf *bytes.Buffer
	cancel    context.CancelFunc
	chunks    chan streamItem
	done      chan struct{}
	model     string
	startTime time.Time
	ctx       context.Context

	mu          sync.Mutex
	closed      bool
	readStarted bool
	totalPrompt int
	totalComp   int
}

// Recv returns the next streaming chunk delta from the Copilot CLI.
func (s *copilotStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	item, ok := <-s.chunks
	if !ok {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	if item.err != nil {
		return openai.ChatCompletionStreamResponse{}, item.err
	}
	return item.resp, nil
}

// Close terminates the stream and releases resources.
func (s *copilotStream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	s.cancel()
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	<-s.done
	return nil
}

type copilotEvent struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	ExitCode  *int            `json:"exitCode,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

type messageDeltaData struct {
	DeltaContent string `json:"deltaContent"`
	MessageID    string `json:"messageId"`
}

type messageData struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

type modelCallSuccessData struct {
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ResponseUsage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"responseUsage"`
}

func (s *copilotStream) readStdout() {
	defer close(s.done)
	defer close(s.chunks)

	scanner := bufio.NewScanner(s.stdout)
	// Allow large tokens / lines
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var emittedDeltas bool
	var fullMessageContent string
	var rawOutputBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		rawOutputBuilder.Write(line)
		rawOutputBuilder.WriteByte('\n')

		var event copilotEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Non-JSON line from copilot (e.g. plain text or diagnostic)
			continue
		}

		switch event.Type {
		case "assistant.message_delta":
			var d messageDeltaData
			if err := json.Unmarshal(event.Data, &d); err == nil && d.DeltaContent != "" {
				emittedDeltas = true
				s.chunks <- streamItem{
					resp: openai.ChatCompletionStreamResponse{
						Choices: []openai.ChatCompletionStreamChoice{
							{
								Delta: openai.ChatCompletionStreamChoiceDelta{
									Content: d.DeltaContent,
								},
							},
						},
					},
				}
			}

		case "assistant.message":
			var m messageData
			if err := json.Unmarshal(event.Data, &m); err == nil && m.Content != "" {
				fullMessageContent = m.Content
			}

		case "model.model_call_success":
			var u modelCallSuccessData
			if err := json.Unmarshal(event.Data, &u); err == nil {
				if u.Usage.PromptTokens > 0 {
					s.totalPrompt = u.Usage.PromptTokens
				} else if u.ResponseUsage.PromptTokens > 0 {
					s.totalPrompt = u.ResponseUsage.PromptTokens
				}
				if u.Usage.CompletionTokens > 0 {
					s.totalComp = u.Usage.CompletionTokens
				} else if u.ResponseUsage.CompletionTokens > 0 {
					s.totalComp = u.ResponseUsage.CompletionTokens
				}
			}
		}
	}

	// Wait for process exit
	cmdErr := s.cmd.Wait()
	duration := time.Since(s.startTime)

	if cmdErr != nil {
		stderrStr := strings.TrimSpace(s.stderrBuf.String())
		rawOutStr := strings.TrimSpace(rawOutputBuilder.String())
		combined := stderrStr
		if combined == "" {
			combined = rawOutStr
		}

		// Handle specific authentication failure diagnostics
		if strings.Contains(combined, "No authentication information found") ||
			strings.Contains(combined, "not authenticated") ||
			strings.Contains(combined, "/login") {
			authErr := errors.New("GitHub Copilot CLI authentication failed: No authentication information found. Run 'copilot login' or set COPILOT_GITHUB_TOKEN/GH_TOKEN, and try again")
			tracing.TraceLLMCall(s.ctx, s.model, "copilot_auth_error", duration, 0, 0, authErr)
			s.chunks <- streamItem{err: authErr}
			return
		}

		execErr := fmt.Errorf("Copilot CLI process exited with error: %w (Output: %s)", cmdErr, combined)
		tracing.TraceLLMCall(s.ctx, s.model, "copilot_exec_error", duration, 0, 0, execErr)
		s.chunks <- streamItem{err: execErr}
		return
	}

	// Fallback if no delta was received but full content exists
	if !emittedDeltas && fullMessageContent != "" {
		s.chunks <- streamItem{
			resp: openai.ChatCompletionStreamResponse{
				Choices: []openai.ChatCompletionStreamChoice{
					{
						Delta: openai.ChatCompletionStreamChoiceDelta{
							Content: fullMessageContent,
						},
					},
				},
			},
		}
	}

	log.Debug().
		Str("model", s.model).
		Dur("duration", duration).
		Int("prompt_tokens", s.totalPrompt).
		Int("completion_tokens", s.totalComp).
		Msg("Copilot CLI chat completed")

	tracing.TraceLLMCall(s.ctx, s.model, "copilot_chat", duration, s.totalPrompt, s.totalComp, nil)
}
