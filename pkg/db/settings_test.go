package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-harness/pkg/crypto"
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

	// 2. Test MCP Servers table CRUD with OAuth2 fields and encryption
	serverRec := MCPServerRecord{
		ID:                "srv-1",
		Name:              "Google Cloud MCP Server",
		Transport:         "sse",
		URL:               "https://mcp.googlecloud.example.com/sse",
		Headers:           map[string]string{"Authorization": "Bearer secret-token-123"},
		AuthType:          "oauth2",
		OAuthClientID:     "google-client-id-123",
		OAuthClientSecret: "google-client-secret-xyz",
		OAuthTokenURL:     "https://oauth2.googleapis.com/token",
		OAuthScopes:       "https://www.googleapis.com/auth/cloud-platform",
		OAuthAccessToken:  "google-access-token-abc",
		Enabled:           true,
	}

	err = database.SaveMCPServer(serverRec)
	if err != nil {
		t.Fatalf("SaveMCPServer failed: %v", err)
	}

	// Verify directly via raw SQL that secrets in mcp_servers table are encrypted!
	var rawSecret, rawToken, rawHeaders string
	err = database.SQL().QueryRow(`SELECT oauth_client_secret, oauth_access_token, headers FROM mcp_servers WHERE id = ?`, "srv-1").Scan(&rawSecret, &rawToken, &rawHeaders)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if !strings.HasPrefix(rawSecret, crypto.Prefix) {
		t.Fatalf("Expected raw oauth_client_secret to be encrypted with prefix '%s', got '%s'", crypto.Prefix, rawSecret)
	}
	if !strings.HasPrefix(rawToken, crypto.Prefix) {
		t.Fatalf("Expected raw oauth_access_token to be encrypted with prefix '%s', got '%s'", crypto.Prefix, rawToken)
	}
	if strings.Contains(rawHeaders, "secret-token-123") {
		t.Fatalf("Expected Authorization header in rawHeaders to be encrypted!")
	}

	// Verify GetMCPServers decrypts correctly
	list, err := database.GetMCPServers()
	if err != nil || len(list) != 1 {
		t.Fatalf("GetMCPServers expected 1 record, got %d (err: %v)", len(list), err)
	}
	got := list[0]
	if got.Name != "Google Cloud MCP Server" || got.AuthType != "oauth2" {
		t.Errorf("GetMCPServers record mismatch: %+v", got)
	}
	if got.OAuthClientID != "google-client-id-123" {
		t.Errorf("OAuthClientID mismatch: %s", got.OAuthClientID)
	}
	if got.OAuthClientSecret != "google-client-secret-xyz" {
		t.Errorf("OAuthClientSecret mismatch: expected 'google-client-secret-xyz', got '%s'", got.OAuthClientSecret)
	}
	if got.OAuthTokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("OAuthTokenURL mismatch: %s", got.OAuthTokenURL)
	}
	if got.OAuthScopes != "https://www.googleapis.com/auth/cloud-platform" {
		t.Errorf("OAuthScopes mismatch: %s", got.OAuthScopes)
	}
	if got.OAuthAccessToken != "google-access-token-abc" {
		t.Errorf("OAuthAccessToken mismatch: expected 'google-access-token-abc', got '%s'", got.OAuthAccessToken)
	}
	if got.Headers["Authorization"] != "Bearer secret-token-123" {
		t.Errorf("Decrypted Authorization header mismatch: %s", got.Headers["Authorization"])
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
