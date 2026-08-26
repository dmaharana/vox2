package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sashabaranov/go-openai/jsonschema"
	"go-harness/pkg/tools"
)

func TestMCPManagerConfig(t *testing.T) {
	reg := tools.NewRegistry()
	mgr := NewManager(reg)

	// 1. Add stdio server
	cfg1, err := mgr.AddServer(ServerConfig{
		Name:      "Test Stdio Server",
		Transport: "stdio",
		Command:   "echo",
		Args:      []string{"hello"},
	})
	if err != nil {
		t.Fatalf("failed to add server: %v", err)
	}

	// 2. Add SSE server with OAuth2
	cfg2, err := mgr.AddServer(ServerConfig{
		Name:              "Google Workspace MCP",
		Transport:         "sse",
		URL:               "https://mcp.googlecloud.example.com/sse",
		AuthType:          "oauth2",
		OAuthClientID:     "g-client-id",
		OAuthClientSecret: "g-client-secret-12345",
		OAuthTokenURL:     "https://oauth2.googleapis.com/token",
		OAuthScopes:       "https://www.googleapis.com/auth/cloud-platform",
	})
	if err != nil {
		t.Fatalf("failed to add sse server: %v", err)
	}

	servers := mgr.ListServers()
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	// Verify secret masking in ListServers
	for _, s := range servers {
		if s.ID == cfg2.ID {
			if s.OAuthClientSecret != "" {
				t.Errorf("expected masked OAuthClientSecret, got: %s", s.OAuthClientSecret)
			}
			if !s.HasOAuthClientSecret {
				t.Errorf("expected HasOAuthClientSecret=true")
			}
			if s.AuthType != "oauth2" {
				t.Errorf("expected AuthType=oauth2, got: %s", s.AuthType)
			}
		}
	}

	// 3. Update server
	cfg1.Name = "Renamed Server"
	if err := mgr.UpdateServer(*cfg1); err != nil {
		t.Fatalf("failed to update server: %v", err)
	}

	// 4. Delete server
	if err := mgr.DeleteServer(cfg2.ID); err != nil {
		t.Fatalf("failed to delete server: %v", err)
	}

	serversAfter := mgr.ListServers()
	if len(serversAfter) != 1 || serversAfter[0].Name != "Renamed Server" {
		t.Errorf("delete/update mismatch: %+v", serversAfter)
	}
}

func TestMCPOAuth2TokenAcquisition(t *testing.T) {
	var tokenCalls int64

	// Mock OAuth2 token server (e.g. Google OAuth2 endpoint)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&tokenCalls, 1)
		clientID, clientSecret, ok := r.BasicAuth()
		if !ok {
			clientID = r.FormValue("client_id")
			clientSecret = r.FormValue("client_secret")
		}

		if clientID != "google-client-id" || clientSecret != "google-client-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "google-mcp-access-token-777",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	reg := tools.NewRegistry()
	mgr := NewManager(reg)

	srv, err := mgr.AddServer(ServerConfig{
		Name:              "Google Cloud MCP",
		Transport:         "sse",
		URL:               "http://invalid.mcp.host:1234/sse", // Won't connect fully to MCP handshake, but tests config & setup
		AuthType:          "oauth2",
		OAuthClientID:     "google-client-id",
		OAuthClientSecret: "google-client-secret",
		OAuthTokenURL:     tokenServer.URL,
		OAuthScopes:       "https://www.googleapis.com/auth/cloud-platform",
	})
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	if srv.AuthType != "oauth2" {
		t.Errorf("expected AuthType oauth2, got %s", srv.AuthType)
	}
}

