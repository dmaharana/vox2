package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/db"
	"go-harness/pkg/tools"
	"go-harness/pkg/tracing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai/jsonschema"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ServerConfig defines the connection parameters for an MCP server.
type ServerConfig struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Transport            string            `json:"transport"` // "stdio", "sse", "http", "streamable"
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	Cwd                  string            `json:"cwd,omitempty"`
	URL                  string            `json:"url,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"` // Custom HTTP headers for SSE/HTTP transport
	AuthType             string            `json:"auth_type,omitempty"` // "none", "headers", "oauth2", "oauth2_google"
	OAuthClientID        string            `json:"oauth_client_id,omitempty"`
	OAuthClientSecret    string            `json:"oauth_client_secret,omitempty"`
	HasOAuthClientSecret bool              `json:"has_oauth_client_secret,omitempty"`
	OAuthTokenURL        string            `json:"oauth_token_url,omitempty"`
	OAuthScopes          string            `json:"oauth_scopes,omitempty"`
	OAuthAccessToken     string            `json:"oauth_access_token,omitempty"`
	HasOAuthAccessToken  bool              `json:"has_oauth_access_token,omitempty"`
	TimeoutSeconds       int               `json:"timeout_seconds,omitempty"` // Per-call timeout in seconds (default 60s)
	Enabled              bool              `json:"enabled"`
	Status               string            `json:"status"` // "connected", "disconnected", "error"
	LastError            string            `json:"last_error,omitempty"`
}

// MCPImageContent represents structured image data returned by an MCP tool.
type MCPImageContent struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"` // Base64 encoded
}

// MCPToolResult represents a normalized, clean MCP tool execution result.
type MCPToolResult struct {
	Content string            `json:"content"`
	Text    string            `json:"text"`
	Images  []MCPImageContent `json:"images,omitempty"`
	IsError bool              `json:"is_error"`
}

type headerRoundTripper struct {
	headers map[string]string
	rt      http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for k, v := range h.headers {
		cloned.Header.Set(k, v)
	}
	rt := h.rt
	if rt == nil {
		rt = http.DefaultTransport
	}
	return rt.RoundTrip(cloned)
}

type activeSession struct {
	session     *mcp.ClientSession
	tools       []mcp.Tool
	prompts     []*mcp.Prompt
	resources   []*mcp.Resource
	cancel      context.CancelFunc
	connectedAt time.Time
}

// Manager handles connections, life-cycles, health monitoring, and tool registration for MCP servers.
type Manager struct {
	mu            sync.RWMutex
	servers       map[string]*ServerConfig
	sessions      map[string]*activeSession
	toolsRegistry *tools.Registry
	db            *db.DB
}

// NewManager creates a new MCP Manager instance.
func NewManager(toolsReg *tools.Registry, database ...*db.DB) *Manager {
	var d *db.DB
	if len(database) > 0 {
		d = database[0]
	}
	mgr := &Manager{
		servers:       make(map[string]*ServerConfig),
		sessions:      make(map[string]*activeSession),
		toolsRegistry: toolsReg,
		db:            d,
	}

	// Register universal read_mcp_resource tool if registry is provided
	if toolsReg != nil {
		mgr.registerResourceTool()
	}

	return mgr
}

// LoadFromDB loads all persistent MCP server configurations from SQLite and auto-connects enabled ones.
func (m *Manager) LoadFromDB(ctx context.Context) error {
	if m.db == nil {
		return nil
	}
	records, err := m.db.GetMCPServers()
	if err != nil {
		return err
	}

	for _, rec := range records {
		cfg := ServerConfig{
			ID:                rec.ID,
			Name:              rec.Name,
			Transport:         rec.Transport,
			Command:           rec.Command,
			Args:              rec.Args,
			Env:               rec.Env,
			URL:               rec.URL,
			Headers:           rec.Headers,
			AuthType:          rec.AuthType,
			OAuthClientID:     rec.OAuthClientID,
			OAuthClientSecret: rec.OAuthClientSecret,
			OAuthTokenURL:     rec.OAuthTokenURL,
			OAuthScopes:       rec.OAuthScopes,
			OAuthAccessToken:  rec.OAuthAccessToken,
			Enabled:           rec.Enabled,
			Status:            "disconnected",
		}
		m.mu.Lock()
		m.servers[cfg.ID] = &cfg
		m.mu.Unlock()

		if cfg.Enabled {
			go func(id string) {
				_ = m.ConnectServer(ctx, id)
			}(cfg.ID)
		}
	}
	log.Info().Int("count", len(records)).Msg("Loaded persistent MCP servers from database")
	return nil
}

