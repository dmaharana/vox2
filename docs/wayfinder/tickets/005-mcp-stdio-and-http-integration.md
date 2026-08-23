# Ticket 005: MCP Client Integration (Stdio & HTTP)

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should the Model Context Protocol (MCP) Go client using the standard MCP SDK (`github.com/modelcontextprotocol/go-sdk` or `github.com/mark3labs/mcp-go`) connect to external tools, prompts, and resources/artifacts over standard I/O child processes (`stdio`) and HTTP SSE streams, and register them into the harness tool registry?

## Resolution

- Implemented `pkg/mcp/manager.go` using the official `github.com/modelcontextprotocol/go-sdk` SDK supporting both `stdio` (child processes with args & env via `mcp.CommandTransport`) and `sse`/`http` transports via `mcp.SSEClientTransport`.
- Implemented automatic MCP tool discovery (`ListTools`), schema translation to OpenAI JSON schemas, and execution proxy with OpenTelemetry tracing spans (`mcp.call_tool.*`).
- Implemented MCP Prompts (`ListPrompts`) and Resources discovery and reading (`ListResources`, `ReadResource`).
- Added full MCP Server Management REST endpoints in `pkg/server` (list, add, connect, disconnect, delete).
- Added unit tests in `pkg/mcp/mcp_test.go` and verified 100% test pass.

## Deliverable / Acceptance Criteria

1. MCP Client Core (`pkg/mcp`):
   - Leverage standard MCP Go SDK client transports.
   - `stdio` transport: launches process, communicates via stdin/stdout pipes, handles lifecycle.
   - `http` / SSE transport: connects to remote MCP HTTP endpoints.
2. Capability discovery:
   - `tools/list` & `tools/call`: Discovers MCP tools, generates OpenAI function definitions, invokes tools and instruments with OTel spans.
   - `prompts/list` & `prompts/get`: Queries prompts from MCP servers.
   - `resources/list` & `resources/read`: Reads MCP server artifacts/resources.
3. Server configuration & management:
   - Config format supporting multiple named MCP servers with command args, env vars, and URLs.
   - Runtime connect, disconnect, and health status tracking.
