# Ticket 007: OpenAI-Compatible LLM Client & Chat Loop

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Open  
**Blocked By:** [Ticket 001](001-core-architecture-websocket-hub.md), [Ticket 003](003-built-in-tools-and-skills-engine.md), [Ticket 004](004-sqlite-conversation-and-fts5-memory.md), [Ticket 005](005-mcp-stdio-and-http-integration.md), [Ticket 006](006-parallel-flow-orchestration.md)

## Question

How should the OpenAI-compatible streaming LLM client using standard SDKs (`github.com/sashabaranov/go-openai`), multi-turn agent loop, memory context injection, dynamic tool dispatch, and streaming token broadcast be implemented?

## Deliverable / Acceptance Criteria

1. OpenAI API client using `github.com/sashabaranov/go-openai` supporting custom `BaseURL`, custom model names, headers, and API keys.
2. Streaming chat completion parser with tool call chunk assembly.
3. Autonomous Multi-Turn Agent Loop:
   - System prompt assembly (including active skills and relevant retrieved memories).
   - Conversation history inclusion (user questions, assistant responses, tool calls).
   - Dynamic tool schema injection (built-in file tools, MCP tools, parallel flow tool, memory tools).
   - Execution of tool calls returned by LLM, recording tool outputs, looping until final text or max iterations.
4. WebSocket event dispatcher:
   - Streams text deltas (`token`), tool start/finish events (`tool_call`), memory search hits, and status updates.
   - Context cancellation support when user hits "Stop/Cancel".