func TestHeaderRoundTripper(t *testing.T) {
	customHeaders := map[string]string{
		"Authorization": "Bearer test-secret-token",
		"X-API-Key":     "key-12345",
		"Custom-Header": "custom-value",
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range customHeaders {
			got := r.Header.Get(k)
			if got != v {
				t.Errorf("Header %s mismatch: got %q, want %q", k, got, v)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := &http.Client{
		Transport: &headerRoundTripper{
			headers: customHeaders,
			rt:      http.DefaultTransport,
		},
	}

	req, _ := http.NewRequest("GET", mockServer.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestConnectManagerLocal(t *testing.T) {
	// Check if local 8090 server is running
	checkResp, err := http.Get("http://localhost:8090/mcp")
	if err != nil {
		t.Skip("local MCP server at :8090 not running, skipping test")
	}
	checkResp.Body.Close()

	reg := tools.NewRegistry()
	mgr := NewManager(reg)

	srv, err := mgr.AddServer(ServerConfig{
		Name:      "Local Users DB MCP",
		Transport: "http",
		URL:       "http://localhost:8090/mcp",
	})
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	err = mgr.ConnectServer(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("ConnectServer failed: %v", err)
	}

	toolsList := reg.List()
	if len(toolsList) == 0 {
		t.Fatalf("Expected discovered tools from local MCP server, got 0")
	}
	t.Logf("Successfully registered %d MCP tools in tool registry", len(toolsList))
}

func TestNoParamMCPTool(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_status",
		Description: "Get server status with no parameters",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, string, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "operational"}},
		}, "operational", nil
	})

	sseHandler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil)
	testServer := httptest.NewServer(sseHandler)
	defer testServer.Close()

	reg := tools.NewRegistry()
	mgr := NewManager(reg)

	srv, err := mgr.AddServer(ServerConfig{
		Name:      "TestServer",
		Transport: "sse",
		URL:       testServer.URL,
	})
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	err = mgr.ConnectServer(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("ConnectServer failed: %v", err)
	}
	defer mgr.DisconnectServer(srv.ID)

	toolName := "mcp_testserver_get_status"
	toolDef, ok := reg.Get(toolName)
	if !ok {
		t.Fatalf("tool %s not registered", toolName)
	}
	t.Logf("Registered tool schema: %+v", toolDef.Parameters)

	// Verify ToOpenAITools converts it to valid OpenAI Tool schema
	openAITools := reg.ToOpenAITools()
	if len(openAITools) != 1 {
		t.Fatalf("expected 1 OpenAI tool, got %d", len(openAITools))
	}
	if openAITools[0].Function.Name != toolName {
		t.Errorf("expected OpenAI tool name %s, got %s", toolName, openAITools[0].Function.Name)
	}
	schema, ok := openAITools[0].Function.Parameters.(jsonschema.Definition)
	if !ok {
		t.Fatalf("expected Parameters to be jsonschema.Definition, got %T", openAITools[0].Function.Parameters)
	}
	if schema.Type != "object" {
		t.Errorf("expected OpenAI tool parameters type 'object', got %v", schema.Type)
	}

	// Test 1: Empty JSON object "{}"
	res, err := reg.Execute(context.Background(), toolName, json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("Execute with '{}' failed: %v", err)
	}
	t.Logf("Execute with '{}' result: %+v", res)

	// Test 2: Empty string ""
	res, err = reg.Execute(context.Background(), toolName, json.RawMessage(""))
	if err != nil {
		t.Fatalf("Execute with '' failed: %v", err)
	}
	t.Logf("Execute with '' result: %+v", res)

	// Test 3: nil args
	res, err = reg.Execute(context.Background(), toolName, nil)
	if err != nil {
		t.Fatalf("Execute with nil failed: %v", err)
	}
	t.Logf("Execute with nil result: %+v", res)

	// Test 4: null JSON
	res, err = reg.Execute(context.Background(), toolName, json.RawMessage("null"))
	if err != nil {
		t.Fatalf("Execute with 'null' failed: %v", err)
	}
	t.Logf("Execute with 'null' result: %+v", res)

	// Test 5: whitespace "   "
	res, err = reg.Execute(context.Background(), toolName, json.RawMessage("   "))
	if err != nil {
		t.Fatalf("Execute with '   ' failed: %v", err)
	}
	t.Logf("Execute with '   ' result: %+v", res)
}
