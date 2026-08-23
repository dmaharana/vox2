package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsAndMCPServersPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "db-persist-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "persist.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// 1. Test Settings table CRUD
	err = database.SetSetting("llm_model", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	val, err := database.GetSetting("llm_model")
	if err != nil || val != "gpt-4o-mini" {
		t.Fatalf("GetSetting expected 'gpt-4o-mini', got '%s' (err: %v)", val, err)
	}

	allSettings, err := database.GetAllSettings()
	if err != nil || allSettings["llm_model"] != "gpt-4o-mini" {
		t.Fatalf("GetAllSettings mismatch: %+v", allSettings)
	}

	// 2. Test MCP Servers table CRUD
	serverRec := MCPServerRecord{
		ID:        "srv-1",
		Name:      "Production Filesystem MCP",
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:       map[string]string{"DEBUG": "1"},
		URL:       "",
		Headers:   map[string]string{"Authorization": "Bearer 123"},
		Enabled:   true,
	}

	err = database.SaveMCPServer(serverRec)
	if err != nil {
		t.Fatalf("SaveMCPServer failed: %v", err)
	}

	list, err := database.GetMCPServers()
	if err != nil || len(list) != 1 {
		t.Fatalf("GetMCPServers expected 1 record, got %d (err: %v)", len(list), err)
	}
	if list[0].Name != "Production Filesystem MCP" || list[0].Headers["Authorization"] != "Bearer 123" {
		t.Errorf("GetMCPServers record mismatch: %+v", list[0])
	}

	// 3. Test Delete
	err = database.DeleteMCPServer("srv-1")
	if err != nil {
		t.Fatalf("DeleteMCPServer failed: %v", err)
	}

	listAfter, err := database.GetMCPServers()
	if err != nil || len(listAfter) != 0 {
		t.Fatalf("expected 0 records after delete, got %d", len(listAfter))
	}
}
