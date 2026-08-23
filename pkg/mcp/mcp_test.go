package mcp

import (
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

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"My Server 123", "my_server_123"},
		{"filesystem-tool", "filesystem_tool"},
		{"---special@@chars---", "special_chars"},
		{"", "server"},
	}

	for _, tt := range tests {
		got := sanitizeName(tt.in)
		if got != tt.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
