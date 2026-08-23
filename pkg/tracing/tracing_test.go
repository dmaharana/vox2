package tracing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-harness/pkg/config"
)

func TestTracingInitAndHelpers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "traces-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	traceFile := filepath.Join(tempDir, "test_trace.json")

	cfg := &config.Config{
		OTelExporter: "console,file",
		TraceFile:    traceFile,
	}

	ctx := context.Background()
	tp, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to initialize tracing: %v", err)
	}
	defer func() {
		_ = Shutdown(context.Background())
	}()

	if tp == nil {
		t.Fatal("expected tracer provider instance")
	}

	// Test helper spans
	ctx, span := StartSpan(ctx, "test.parent")
	TraceLLMCall(ctx, "gpt-4o", "Hello world prompt", 120*time.Millisecond, 15, 30, nil)
	TraceToolCall(ctx, "read_file", map[string]string{"path": "test.txt"}, map[string]string{"content": "file data"}, nil)
	TraceSkillUse(ctx, "weather-skill", "fetch_forecast", nil)
	TraceFlow(ctx, "flow-123", 4, nil)
	span.End()

	// Shutdown to flush
	if err := Shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}

	// Verify trace file exists and has content
	data, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatalf("failed to read trace file: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("expected trace file to contain exported spans")
	}
}
