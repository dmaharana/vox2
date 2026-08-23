package flow

import (
	"context"
	"encoding/json"
	"fmt"

	"go-harness/pkg/tools"

	"github.com/sashabaranov/go-openai/jsonschema"
)

// RegisterFlowTools adds the spawn_parallel_flow tool into the registry.
func RegisterFlowTools(reg *tools.Registry, engine *Engine, runner TaskRunner) {
	reg.Register(tools.ToolDefinition{
		Name:        "spawn_parallel_flow",
		Description: "Spins up multiple concurrent sub-tasks/sub-agents in parallel to perform independent actions, then aggregates and summarizes their results back to the main loop.",
		Category:    "flow",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"synthesis_prompt": {
					Type:        jsonschema.String,
					Description: "Overall objective or instruction on how the combined outputs should be summarized.",
				},
				"tasks": {
					Type:        jsonschema.Array,
					Description: "List of independent sub-tasks to run in parallel.",
					Items: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"id": {
								Type:        jsonschema.String,
								Description: "Unique identifier for this task (e.g. 'task_1')",
							},
							"title": {
								Type:        jsonschema.String,
								Description: "Short descriptive name of the sub-task",
							},
							"instruction": {
								Type:        jsonschema.String,
								Description: "Specific prompt or instruction for this sub-task",
							},
							"context": {
								Type:        jsonschema.String,
								Description: "Additional context or background data for this task",
							},
						},
						Required: []string{"title", "instruction"},
					},
				},
			},
			Required: []string{"tasks"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in ParallelFlowInput
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments for spawn_parallel_flow: %w", err)
			}

			if len(in.Tasks) == 0 {
				return nil, fmt.Errorf("tasks array cannot be empty")
			}

			output, err := engine.ExecuteParallel(ctx, in, runner)
			if err != nil {
				return nil, err
			}

			return map[string]any{
				"flow_id":     output.FlowID,
				"total_tasks": output.TotalTasks,
				"completed":   output.Completed,
				"failed":      output.Failed,
				"summary":     output.Summary,
				"results":     output.TaskResults,
				"duration_ms": output.DurationMs,
			}, nil
		},
	})
}
