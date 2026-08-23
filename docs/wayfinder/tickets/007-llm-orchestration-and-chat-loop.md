# Ticket 007: OpenAI-Compatible LLM Client & Chat Loop

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should the OpenAI-compatible streaming LLM client using standard SDKs (`github.com/sashabaranov/go-openai`), multi-turn agent loop, memory context injection, dynamic tool dispatch, and streaming token broadcast be implemented?

## Resolution

- Implemented `pkg/llm/client.go` using `sashabaranov/go-openai` supporting custom `BaseURL`, `Model`, `APIKey`, temperature, and token parameters with dynamic refresh.
- Implemented `pkg/llm/orchestrator.go` autonomous multi-turn agent loop with:
  - System prompt generation injecting active skills and FTS5 retrieved memories.
  - Multi-turn conversation persistence in SQLite database.
  - Streaming token generation dispatching real-time `token` deltas over WebSocket.
  - Dynamic tool calling (built-in file tools, MCP tools, memory tools, parallel flows).
  - Context cancellation support when users request Stop/Cancel.
  - End-of-turn disclaimer `"AI can make mistakes, so double-check responses"` attached in `done` events.
- Added test suite in `pkg/llm/llm_test.go` verifying streaming chunks, mock LLM execution, and SQLite message logging.

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
