package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/tracing"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ChatStream defines the streaming interface for chat completions across providers.
type ChatStream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

// Client wraps sashabaranov/go-openai client and Copilot CLI client with configuration, OAuth2 auto-refresh, and tracing.
type Client struct {
	mu                sync.RWMutex
	cfg               *config.Config
	provider          string
	copilot           *CopilotClient
	openAI            *openai.Client
	baseURL           string
	model             string
	apiKey            string
	authType          string
	oauthClientID     string
	oauthClientSecret string
	oauthTokenURL     string
	oauthScopes       string
	temperature       float64
	maxTokens         int
}

// NewClient creates a new LLM client.
func NewClient(cfg *config.Config) *Client {
	c := &Client{
		cfg:     cfg,
		copilot: NewCopilotClient(cfg),
	}
	c.refresh()
	return c
}

func (c *Client) refresh() {
	snap := c.cfg.Clone()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.provider = snap.LLMProvider
	c.baseURL = snap.LLMBaseURL
	c.model = snap.LLMModel
	c.apiKey = snap.LLMAPIKey
	c.authType = snap.LLMAuthType
	c.oauthClientID = snap.LLMOAuthClientID
	c.oauthClientSecret = snap.LLMOAuthClientSecret
	c.oauthTokenURL = snap.LLMOAuthTokenURL
	c.oauthScopes = snap.LLMOAuthScopes
	c.temperature = snap.LLMTemperature
	c.maxTokens = snap.LLMMaxTokens

	if c.baseURL == "" {
		c.baseURL = "https://api.openai.com/v1"
	}
	if c.model == "" {
		c.model = "gpt-4o"
	}

	apiKey := c.apiKey
	if apiKey == "" && (c.authType == "oauth2" || (c.oauthClientID != "" && c.oauthClientSecret != "")) {
		// Placeholder for go-openai validator when OAuth2 client handles the token header
		apiKey = "oauth2-token"
	}

	clientCfg := openai.DefaultConfig(apiKey)
	if c.baseURL != "" {
		clientCfg.BaseURL = strings.TrimRight(c.baseURL, "/")
	}

	// Configure OAuth2 if auth_type is oauth2 or client credentials are provided
	if c.authType == "oauth2" || (c.oauthClientID != "" && c.oauthClientSecret != "") {
		tokenURL := c.oauthTokenURL
		if tokenURL == "" {
			tokenURL = "https://oauth2.googleapis.com/token"
		}

		var scopes []string
		if c.oauthScopes != "" {
			for _, s := range strings.FieldsFunc(c.oauthScopes, func(r rune) bool {
				return r == ' ' || r == ',' || r == ';'
			}) {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					scopes = append(scopes, trimmed)
				}
			}
		}

		ccConfig := &clientcredentials.Config{
			ClientID:     c.oauthClientID,
			ClientSecret: c.oauthClientSecret,
			TokenURL:     tokenURL,
			Scopes:       scopes,
		}

		// TokenSource handles token caching and automatic renewal before expiry
		ctx := context.Background()
		ts := ccConfig.TokenSource(ctx)
		clientCfg.HTTPClient = &http.Client{
			Transport: &oauth2.Transport{
				Source: ts,
				Base:   http.DefaultTransport,
			},
			Timeout: 120 * time.Second,
		}
	}

	c.openAI = openai.NewClientWithConfig(clientCfg)
}

// IsCopilotProvider checks if the active provider is GitHub Copilot CLI.
func (c *Client) IsCopilotProvider() bool {
	snap := c.cfg.Clone()
	p := strings.ToLower(strings.TrimSpace(snap.LLMProvider))
	return p == "copilot" || p == "github-copilot" || p == "copilot-cli"
}

// GetCopilotClient returns the Copilot CLI client.
func (c *Client) GetCopilotClient() *CopilotClient {
	return c.copilot
}

// StreamChat sends a chat completion request and returns a streaming response.
func (c *Client) StreamChat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (ChatStream, error) {
	c.refresh()

	if c.IsCopilotProvider() {
		return c.copilot.StreamChat(ctx, messages, tools)
	}

	c.mu.RLock()
	model := c.model
	temp := c.temperature
	maxTokens := c.maxTokens
	cli := c.openAI
	c.mu.RUnlock()

	req := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: float32(temp),
		MaxTokens:   maxTokens,
		Stream:      true,
	}

	if len(tools) > 0 {
		req.Tools = tools
		req.ToolChoice = "auto"
	}

	startTime := time.Now()
	stream, err := cli.CreateChatCompletionStream(ctx, req)
	if err != nil {
		tracing.TraceLLMCall(ctx, model, "stream_init_error", time.Since(startTime), 0, 0, err)
		return nil, err
	}

	return stream, nil
}

// StreamDelta represents assembled chunk deltas and tool call parts.
type StreamDelta struct {
	Content   string
	ToolCalls []openai.ToolCall
}

// ReadStreamChunk reads and accumulates tool calls from a streaming delta.
func ReadStreamChunk(stream ChatStream, toolCallMap map[int]*openai.ToolCall) (StreamDelta, error) {
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
