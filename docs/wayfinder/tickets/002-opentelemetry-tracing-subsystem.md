# Ticket 002: OpenTelemetry Tracing Pipeline

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should the OpenTelemetry SDK tracer provider be configured to support flexible multi-destination exporting (Console stdout, local rotating file, or remote OTLP gRPC/HTTP collector) across LLM calls, tool executions, subflows, and skill evaluations?

## Resolution

- Implemented `pkg/tracing` supporting configurable exporter pipelines (`console`, `file`, `otlp`, `all`, `none`).
- Added composite text-map propagation, batch/simple span processors, and clean shutdown.
- Implemented standard span helpers for LLM chat completion spans, tool execution spans, skill usage spans, and parallel flow spans.
- Added comprehensive unit tests in `pkg/tracing/tracing_test.go` verifying span creation, attributes, and file flushing.

## Deliverable / Acceptance Criteria

1. `pkg/tracing` package initializing tracer provider with resource attributes (service name, version).
2. Exporters dynamically selectable via config/env:
   - `console`: stdout span dumper.
   - `file`: writes JSON/structured spans to a configured trace file path.
   - `otlp`: remote OTLP collector endpoint.
   - `all` or composite multi-exporter.
3. Helper wrappers / span helpers for:
   - LLM generation spans (with token counts, prompt metadata, duration).
   - Tool execution spans (tool name, input args, output/error).
   - Skill loading & invocation spans.
   - Subflow orchestration spans.