// AddServer adds a server configuration.
func (m *Manager) AddServer(cfg ServerConfig) (*ServerConfig, error) {
	m.mu.Lock()
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.Name == "" {
		cfg.Name = "MCP Server " + cfg.ID[:8]
	}
	cfg.Status = "disconnected"

	m.servers[cfg.ID] = &cfg
	m.mu.Unlock()

	if m.db != nil {
		_ = m.db.SaveMCPServer(db.MCPServerRecord{
			ID:                cfg.ID,
			Name:              cfg.Name,
			Transport:         cfg.Transport,
			Command:           cfg.Command,
			Args:              cfg.Args,
			Env:               cfg.Env,
			URL:               cfg.URL,
			Headers:           cfg.Headers,
			AuthType:          cfg.AuthType,
			OAuthClientID:     cfg.OAuthClientID,
			OAuthClientSecret: cfg.OAuthClientSecret,
			OAuthTokenURL:     cfg.OAuthTokenURL,
			OAuthScopes:       cfg.OAuthScopes,
			OAuthAccessToken:  cfg.OAuthAccessToken,
			Enabled:           cfg.Enabled,
		})
	}

	return &cfg, nil
}

// UpdateServer updates an existing server config and reconnects if active.
func (m *Manager) UpdateServer(cfg ServerConfig) error {
	m.mu.Lock()
	existing, ok := m.servers[cfg.ID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server not found: %s", cfg.ID)
	}

	wasConnected := existing.Status == "connected"
	existing.Name = cfg.Name
	existing.Transport = cfg.Transport
	existing.Command = cfg.Command
	existing.Args = cfg.Args
	existing.Env = cfg.Env
	existing.Cwd = cfg.Cwd
	existing.URL = cfg.URL
	existing.Headers = cfg.Headers
	existing.AuthType = cfg.AuthType
	existing.OAuthClientID = cfg.OAuthClientID
	if cfg.OAuthClientSecret != "" {
		existing.OAuthClientSecret = cfg.OAuthClientSecret
	}
	existing.OAuthTokenURL = cfg.OAuthTokenURL
	existing.OAuthScopes = cfg.OAuthScopes
	if cfg.OAuthAccessToken != "" {
		existing.OAuthAccessToken = cfg.OAuthAccessToken
	}
	existing.TimeoutSeconds = cfg.TimeoutSeconds
	existing.Enabled = cfg.Enabled
	m.mu.Unlock()

	if m.db != nil {
		_ = m.db.SaveMCPServer(db.MCPServerRecord{
			ID:                existing.ID,
			Name:              existing.Name,
			Transport:         existing.Transport,
			Command:           existing.Command,
			Args:              existing.Args,
			Env:               existing.Env,
			URL:               existing.URL,
			Headers:           existing.Headers,
			AuthType:          existing.AuthType,
			OAuthClientID:     existing.OAuthClientID,
			OAuthClientSecret: existing.OAuthClientSecret,
			OAuthTokenURL:     existing.OAuthTokenURL,
			OAuthScopes:       existing.OAuthScopes,
			OAuthAccessToken:  existing.OAuthAccessToken,
			Enabled:           existing.Enabled,
		})
	}

	_ = m.DisconnectServer(cfg.ID)
	if wasConnected || cfg.Enabled {
		_ = m.ConnectServer(context.Background(), cfg.ID)
	}

	return nil
}

// DeleteServer disconnects and deletes an MCP server.
func (m *Manager) DeleteServer(id string) error {
	_ = m.DisconnectServer(id)

	m.mu.Lock()
	delete(m.servers, id)
	m.mu.Unlock()

	if m.db != nil {
		_ = m.db.DeleteMCPServer(id)
	}

	return nil
}

