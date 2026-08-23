package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

	// 2. Add SSE server
	cfg2, err := mgr.AddServer(ServerConfig{
		Name:      "Test SSE Server",
		Transport: "sse",
		URL:       "http://localhost:9999/sse",
	})
	if err != nil {
		t.Fatalf("failed to add sse server: %v", err)
	}

	servers := mgr.ListServers()
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
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
