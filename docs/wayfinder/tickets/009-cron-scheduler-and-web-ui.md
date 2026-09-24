# Ticket 009: Background Cron Scheduler, /cron Slash Command & Web UI

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Completed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should background scheduled intents (e.g. `/cron "*/10 * * * *" -- /check-stock-price AMD`), persistent SQLite scheduling, dedicated execution conversations, single-flight overlap protection, chat slash command controls, and complete Web UI cron management be integrated into the Go Agent Harness?

## Resolution

- Implemented `pkg/cron` scheduler engine powered by `github.com/robfig/cron/v3` with single-flight mutex protection per job.
- Created `cron_jobs` SQLite table with full schema migrations and persistence across restarts in `pkg/db`.
- Integrated full tool availability (internal tools, progressive skills, cognitive memory, MCP tools) and slash command parsing in `pkg/llm/orchestrator.go`.
- Added `/cron` slash command supporting creation (`/cron [name:<name>] "<expr>" [--] <intent>`), listing (`/cron list`), pausing (`/cron pause <id>`), resuming (`/cron resume <id>`), manual trigger (`/cron run <id>`), deletion (`/cron delete <id>`), and help.
- Implemented REST API endpoints under `/api/cron/jobs` for listing, creating, toggling, manually running, and deleting jobs.
- Implemented `CronModal.tsx`, sidebar navigation button with active count badge in `AppSidebar.tsx`, skill card "Schedule" deep links in `ToolsSkillsModal.tsx`, and autocomplete in `SlashAutocomplete.tsx`.

## Deliverable / Acceptance Criteria

1. **Database & Persistence (`pkg/db`)**:
   - `cron_jobs` table storing `id`, `name`, `schedule`, `intent`, `conversation_id`, `enabled`, `last_run`, `next_run`, `last_status`, `last_error`, `created_at`, `updated_at`.
   - Thread-safe CRUD operations.
2. **Cron Scheduler Engine (`pkg/cron`)**:
   - `Manager` wrapping `robfig/cron/v3` supporting 5-field cron expressions and standard descriptors (`@every 10m`, `@hourly`).
   - Single-flight overlap protection ensuring long-running LLM tasks drop overlapping ticks safely.
   - Isolated execution in dedicated cron conversations (`[Cron] <name/intent>`).
   - Availability of all tools (internal, skills, memory, MCP) to scheduled runs.
3. **Chat Slash Command (`pkg/llm/orchestrator.go`)**:
   - Intercepts `/cron` in chat.
   - Supports creation syntax `/cron "<expr>" -- <intent>` and `/cron name:<name> "<expr>" -- <intent>`.
   - Supports `/cron list`, `/cron pause <id>`, `/cron resume <id>`, `/cron run <id>`, `/cron delete <id>`, `/cron help`.
   - Supports referencing jobs by alias name or `#<index>` or UUID.
4. **REST API (`pkg/server`)**:
   - `GET /api/cron/jobs`
   - `POST /api/cron/jobs`
   - `POST /api/cron/jobs/{id}/toggle`
   - `POST /api/cron/jobs/{id}/run`
   - `DELETE /api/cron/jobs/{id}`
5. **Web UI (`web/src`)**:
   - `CronModal.tsx` for viewing, toggling, triggering, deleting, and creating jobs with quick schedule presets.
   - Sidebar item with active count badge in `AppSidebar.tsx`.
   - "Schedule" deep link button on each skill card in `ToolsSkillsModal.tsx`.
   - Autocomplete in `SlashAutocomplete.tsx`.
