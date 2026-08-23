# Map: Go Agent Harness with Embedded Shadcn SPA

**Label:** `wayfinder:map`  
**Status:** In Progress  

## Destination

A production-ready, modular Go-based AI agent harness with an embedded React (Vite + Tailwind + shadcn/ui) SPA, featuring OpenAI-compatible LLM orchestration, dynamic skill & MCP (stdio & HTTP) tool discovery, parallel subflow execution, multi-tier SQLite memory (short-term & long-term with FTS5 search) + conversation persistence, configurable OpenTelemetry tracing (console/file/OTLP), chi router, zerolog, and complete UI configuration controls.

## Notes

- **Domain:** AI Agent Harness, Golang Backend, React SPA Frontend, Model Context Protocol (MCP), OpenTelemetry.
- **Key Architectural Decisions:**
  - Backend: Go 1.26+ with `go-chi/chi/v5`, `rs/zerolog`, and `embed.FS` for single-binary distribution.
  - LLM SDK: Standard robust OpenAI-compatible Go SDK (`github.com/sashabaranov/go-openai`) / Google Gen AI SDK / Genkit for streaming and function calling.
  - MCP SDK: Official / standard MCP Go SDK (`github.com/modelcontextprotocol/go-sdk` / `github.com/mark3labs/mcp-go`) for stdio and SSE client transports.
  - Streaming Protocol: WebSockets for bidirectional full-duplex communication (streaming tokens, tool progress, cancellations).
  - Memory Engine: Pure SQLite with FTS5 virtual tables for keyword/tag search, metadata tracking (`last_used`, hit counts), and automated long-term memory promotion.
  - Flow Orchestration: Agent tool-driven (`spawn_parallel_flow`) running sub-agent flows concurrently with fan-in summarization.
  - Tracing: OpenTelemetry Go SDK (`go.opentelemetry.io/otel`) with configurable pipeline exporters (Console, File, OTLP endpoint).
- **Reference Guidelines:** Simplicity first, surgical modular design, clean separation of concerns in Go packages (`pkg/llm`, `pkg/tools`, `pkg/skills`, `pkg/mcp`, `pkg/memory`, `pkg/flow`, `pkg/tracing`, `pkg/server`).

## Decisions so far

<!-- the index: one line per closed ticket, enough to judge relevance, then zoom the link for the detail the ticket holds -->

- [[Ticket 001] Core Server Architecture & WebSocket Hub](tickets/001-core-architecture-websocket-hub.md): Chi router with CORS/middlewares, Zerolog, `.env` dynamic config, and WebSocket Hub with full-duplex JSON messaging and cancellation.
- [[Ticket 002] OpenTelemetry Tracing Pipeline](tickets/002-opentelemetry-tracing-subsystem.md): Configurable tracer provider with console, file, OTLP exporters, and span helpers for LLM, tools, skills, and flows.
- [[Ticket 003] Built-in Tools & Dynamic Skills Loader](tickets/003-built-in-tools-and-skills-engine.md): File tools (read/write/update/list), dynamic `skills/` loader, prompt injection, and unified tool registry.
- [[Ticket 004] SQLite Persistence: Conversations & FTS5 Tiered Memory](tickets/004-sqlite-conversation-and-fts5-memory.md): SQLite schema with WAL mode, conversation CRUD + CSV export, and cognitive tiered memory with FTS5 search and aging promotion.

## Frontier & Open Tickets

1. [[Ticket 005] MCP Client Integration (Stdio & HTTP)](tickets/005-mcp-stdio-and-http-integration.md) (Unblocked - Frontier)
2. [[Ticket 006] Parallel Subflow Orchestration Engine](tickets/006-parallel-flow-orchestration.md) (Unblocked - Frontier)
3. [[Ticket 007] OpenAI-Compatible LLM Client & Chat Loop](tickets/007-llm-orchestration-and-chat-loop.md) (Blocked by Ticket 005, Ticket 006)
4. [[Ticket 008] Embedded React (Shadcn/UI) SPA & Complete Control UI](tickets/008-embedded-react-shadcn-spa.md) (Blocked by Ticket 007)

## Not yet specified

- **Multi-user Authentication & Access Control:** RBAC / API keys for multi-tenant deployments (deferred for single-user local harness).
- **Audio/Voice Multimodal I/O:** Whisper speech-to-text / TTS integration in UI.
- **Custom MCP Server Hosting:** Allowing the Go harness to expose its own built-in tools as an MCP server to external clients.

## Out of scope

- **External Cloud Vector DBs:** Pinecone, Qdrant, Weaviate (ruled out in favor of pure SQLite FTS5 for local zero-dependency operation).
- **Kubernetes / Distributed Node Orchestration:** Single-binary standalone deployment is the target.
