package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-harness/pkg/tools"
	"go-harness/pkg/ws"
)

func TestParallelFlowExecution(t *testing.T) {
	hub := ws.NewHub(nil)
	go hub.Run()
	defer hub.Close()

	engine := NewEngine(hub, 3)

	mockRunner := func(ctx context.Context, task Task) (string, error) {
		time.Sleep(10 * time.Millisecond)
		if task.ID == "fail_task" {
			return "", fmt.Errorf("simulated task error")
		}
		return fmt.Sprintf("Processed %s with instruction: %s", task.Title, task.Instruction), nil
	}

	input := ParallelFlowInput{
		SynthesisPrompt: "Compare results from multiple regions",
		Tasks: []Task{
			{ID: "task_1", Title: "Region US", Instruction: "Check server load in US East"},
			{ID: "task_2", Title: "Region EU", Instruction: "Check server load in EU West"},
			{ID: "fail_task", Title: "Region AP", Instruction: "Check server load in AP South"},
		},
	}

	out, err := engine.ExecuteParallel(context.Background(), input, mockRunner)
	if err != nil {
		t.Fatalf("unexpected flow error: %v", err)
	}

	if out.TotalTasks != 3 {
		t.Errorf("expected 3 tasks, got %d", out.TotalTasks)
	}
	if out.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", out.Completed)
	}
	if out.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", out.Failed)
	}

	if !strings.Contains(out.Summary, "Compare results from multiple regions") || !strings.Contains(out.Summary, "Region US") {
		t.Errorf("summary missing expected contents: %s", out.Summary)
	}
}

func TestParallelFlowToolRegistry(t *testing.T) {
	reg := tools.NewRegistry()
	engine := NewEngine(nil, 2)

	mockRunner := func(ctx context.Context, task Task) (string, error) {
		return "Done: " + task.Title, nil
	}

	RegisterFlowTools(reg, engine, mockRunner)

	tool, ok := reg.Get("spawn_parallel_flow")
	if !ok || !tool.Enabled {
		t.Fatalf("spawn_parallel_flow tool not registered properly")
	}

	args, _ := json.Marshal(ParallelFlowInput{
		SynthesisPrompt: "Summary of tasks",
		Tasks: []Task{
			{Title: "Task A", Instruction: "Do A"},
			{Title: "Task B", Instruction: "Do B"},
		},
	})

	res, err := reg.Execute(context.Background(), "spawn_parallel_flow", args)
	if err != nil {
		t.Fatalf("tool execution error: %v", err)
	}

	resMap, ok := res.(map[string]any)
	if !ok || resMap["completed"].(int) != 2 {
		t.Errorf("unexpected tool result: %+v", res)
	}
}