// ListServers returns a copy of all configured MCP servers with secrets masked.
func (m *Manager) ListServers() []ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		safeCopy := *s
		safeCopy.HasOAuthClientSecret = s.OAuthClientSecret != ""
		safeCopy.HasOAuthAccessToken = s.OAuthAccessToken != ""
		safeCopy.OAuthClientSecret = "" // mask
		safeCopy.OAuthAccessToken = ""  // mask
		result = append(result, safeCopy)
	}
	return result
}

// ConnectServer establishes an MCP connection via official Stdio, Streamable HTTP, or SSE transport.
func (m *Manager) ConnectServer(ctx context.Context, id string) error {
	m.mu.Lock()
	srv, ok := m.servers[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server %s not found", id)
	}
	m.mu.Unlock()

	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "go-harness",
		Version: "1.0.0",
	}, nil)

	sessionCtx, sessionCancel := context.WithCancel(context.Background())

	var session *mcp.ClientSession
	var err error
	transportType := strings.ToLower(srv.Transport)

	if transportType == "stdio" {
		if srv.Command == "" {
			sessionCancel()
			return fmt.Errorf("command is required for stdio transport")
		}

		// Expand environment variables in command and args
		expandedCmd := os.ExpandEnv(srv.Command)
		var expandedArgs []string
		for _, a := range srv.Args {
			expandedArgs = append(expandedArgs, os.ExpandEnv(a))
		}

		cmd := exec.Command(expandedCmd, expandedArgs...)
		if srv.Cwd != "" {
			cmd.Dir = os.ExpandEnv(srv.Cwd)
		}

		cmd.Env = os.Environ()
		if len(srv.Env) > 0 {
			for k, v := range srv.Env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, os.ExpandEnv(v)))
			}
		}

		transport := &mcp.CommandTransport{Command: cmd}
		session, err = mcpClient.Connect(sessionCtx, transport, nil)
	} else if transportType == "sse" || transportType == "http" || transportType == "streamable" {
		if srv.URL == "" {
			sessionCancel()
			return fmt.Errorf("url is required for http/sse transport")
		}

		var baseTransport http.RoundTripper = http.DefaultTransport

		// OAuth2 authentication support (e.g. Google OAuth2, client credentials, or access tokens)
		if srv.AuthType == "oauth2" || srv.AuthType == "oauth2_google" || (srv.OAuthClientID != "" && srv.OAuthClientSecret != "") || srv.OAuthAccessToken != "" {
			if srv.OAuthClientID != "" && srv.OAuthClientSecret != "" {
				tokenURL := srv.OAuthTokenURL
				if tokenURL == "" {
					tokenURL = "https://oauth2.googleapis.com/token"
				}
				var scopes []string
				if srv.OAuthScopes != "" {
					for _, s := range strings.FieldsFunc(srv.OAuthScopes, func(r rune) bool {
						return r == ' ' || r == ',' || r == ';'
					}) {
						if t := strings.TrimSpace(s); t != "" {
							scopes = append(scopes, t)
						}
					}
				}
				ccConfig := &clientcredentials.Config{
					ClientID:     srv.OAuthClientID,
					ClientSecret: srv.OAuthClientSecret,
					TokenURL:     tokenURL,
					Scopes:       scopes,
				}
				ts := ccConfig.TokenSource(sessionCtx)
				baseTransport = &oauth2.Transport{
					Source: ts,
					Base:   http.DefaultTransport,
				}
			} else if srv.OAuthAccessToken != "" {
				ts := oauth2.StaticTokenSource(&oauth2.Token{
					AccessToken: srv.OAuthAccessToken,
					TokenType:   "Bearer",
				})
				baseTransport = &oauth2.Transport{
					Source: ts,
					Base:   http.DefaultTransport,
				}
			}
		}

		if len(srv.Headers) > 0 {
			baseTransport = &headerRoundTripper{
				headers: srv.Headers,
				rt:      baseTransport,
			}
		}

		httpClient := &http.Client{
			Transport: baseTransport,
			Timeout:   60 * time.Second,
		}

		// First try StreamableClientTransport (MCP Streamable HTTP protocol)
		streamableTransport := &mcp.StreamableClientTransport{
			Endpoint:             srv.URL,
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
		}

		session, err = mcpClient.Connect(sessionCtx, streamableTransport, nil)
		if err != nil {
			log.Debug().Err(err).Str("server", srv.Name).Msg("Streamable HTTP connect failed, trying SSE transport fallback")
			// Fallback to legacy SSEClientTransport
			sseTransport := &mcp.SSEClientTransport{
				Endpoint:   srv.URL,
				HTTPClient: httpClient,
			}
			session, err = mcpClient.Connect(sessionCtx, sseTransport, nil)
		}
	} else {
		sessionCancel()
		return fmt.Errorf("unsupported transport: %s", srv.Transport)
	}

	if err != nil {
		sessionCancel()
		m.setServerError(id, err)
		return fmt.Errorf("failed to connect to official MCP server '%s': %w", srv.Name, err)
	}

	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Discover Tools
	toolsRes, err := session.ListTools(initCtx, nil)
	var discoveredTools []mcp.Tool
	if err == nil && toolsRes != nil {
		for _, t := range toolsRes.Tools {
			if t != nil {
				discoveredTools = append(discoveredTools, *t)
			}
		}
	}

	// Discover Prompts
	promptsRes, _ := session.ListPrompts(initCtx, nil)
	var discoveredPrompts []*mcp.Prompt
	if promptsRes != nil {
		discoveredPrompts = promptsRes.Prompts
	}

	// Discover Resources
	resourcesRes, _ := session.ListResources(initCtx, nil)
	var discoveredResources []*mcp.Resource
	if resourcesRes != nil {
		discoveredResources = resourcesRes.Resources
	}

	m.mu.Lock()
	srv.Status = "connected"
	srv.LastError = ""
	srv.Enabled = true

	// Clean up old session if existing
	if oldSession, exists := m.sessions[id]; exists {
		if oldSession.cancel != nil {
			oldSession.cancel()
		}
		_ = oldSession.session.Close()
	}

	m.sessions[id] = &activeSession{
		session:     session,
		tools:       discoveredTools,
		prompts:     discoveredPrompts,
		resources:   discoveredResources,
		cancel:      sessionCancel,
		connectedAt: time.Now(),
	}
	m.mu.Unlock()

	// Register discovered tools into unified tools registry
	m.syncToolsToRegistry(id, srv.Name, discoveredTools)

	log.Info().
		Str("server", srv.Name).
		Int("tools", len(discoveredTools)).
		Int("prompts", len(discoveredPrompts)).
		Int("resources", len(discoveredResources)).
		Msg("MCP server connected and synchronized successfully")
	return nil
}

