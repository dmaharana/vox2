# Task Plan: Enhanced Skills, File Edit Tool, and MCP Robustness

## Goals
1. Review how skills are handled in `pi` agent (`/home/titu/sources/node-workspace/pi`). [COMPLETE]
2. Review how the file edit tool is implemented in `pi` agent (`/home/titu/sources/node-workspace/pi`). [COMPLETE]
3. Implement skill handling enhancements in `go-harness` inspired by `pi` agent. [COMPLETE]
4. Implement a file edit tool in `go-harness` inspired by `pi` agent. [COMPLETE]
5. Review MCP handling and implement best practices for production robustness in `go-harness`. [COMPLETE]
6. Test and verify all changes. [COMPLETE]

## Phases
- [x] **Phase 1: Research & Discovery**
  - Explored `pi` and `go-harness` skills, tools, and MCP subsystems.
  - Documented findings in `findings.md`.
- [x] **Phase 2: Design & Alignment**
  - Defined Agent Skills standard XML progressive disclosure format.
  - Defined file edit tool with exact/fuzzy matching, line endings, BOM, and diffs.
  - Defined MCP robustness improvements (lifecycle, health checks, auto-reconnect, normalized output, universal resource reader).
- [x] **Phase 3: Implementation - Skill Handling**
  - Enhanced `pkg/skills/skills.go` (Agent Skills YAML frontmatter, validation, diagnostics, multi-directory discovery, XML prompt).
  - Added unit tests in `pkg/skills/skills_test.go`.
- [x] **Phase 4: Implementation - File Edit Tool**
  - Implemented `pkg/tools/diff.go` and `pkg/tools/edit_file.go`.
  - Registered `edit` and `edit_file` in `pkg/tools/builtin_file.go`.
  - Added unit tests in `pkg/tools/tools_test.go`.
- [x] **Phase 5: Implementation - MCP Handling & Robustness**
  - Enhanced `pkg/mcp/manager.go` with:
    - Dedicated session lifecycle management and clean context cancellation.
    - Transparent 1-time auto-reconnect on broken pipe / network disconnects during tool calls.
    - Live health checking and liveness monitoring (`PingServer`, `HealthCheckAll`).
    - Normalized tool output structuring (`MCPToolResult` with clean text, base64 images, and error separation).
    - Universal MCP resource reader (`read_mcp_resource`).
    - Stdio environment variable expansion (`os.ExpandEnv`) and working directory (`Cwd`) support.
    - Custom per-server timeout support (`TimeoutSeconds`).
  - Added unit tests in `pkg/mcp/mcp_test.go`.
- [x] **Phase 6: Verification & System Integration**
  - Executed `go test -count=1 ./...` with 100% passing tests across all packages.
  - Verified `go build main.go` succeeds cleanly.

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Missing description fallback on non-frontmatter root markdown files in server test | 1 | Added smart fallback description extraction for standalone .md files while continuing to filter common documentation docs (README.md, AGENTS.md, etc.) |
| MCP SSE transport EOF on tool execution | 1 | Preserved session context lifetime across connection lifecycle rather than using a short-lived deferred cancellation context. |
