package flow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/tracing"
	"go-harness/pkg/ws"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Task defines an independent unit of work to be executed in parallel.
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Instruction string   `json:"instruction"`
	Tools       []string `json:"tools,omitempty"`
	Context     string   `json:"context,omitempty"`
}

// TaskResult holds the execution outcome of a single task.
type TaskResult struct {
	TaskID     string `json:"task_id"`
	Title      string `json:"title"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Status     string `json:"status"` // "completed" or "failed"
}

// ParallelFlowInput is the schema passed to the parallel flow runner.
type ParallelFlowInput struct {
	Tasks           []Task `json:"tasks"`
	SynthesisPrompt string `json:"synthesis_prompt,omitempty"`
}

// ParallelFlowOutput contains the individual task outcomes and aggregated summary.
type ParallelFlowOutput struct {
	FlowID      string       `json:"flow_id"`
	TotalTasks  int          `json:"total_tasks"`
	Completed   int          `json:"completed"`
	Failed      int          `json:"failed"`
	TaskResults []TaskResult `json:"task_results"`
	Summary     string       `json:"summary"`
	DurationMs  int64        `json:"duration_ms"`
}

// TaskRunner executes an individual task.
type TaskRunner func(ctx context.Context, task Task) (string, error)

// Engine orchestrates parallel subflow execution and telemetry.
type Engine struct {
	hub            *ws.Hub
	maxConcurrency int
}

// NewEngine creates a new subflow Engine.
func NewEngine(hub *ws.Hub, maxConcurrency int) *Engine {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	return &Engine{
		hub:            hub,
		maxConcurrency: maxConcurrency,
	}
}

// ExecuteParallel runs tasks in parallel with bounded concurrency and summarizes results.
func (e *Engine) ExecuteParallel(ctx context.Context, input ParallelFlowInput, runner TaskRunner) (*ParallelFlowOutput, error) {
	if len(input.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks provided for parallel flow")
	}

	flowID := uuid.New().String()
	startTime := time.Now()

	// Tracing parent span
	ctx, span := tracing.StartSpan(ctx, "flow.parallel_execution",
		trace.WithAttributes(
			attribute.String("flow.id", flowID),
			attribute.Int("flow.task_count", len(input.Tasks)),
		),
	)
	defer span.End()

	log.Info().Str("flow_id", flowID).Int("tasks", len(input.Tasks)).Msg("Starting parallel subflow execution")

	results := make([]TaskResult, len(input.Tasks))
	sem := make(chan struct{}, e.maxConcurrency)
	var wg sync.WaitGroup

	for i, task := range input.Tasks {
		if task.ID == "" {
			task.ID = fmt.Sprintf("task_%d", i+1)
		}
		if task.Title == "" {
			task.Title = fmt.Sprintf("Task %d", i+1)
		}

		wg.Add(1)
		go func(idx int, t Task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tStartTime := time.Now()

			// Broadcast task start event
			e.emitEvent(flowID, idx+1, len(input.Tasks), t.Title, "running", "", 0)

			// Subtask span
			tCtx, tSpan := tracing.StartSpan(ctx, "flow.subtask."+t.ID,
				trace.WithAttributes(
					attribute.String("task.id", t.ID),
					attribute.String("task.title", t.Title),
				),
			)
			defer tSpan.End()

			var out string
			var runErr error
			if runner != nil {
				out, runErr = runner(tCtx, t)
			} else {
				out = fmt.Sprintf("Simulated completion for task: %s", t.Title)
			}

			dur := time.Since(tStartTime).Milliseconds()

			res := TaskResult{
				TaskID:     t.ID,
				Title:      t.Title,
				Output:     out,
				DurationMs: dur,
			}

			if runErr != nil {
				res.Error = runErr.Error()
				res.Status = "failed"
				tSpan.RecordError(runErr)
				e.emitEvent(flowID, idx+1, len(input.Tasks), t.Title, "failed", runErr.Error(), dur)
			} else {
				res.Status = "completed"
				e.emitEvent(flowID, idx+1, len(input.Tasks), t.Title, "completed", out, dur)
			}

			results[idx] = res
		}(i, task)
	}

	wg.Wait()

	// Calculate counts
	completed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "completed" {
			completed++
		} else {
			failed++
		}
	}

	totalDur := time.Since(startTime).Milliseconds()

	// Synthesize summary
	summary := synthesizeResults(input.SynthesisPrompt, results)

	output := &ParallelFlowOutput{
		FlowID:      flowID,
		TotalTasks:  len(input.Tasks),
		Completed:   completed,
		Failed:      failed,
		TaskResults: results,
		Summary:     summary,
		DurationMs:  totalDur,
	}

	log.Info().Str("flow_id", flowID).Int("completed", completed).Int("failed", failed).Int64("dur_ms", totalDur).Msg("Parallel subflow finished")
	return output, nil
}

func (e *Engine) emitEvent(flowID string, index, total int, taskName, status, summary string, durMs int64) {
	if e.hub == nil {
		return
	}

	e.hub.Broadcast(ws.OutboundMessage{
		Type:      ws.TypeSubflowEvent,
		Timestamp: time.Now().UTC(),
		Payload: ws.SubflowPayload{
			FlowID:     flowID,
			TaskIndex:  index,
			TotalTasks: total,
			TaskName:   taskName,
			Status:     status,
			Summary:    summary,
			DurationMs: durMs,
		},
	})
}

func synthesizeResults(synthesisPrompt string, results []TaskResult) string {
	var sb strings.Builder
	sb.WriteString("### Parallel Subflow Execution Summary\n\n")

	if synthesisPrompt != "" {
		sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", synthesisPrompt))
	}

	for _, r := range results {
		statusIcon := "✅"
		if r.Status == "failed" {
			statusIcon = "❌"
		}
		sb.WriteString(fmt.Sprintf("#### %s %s (`%s`) - %dms\n", statusIcon, r.Title, r.TaskID, r.DurationMs))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("> **Error:** %s\n\n", r.Error))
		}
		if r.Output != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", r.Output))
		}
	}

	return sb.String()
}
