# Findings & Research

## Pi Agent Architecture (`/home/titu/sources/node-workspace/pi`)

### 1. Skills Handling (`packages/coding-agent/src/core/skills.ts`, `packages/agent/src/harness/skills.ts`)
- **Agent Skills Spec Compliance**:
  - Validates skill names: 1-64 chars, lowercase a-z, 0-9, single hyphens (`^[a-z0-9-]+$`), no leading/trailing hyphens, no consecutive hyphens.
  - Validates descriptions: Required, 1-1024 chars.
  - Supports frontmatter fields: `name`, `description`, `disable-model-invocation` (when true, hidden from system prompt XML but invokable via slash commands), `license`, `compatibility`, `metadata`, `allowed-tools`.
  - Multi-location discovery with priority:
    - Global skills: `~/.pi/agent/skills/` or `~/.agents/skills/`
    - Project skills: `./skills/`, `.agents/skills/`, `.pi/skills/`
    - Explicit paths from configuration
  - Discovery rules:
    - Directory with `SKILL.md`: treated as skill root, scans companion files (`scripts/`, `references/`, `templates/`, `assets/`), does not recurse deeper into that skill root.
    - Root `.md` files: parsed for frontmatter `description` & `name`.
    - Subdirectories without `SKILL.md`: recursively searched.
    - Ignores `.git`, `node_modules`, `__pycache__`, hidden files (`.*`), and `.gitignore`/`.ignore` files.
    - Name collisions: detects duplicates, keeps higher priority (or first found), records collision diagnostics.
  - Progressive Disclosure System Prompt (Agent Skills XML Standard):
    ```xml
    The following skills provide specialized instructions for specific tasks.
    Use the read tool to load a skill's file when the task matches its description.
    When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.

    <available_skills>
      <skill>
        <name>skill-name</name>
        <description>skill description</description>
        <location>/absolute/path/to/SKILL.md</location>
      </skill>
    </available_skills>
    ```
  - Slash command execution:
    - `/skill:<name>` or `/skill <name>` or `/<skill_name>` passing extra arguments as `User: <args>`.

### 2. File Edit Tool (`packages/coding-agent/src/core/tools/edit.ts`, `packages/coding-agent/src/core/tools/edit-diff.ts`, `packages/agent/src/harness/tools/edit.ts`)
- **Schema & Flexible Inputs**:
  - `path`: string (relative or absolute)
  - `edits`: array of `{ oldText: string, newText: string }`
  - Permissive parsing: supports single edit (`oldText`, `newText` directly) or JSON-encoded strings.
- **Core Algorithms**:
  - **Line Endings & BOM**:
    - Detects `\r\n` vs `\n`.
    - Normalizes to `\n` for matching.
    - Strips UTF-8 BOM before matching and restores BOM and original line endings on save.
  - **Exact & Fuzzy Matching**:
    - Tries exact match first (`strings.Index`).
    - Falls back to fuzzy matching:
      - NFKC unicode normalization.
      - Per-line trailing whitespace stripping (`trimEnd`).
      - Smart quotes (`’`, `‘`, `“`, `”`) to ASCII `'` and `"`.
      - Unicode dashes/hyphens (en-dash, em-dash, figure dash, etc.) to ASCII `-`.
      - Unicode spaces (NBSP, em space, thin space, etc.) to ASCII `' '`.
  - **Safety & Validation**:
    - `oldText` must not be empty.
    - `oldText` must be unique in file: if count > 1, returns `"Found X occurrences of edits[i] in <path>. Each oldText must be unique. Please provide more context to make it unique."`
    - If not found, returns `"Could not find edits[i] in <path>. The oldText must match exactly including all whitespace and newlines."`
    - All edits are matched against the *original file content*.
    - Overlap detection: if `prev.matchIndex + prev.matchLength > current.matchIndex`, returns `"edits[X] and edits[Y] overlap in <path>. Merge them into one edit or target disjoint regions."`
  - **Application**:
    - Applies replacements in reverse match order.
    - When fuzzy match was used, preserves unchanged lines from original content.
    - Checks `baseContent != newContent` to prevent no-op replacements.
  - **Diff & Patch Output**:
    - Unified patch output (`---` / `+++` / `@@ ... @@`).
    - Line-numbered display diff with context lines (default 4) and first changed line indicator.

## Go-Harness Architecture & Improvement Opportunities

### Current State in `go-harness`
1. `pkg/skills/skills.go`:
   - Single folder scanning (`./skills`).
   - Simple YAML parser for `name` and `description`.
   - Prompt format uses Markdown list `- **name**: description (N helper files)` with `read_skill` tool.
   - Does not support `disable-model-invocation`, multiple search directories (global + project), spec-compliant XML prompt generation, lenient validation diagnostics, or Agent Skills standard formatting.