// DisconnectServer terminates an active MCP session.
func (m *Manager) DisconnectServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, ok := m.servers[id]
	if ok {
		srv.Status = "disconnected"
		srv.LastError = ""
	}

	session, exists := m.sessions[id]
	if exists {
		if m.toolsRegistry != nil && srv != nil {
			for _, t := range session.tools {
				toolName := fmt.Sprintf("mcp_%s_%s", sanitizeName(srv.Name), t.Name)
				m.toolsRegistry.Unregister(toolName)
			}
		}
		if session.cancel != nil {
			session.cancel()
		}
		_ = session.session.Close()
		delete(m.sessions, id)
	}

	return nil
}

// PingServer sends a ping to the MCP server to verify session health.
func (m *Manager) PingServer(ctx context.Context, id string) error {
	m.mu.RLock()
	session, exists := m.sessions[id]
	srv, srvExists := m.servers[id]
	m.mu.RUnlock()

	if !srvExists {
		return fmt.Errorf("server %s not found", id)
	}
	if !exists || session == nil || session.session == nil {
		return fmt.Errorf("server %s is not connected", id)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := session.session.Ping(pingCtx, nil)
	if err != nil {
		m.setServerError(id, err)
		return fmt.Errorf("ping failed: %w", err)
	}

	m.mu.Lock()
	if srv.Status != "connected" {
		srv.Status = "connected"
		srv.LastError = ""
	}
	m.mu.Unlock()

	return nil
}

// HealthCheckAll pings all connected servers and updates statuses.
func (m *Manager) HealthCheckAll(ctx context.Context) map[string]string {
	m.mu.RLock()
	serverIDs := make([]string, 0, len(m.servers))
	for id := range m.servers {
		serverIDs = append(serverIDs, id)
	}
	m.mu.RUnlock()

	statuses := make(map[string]string)
	for _, id := range serverIDs {
		m.mu.RLock()
		srv := m.servers[id]
		m.mu.RUnlock()

		if srv == nil || !srv.Enabled {
			statuses[id] = "disabled"
			continue
		}

		if err := m.PingServer(ctx, id); err != nil {
			statuses[id] = "unreachable: " + err.Error()
		} else {
			statuses[id] = "healthy"
		}
	}
	return statuses
}

// GetTools returns the list of tools discovered for a server.
func (m *Manager) GetTools(serverID string) ([]mcp.Tool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[serverID]
	if !exists {
		return nil, fmt.Errorf("server %s is not connected", serverID)
	}
	return session.tools, nil
}

// GetPrompts returns the list of prompts available for a server.
func (m *Manager) GetPrompts(serverID string) ([]*mcp.Prompt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[serverID]
	if !exists {
		return nil, fmt.Errorf("server %s is not connected", serverID)
	}
	return session.prompts, nil
}

// GetResources returns discovered resources for a server.
func (m *Manager) GetResources(ctx context.Context, serverID string) ([]*mcp.Resource, error) {
	m.mu.RLock()
	session, exists := m.sessions[serverID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s is not connected", serverID)
	}

	res, err := session.session.ListResources(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res.Resources, nil
}

// ReadResource reads content of a specific resource.
func (m *Manager) ReadResource(ctx context.Context, serverID, uri string) (*mcp.ReadResourceResult, error) {
	m.mu.RLock()
	session, exists := m.sessions[serverID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s is not connected", serverID)
	}

	return session.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
}

func (m *Manager) registerResourceTool() {
	m.toolsRegistry.Register(tools.ToolDefinition{
		Name:        "read_mcp_resource",
		Description: "Reads and retrieves the content of a resource URI exposed by a connected MCP server.",
		Category:    "mcp",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"server_name": {
					Type:        jsonschema.String,
					Description: "Name or ID of the MCP server providing the resource",
				},
				"uri": {
					Type:        jsonschema.String,
					Description: "The exact resource URI to read (e.g. 'file:///logs/app.log')",
				},
			},
			Required: []string{"server_name", "uri"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				ServerName string `json:"server_name"`
				URI        string `json:"uri"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}

			serverID := in.ServerName
			m.mu.RLock()
			for _, s := range m.servers {
				if strings.EqualFold(s.Name, in.ServerName) || s.ID == in.ServerName {
					serverID = s.ID
					break
				}
			}
			m.mu.RUnlock()

			res, err := m.ReadResource(ctx, serverID, in.URI)
			if err != nil {
				return nil, fmt.Errorf("failed to read MCP resource: %w", err)
			}

			var contents []string
			for _, c := range res.Contents {
				if c != nil {
					if c.Text != "" {
						contents = append(contents, c.Text)
					} else if len(c.Blob) > 0 {
						contents = append(contents, fmt.Sprintf("[Blob data: %s, mime: %s]", string(c.Blob), c.MIMEType))
					}
				}
			}

			return map[string]any{
				"uri":      in.URI,
				"contents": strings.Join(contents, "\n"),
			}, nil
		},
	})
}

func (m *Manager) syncToolsToRegistry(serverID, serverName string, discoveredTools []mcp.Tool) {
	if m.toolsRegistry == nil {
		return
	}

	for _, tool := range discoveredTools {
		toolName := fmt.Sprintf("mcp_%s_%s", sanitizeName(serverName), tool.Name)
		capturedToolName := tool.Name
		capturedServerID := serverID

		var schema jsonschema.Definition
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			_ = json.Unmarshal(schemaBytes, &schema)
		} else {
			schema = jsonschema.Definition{Type: jsonschema.Object}
		}

		if schema.Type == "" {
			schema.Type = jsonschema.Object
		}
		if schema.Properties == nil {
			schema.Properties = make(map[string]jsonschema.Definition)
		}

		m.toolsRegistry.Register(tools.ToolDefinition{
			Name:        toolName,
			Description: fmt.Sprintf("[%s] %s", serverName, tool.Description),
			Category:    "mcp",
			Enabled:     true,
			Parameters:  schema,
			Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
				return m.executeMCPTool(ctx, capturedServerID, toolName, capturedToolName, args)
			},
		})
	}
}

func (m *Manager) executeMCPTool(ctx context.Context, serverID, fullToolName, originalToolName string, args json.RawMessage) (any, error) {
	var params map[string]any
	trimmed := strings.TrimSpace(string(args))
	if len(trimmed) > 0 && trimmed != "{}" && trimmed != "null" {
		_ = json.Unmarshal(args, &params)
	}
	if params == nil {
		params = make(map[string]any)
	}

	m.mu.RLock()
	sessionInfo, ok := m.sessions[serverID]
	srv := m.servers[serverID]
	m.mu.RUnlock()

	if !ok || sessionInfo == nil || sessionInfo.session == nil {
		// Attempt transparent on-demand reconnection
		if err := m.ConnectServer(ctx, serverID); err != nil {
			return nil, fmt.Errorf("MCP server '%s' is not connected: %w", serverID, err)
		}
		m.mu.RLock()
		sessionInfo = m.sessions[serverID]
		m.mu.RUnlock()
	}

	callTimeout := 60 * time.Second
	if srv != nil && srv.TimeoutSeconds > 0 {
		callTimeout = time.Duration(srv.TimeoutSeconds) * time.Second
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	callCtx, span := tracing.StartSpan(callCtx, "mcp.call_tool."+fullToolName)
	defer span.End()

	res, err := sessionInfo.session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      originalToolName,
		Arguments: params,
	})

	// Retry once on broken pipe / EOF connection errors
	if err != nil && isConnectionError(err) {
		log.Warn().Err(err).Str("server", serverID).Msg("MCP tool call encountered connection error, attempting auto-reconnect")
		if reconnErr := m.ConnectServer(ctx, serverID); reconnErr == nil {
			m.mu.RLock()
			sessionInfo = m.sessions[serverID]
			m.mu.RUnlock()
			if sessionInfo != nil && sessionInfo.session != nil {
				res, err = sessionInfo.session.CallTool(callCtx, &mcp.CallToolParams{
					Name:      originalToolName,
					Arguments: params,
				})
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Format response cleanly
	var textParts []string
	var images []MCPImageContent

	for _, c := range res.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			textParts = append(textParts, v.Text)
		case *mcp.ImageContent:
			images = append(images, MCPImageContent{
				MIMEType: v.MIMEType,
				Data:     string(v.Data),
			})
		case *mcp.EmbeddedResource:
			if v.Resource != nil {
				if v.Resource.Text != "" {
					textParts = append(textParts, v.Resource.Text)
				} else if len(v.Resource.Blob) > 0 {
					textParts = append(textParts, fmt.Sprintf("[Resource blob: %s (mime: %s)]", v.Resource.URI, v.Resource.MIMEType))
				} else {
					textParts = append(textParts, fmt.Sprintf("[Resource: %s]", v.Resource.URI))
				}
			}
		default:
			b, _ := json.Marshal(v)
			textParts = append(textParts, string(b))
		}
	}

	combinedText := strings.Join(textParts, "\n")

	if res.IsError {
		if combinedText == "" {
			combinedText = "mcp tool execution error"
		}
		return nil, fmt.Errorf("mcp tool error: %s", combinedText)
	}

	// If only plain text and no images, return clean structured object
	return MCPToolResult{
		Content: combinedText,
		Text:    combinedText,
		Images:  images,
		IsError: false,
	}, nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "eof") ||
		strings.Contains(s, "closed network connection") ||
		errors.Is(err, context.Canceled)
}

func (m *Manager) setServerError(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if srv, ok := m.servers[id]; ok {
		srv.Status = "error"
		srv.LastError = err.Error()
	}
}

func sanitizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out []rune
	lastWasUnderscore := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
			lastWasUnderscore = false
		} else {
			if !lastWasUnderscore {
				out = append(out, '_')
				lastWasUnderscore = true
			}
		}
	}
	res := strings.Trim(string(out), "_")
	if res == "" {
		return "server"
	}
	return res
}
