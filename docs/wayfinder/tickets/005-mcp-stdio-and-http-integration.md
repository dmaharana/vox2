# Ticket 005: MCP Client Integration (Stdio & HTTP)

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Open  
**Blocked By:** [Ticket 002](002-opentelemetry-tracing-subsystem.md), [Ticket 003](003-built-in-tools-and-skills-engine.md)

## Question

How should the Model Context Protocol (MCP) Go client using the standard MCP SDK (`github.com/modelcontextprotocol/go-sdk` or `github.com/mark3labs/mcp-go`) connect to external tools, prompts, and resources/artifacts over standard I/O child processes (`stdio`) and HTTP SSE streams, and register them into the harness tool registry?

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