2. `pkg/tools/builtin_file.go`:
   - Has `read_file`, `write_file`, `update_file` (single naive string replacement).
   - Missing the advanced `edit` / `edit_file` tool with:
     - Multi-region disjoint edits (`edits: [{oldText, newText}]`)
     - Line ending & BOM preservation
     - Exact match with fuzzy fallback (smart quotes, dashes, unicode spaces, trailing whitespace)
     - Overlap detection & uniqueness validation with rich error messages
     - Unified patch and line-numbered visual diff generation.

### Target Architecture
1. **Enhanced Skills Subsystem (`pkg/skills`)**:
   - Agent Skills Spec compliance:
     - Frontmatter parsing supporting `name`, `description`, `disable-model-invocation`, `license`, `compatibility`, `metadata`, `allowed-tools`.
     - Name validation (1-64 chars, a-z0-9-) & description validation (1-1024 chars) with diagnostics (warnings/collisions).
     - Multi-directory discovery: Global (`~/.vox/skills`, `~/.agents/skills`), Project (`./skills`, `./.agents/skills`, `./.vox/skills`), and explicit custom paths.
     - Recursive discovery of `SKILL.md` files while ignoring `.git`, `node_modules`, hidden files, and `.gitignore` patterns.
     - Companion file manifest indexing (`scripts/`, `references/`, `templates/`, `assets/`).
     - Standard XML progressive disclosure format (`<available_skills><skill>...</skill></available_skills>`) as well as Markdown listing.
     - Support for both `read_skill` and `read_file` with location paths.
     - Diagnostics tracking (warnings, collisions).
2. **Advanced File Edit Tool (`pkg/tools/edit_file.go` & `pkg/tools/diff.go`)**:
   - Implement `edit` (and `edit_file`) tool in `pkg/tools/`.
   - Implement exact + fuzzy matching with unicode/quote/dash normalization and line-ending/BOM handling.
   - Implement multi-edit validation (uniqueness, overlap detection, non-empty).
   - Implement diff and unified patch generation in Go.
   - Register in tool registry.

## MCP Robustness & Best Practices Review

### Identified Gaps in Current Go-Harness MCP Manager:
1. **Connection Lifecycle & Bounded Contexts**:
   - `ConnectServer` used an unbounded `sessionCtx := context.Background()` for `mcpClient.Connect`, risking indefinite hangs if a Stdio subprocess or remote SSE connection stalls during handshake.
2. **Resilience & Auto-Reconnect**:
   - If an MCP server restarts or drops its transport (broken pipe / EOF), subsequent tool calls fail permanently until manually reconnected via UI.
   - Missing Ping/HealthCheck capability to monitor server liveness.
3. **Tool Result Serialization**:
   - `session.CallTool` returns raw `[]mcp.Content` interfaces (`*mcp.TextContent`, `*mcp.ImageContent`, `*mcp.EmbeddedResource`).
   - LLMs receive better, clearer responses when text, images, and resources are extracted into a clean, normalized structure or text string.
4. **Environment & Stdio Process Management**:
   - Stdio commands did not expand environment variables (`$HOME`, etc.) or allow working directory configuration.
   - Needed clean termination guarantees during disconnects.
5. **Resource Discovery (`read_mcp_resource`)**:
   - Servers expose valuable resources (files, schema, live DB tables), but no tool was registered in the tool registry allowing the LLM to read MCP resources dynamically.
6. **Per-Tool Execution Timeouts**:
   - Remote MCP calls should have a sensible default timeout (e.g. 60s) so a hung server does not block the orchestrator indefinitely.

### Target MCP Enhancements:
1. **Bounded Connection Handshake**:
   - Use configurable connection timeout (default 30s) derived from parent context.
2. **Auto-Reconnect on Transient Tool Call Failures**:
   - If a tool call fails with network/pipe/session error and `srv.Enabled == true`, attempt automatic transparent reconnect and retry the tool call once.
3. **Live Health Checks (`PingServer` & `HealthCheckAll`)**:
   - `session.Ping(ctx)` support to actively verify and report server health status (`connected`, `error`, `disconnected`).
4. **Clean Tool Result Normalization**:
   - Extract text content, image data (MIME + base64), and resource references into structured output:
     ```json
     {
       "content": "...",
       "text": "...",
       "images": [...],
       "is_error": false
     }
     ```
5. **Dynamic Resource Tool (`read_mcp_resource`)**:
   - Register a `read_mcp_resource` tool into the agent tool registry when servers with resources are connected.
6. **Stdio Process Safety & Environment Expansion**:
   - Expand environment variables in command arguments.
