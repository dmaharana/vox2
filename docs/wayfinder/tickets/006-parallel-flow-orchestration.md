# Ticket 006: Parallel Subflow Orchestration Engine

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Open  
**Blocked By:** [Ticket 002](002-opentelemetry-tracing-subsystem.md), [Ticket 003](003-built-in-tools-and-skills-engine.md)

## Question

How should the agent orchestrate concurrent sub-tasks via a `spawn_parallel_flow` tool, executing independent actions in parallel worker goroutines, capturing telemetry/logs, and synthesizing/summarizing outputs back to the main agent loop?

## Deliverable / Acceptance Criteria

1. Flow Engine (`pkg/flow`):
   - Flow executor accepting parallel task definitions (`id`, `instruction`, `tools_allowed`, `context`).
   - Concurrency controller with timeout, error handling, and goroutine pool management.
2. Parallel Flow Tool Registration:
   - Tool `spawn_parallel_flow` exposed to main LLM agent loop.
   - Executes sub-agent prompts in parallel, collects individual outputs.
3. Fan-in & Summarization:
   - Aggregates subflow outputs into a structured summary for the parent loop.
   - Traced with OTel parent-child span hierarchy.
   - Emits real-time progress events over WebSockets to the UI.
