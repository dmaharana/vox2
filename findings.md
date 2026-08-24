# Findings & Architecture Insights

## Autocomplete Design for Skills, Tools & Slash Commands

1. **Frontend Autocomplete (`web/src/components/SlashAutocomplete.tsx`)**:
   - Triggers whenever `input.startsWith('/')` and before the first whitespace.
   - Dynamically aggregates:
     - **System Commands**: `/help`, `/skills`, `/tools` (badge: `Command`, icon: `Terminal`).
     - **Discovered Skills**: `/<skill_name>` (badge: `Skill`, icon: `Sparkles`).
     - **Registered Tools**: `/<tool_name>` (badge: category e.g. `builtin`, `memory`, `flow`, `mcp`, icon: `Wrench`).
   - Filters in real-time as user types after `/`.
   - Full keyboard accessibility: `ArrowUp`/`ArrowDown` for selection, `Tab`/`Enter` to autocomplete, and `Escape` to dismiss.
   - Positioned in a glassmorphic popover above the input textarea with automatic scrolling for active items.

2. **Backend Direct Execution & Directives (`pkg/llm/orchestrator.go`)**:
   - `/tools`: Instantly outputs an interactive table of all registered tools and their categories without LLM overhead.
   - `/<tool_name> [args]` / `/tool <name> [args]`: Directly primes the LLM with an explicit directive to execute that specific tool.
   - `/<skill_name> [prompt]` / `/skill <name> [prompt]`: Injects that skill's full instructions into the current turn.
