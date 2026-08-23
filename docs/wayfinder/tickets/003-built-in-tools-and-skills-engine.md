# Ticket 003: Built-in Tools & Dynamic Skills Loader

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should built-in file system tools (`read_file`, `write_file`, `update_file`, `list_directory`) and dynamic skill loading from the `skills/` folder (or configurable `.env` path) be implemented and registered with enable/disable toggles?

## Resolution

- Implemented `pkg/tools` with thread-safe `Registry`, tool enable/disable switches, OpenAI function definition conversion (`ToOpenAITools`), and OTel span instrumentation.
- Implemented built-in file tools: `read_file` (with line range slicing), `write_file` (with auto-directory creation), `update_file` (targeted replacement), and `list_directory` (recursive/flat traversal with metadata).
- Implemented `pkg/skills` dynamic loader supporting `SKILL.md` frontmatter and Markdown parsing, skill enable/disable toggles, and system prompt prompt injection formatting.
- Unit tests added in `pkg/tools/tools_test.go` and `pkg/skills/skills_test.go`, all passing.

## Deliverable / Acceptance Criteria

1. Built-in Tools in `pkg/tools`:
   - `read_file`: Reads text, Markdown, JSON, YAML files safely with path validation.
   - `write_file`: Writes content to file, creating parent directories if needed.
   - `update_file`: Targeted block replacement / line-range updates.
   - `list_directory`: Recursively lists directories and files with size and path info.
2. Skill Loader in `pkg/skills`:
   - Scans `skills/` directory for subdirectories containing `SKILL.md` (or YAML frontmatter + prompt/instructions).
   - Exposes skill definitions, descriptions, and prompt injection for the agent.
3. Unified Tool & Skill Registry:
   - Metadata, OpenAI tool definition JSON schemas, and execution handlers.
   - Dynamic enable/disable state per tool and skill.
