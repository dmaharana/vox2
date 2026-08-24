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
	skillsDir := filepath.Join(tempDir, "skills")
	_ = os.MkdirAll(skillsDir, 0755)
	skillsLoader := skills.NewLoader(skillsDir)
	_ = skillsLoader.Load()

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

	// 5. Skills List & On-Demand Refresh
	req = httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("skills list failed: %d", w.Code)
	}

	// Dynamically write a new skill to disk
	_ = os.WriteFile(filepath.Join(skillsDir, "dynamic-skill.md"), []byte("# Dynamic Skill\nDoes something cool"), 0644)

	// Call POST /api/skills/refresh
	req = httptest.NewRequest(http.MethodPost, "/api/skills/refresh", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("skills refresh failed: %d, body: %s", w.Code, w.Body.String())
	}

	var refreshedSkills []skills.Skill
	_ = json.Unmarshal(w.Body.Bytes(), &refreshedSkills)
	if len(refreshedSkills) != 1 || refreshedSkills[0].Name != "dynamic-skill" {
		t.Errorf("expected 1 refreshed skill named dynamic-skill, got %+v", refreshedSkills)
	}
}
