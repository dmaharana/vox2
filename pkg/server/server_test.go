package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-harness/pkg/config"
	"go-harness/pkg/db"
	"go-harness/pkg/memory"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/ws"
)

func TestServerFullRESTSuite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "srv-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, _ := db.Open(dbPath)
	defer database.Close()

	memMgr := memory.NewManager(database)
	skillsLoader := skills.NewLoader(filepath.Join(tempDir, "skills"))
	toolsReg := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolsReg, tempDir)

	cfg, _ := config.Load()
	hub := ws.NewHub(nil)
	go hub.Run()
	defer hub.Close()

	srv := New(cfg, hub, ServerOptions{
		DB:           database,
		MemoryMgr:    memMgr,
		SkillsLoader: skillsLoader,
		ToolsReg:     toolsReg,
	})

	// 1. Health
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health failed: %d", w.Code)
	}

	// 2. Tools list
	req = httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("tools list failed: %d", w.Code)
	}

	// 3. Create conversation & export
	conv, _ := database.CreateConversation("Chat 1")
	_ = database.AddMessage(db.Message{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        "Hi agent",
	})

	req = httptest.NewRequest(http.MethodGet, "/api/conversations", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("list conversations failed: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/conversations/"+conv.ID+"/export", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("export conversation failed: %d", w.Code)
	}

	// 4. Memory REST
	memBody, _ := json.Marshal(map[string]any{
		"key":         "project_stack",
		"content":     "Go backend with React Vite SPA",
		"memory_type": "semantic",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/memories", bytes.NewReader(memBody))
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("save memory failed: %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/memories?q=React", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("search memory failed: %d", w.Code)
	}
}
