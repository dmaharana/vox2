package llm

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/tracing"

	"github.com/sashabaranov/go-openai"
)

// Client wraps sashabaranov/go-openai client with configuration and tracing.
type Client struct {
	cfg        *config.Config
	openAI     *openai.Client
	baseURL    string
	model      string
	apiKey     string
	temperature float64
	maxTokens  int
}

// NewClient creates a new OpenAI-compatible LLM client.
func NewClient(cfg *config.Config) *Client {
	c := &Client{
		cfg: cfg,
	}
	c.refresh()
	return c
}

func (c *Client) refresh() {
	snap := c.cfg.Clone()
	c.baseURL = snap.LLMBaseURL
	c.model = snap.LLMModel
	c.apiKey = snap.LLMAPIKey
	c.temperature = snap.LLMTemperature
	c.maxTokens = snap.LLMMaxTokens

	if c.baseURL == "" {
		c.baseURL = "https://api.openai.com/v1"
	}
	if c.model == "" {
		c.model = "gpt-4o"
	}

	clientCfg := openai.DefaultConfig(c.apiKey)
	if c.baseURL != "" {
		clientCfg.BaseURL = strings.TrimRight(c.baseURL, "/")
		if !strings.HasSuffix(clientCfg.BaseURL, "/v1") && !strings.Contains(clientCfg.BaseURL, "openai.com") {
			// Some local endpoints (like Ollama or vLLM) support direct base URLs
		}
	}

	c.openAI = openai.NewClientWithConfig(clientCfg)
}

// StreamChat sends a chat completion request and returns a streaming response.
func (c *Client) StreamChat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (*openai.ChatCompletionStream, error) {
	c.refresh()

	req := openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: float32(c.temperature),
		MaxTokens:   c.maxTokens,
		Stream:      true,
	}

	if len(tools) > 0 {
		req.Tools = tools
		req.ToolChoice = "auto"
	}

	startTime := time.Now()
	stream, err := c.openAI.CreateChatCompletionStream(ctx, req)
	if err != nil {
		tracing.TraceLLMCall(ctx, c.model, "stream_init_error", time.Since(startTime), 0, 0, err)
		return nil, err
	}

	return stream, nil
}

// CompletionStreamChunk represents assembled chunk deltas and tool call parts.
type StreamDelta struct {
	Content   string
	ToolCalls []openai.ToolCall
}

// ReadStreamChunk reads and accumulates tool calls from a streaming delta.
func ReadStreamChunk(stream *openai.ChatCompletionStream, toolCallMap map[int]*openai.ToolCall) (StreamDelta, error) {
	resp, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return StreamDelta{}, io.EOF
		}
		return StreamDelta{}, err
	}

	if len(resp.Choices) == 0 {
		return StreamDelta{}, nil
	}

	choice := resp.Choices[0]
	delta := choice.Delta

	var deltaContent string
	if delta.Content != "" {
		deltaContent = delta.Content
	}

	// Assemble streaming tool calls
	for _, tc := range delta.ToolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}

		existing, exists := toolCallMap[idx]
		if !exists {
			toolCallMap[idx] = &openai.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		} else {
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Function.Name += tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
	}

	return StreamDelta{
		Content: deltaContent,
	}, nil
}
