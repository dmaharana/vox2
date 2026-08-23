package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-harness/pkg/db"
	"go-harness/pkg/tools"
)

func TestMemoryFTSAndPromotion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mem-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "memory.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	mgr := NewManager(database)

	// 1. Save semantic memory
	item1 := &MemoryItem{
		Key:        "user_name",
		Content:    "The user's preferred name is Alice, a software architect.",
		MemoryType: TypeSemantic,
		Tags:       "profile, user, preferences",
	}
	if err := mgr.Save(item1); err != nil {
		t.Fatalf("failed to save memory 1: %v", err)
	}

	// 2. Save procedural memory
	item2 := &MemoryItem{
		Key:        "deploy_workflow",
		Content:    "To deploy, run make build and docker push to registry.",
		MemoryType: TypeProcedural,
		Tags:       "deploy, devops, docker",
	}
	if err := mgr.Save(item2); err != nil {
		t.Fatalf("failed to save memory 2: %v", err)
	}

	// 3. Save working memory (short-term)
	item3 := &MemoryItem{
		Key:        "current_task",
		Content:    "Currently researching OpenTelemetry exporters for Golang.",
		MemoryType: TypeWorking,
		Tags:       "task, active",
	}
	if err := mgr.Save(item3); err != nil {
		t.Fatalf("failed to save memory 3: %v", err)
	}

	// 4. Search FTS5
	results, err := mgr.Search("Alice architect", "", "", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 || results[0].Key != "user_name" {
		t.Fatalf("expected to find Alice profile, got %+v", results)
	}

	// 5. Search with tags
	resultsDevOps, err := mgr.Search("docker deploy", "", "", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(resultsDevOps) == 0 || resultsDevOps[0].Key != "deploy_workflow" {
		t.Fatalf("expected to find deploy workflow, got %+v", resultsDevOps)
	}

	// Search short term memory so its access count increments
	resultsWorking, err := mgr.Search("OpenTelemetry", "", "", 10)
	if err != nil || len(resultsWorking) == 0 {
		t.Fatalf("expected to find working memory, got %v", err)
	}

	// 6. Test Auto-promotion
	promoted, err := mgr.AutoPromoteAging(1)
	if err != nil {
		t.Fatalf("autopromote error: %v", err)
	}
	if promoted < 1 {
		t.Errorf("expected at least 1 memory promoted, got %d", promoted)
	}

	// 7. Test Memory Tools Execution
	reg := tools.NewRegistry()
	mgr.RegisterMemoryTools(reg)

	ctx := context.Background()
	saveArgs, _ := json.Marshal(map[string]any{
		"key":         "favorite_color",
		"content":     "Alice likes blue and teal themes.",
		"memory_type": "semantic",
		"tags":        "ui, colors",
	})
	res, err := reg.Execute(ctx, "save_memory", saveArgs)
	if err != nil {
		t.Fatalf("save_memory tool failed: %v", err)
	}
	if resMap, ok := res.(map[string]any); !ok || resMap["success"] != true {
		t.Errorf("expected tool success, got %+v", res)
	}

	// Allow async FTS5 index update
	time.Sleep(10 * time.Millisecond)

	searchArgs, _ := json.Marshal(map[string]any{
		"query": "teal theme",
	})
	resSearch, err := reg.Execute(ctx, "search_memory", searchArgs)
	if err != nil {
		t.Fatalf("search_memory tool failed: %v", err)
	}
	if resSearchMap, ok := resSearch.(map[string]any); !ok || resSearchMap["count"].(int) == 0 {
		t.Errorf("expected search tool to return color memory, got %+v", resSearch)
	}
}
