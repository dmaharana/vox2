# Ticket 004: SQLite Persistence: Conversations & FTS5 Tiered Memory

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should SQLite be designed to store multi-session conversations (CRUD + CSV export) and maintain a tiered cognitive memory model (Short-Term: working context, semantic cache; Long-Term: semantic, episodic, procedural) with FTS5 keyword/tag search and automatic aging promotion?

## Resolution

- Implemented `pkg/db` using pure Go `modernc.org/sqlite` with WAL mode, foreign keys, and table migrations for `conversations`, `messages`, `memories`, and `memories_fts` (FTS5).
- Implemented Conversation CRUD methods and full CSV export generation.
- Implemented `pkg/memory` tiered cognitive manager supporting short-term (`working`, `semantic_cache`) and long-term (`semantic`, `episodic`, `procedural`) memories with FTS5 BM25 search, access counter tracking, and automatic aging promotion.
- Integrated `save_memory` and `search_memory` tools into agent tool registry.
- Added REST API routes in `pkg/server` for conversations, CSV download, and memory queries.
- Added unit test suites in `pkg/db/conversation_test.go` and `pkg/memory/memory_test.go`, all passing.

## Deliverable / Acceptance Criteria

1. SQLite Database initialization with modern driver (e.g. `modernc.org/sqlite` pure Go):
   - Conversation table: `id`, `title`, `created_at`, `updated_at`.
   - Message table: `id`, `conversation_id`, `role`, `content`, `tool_calls`, `created_at`.
2. Conversation CRUD API & CSV export:
   - Save session, load history, list sessions, delete session, export messages as CSV.
3. Tiered Memory Subsystem (`pkg/memory`):
   - SQLite tables with FTS5 virtual table for indexing memory content & tags.
   - Memory types: `working`, `semantic_cache`, `semantic`, `episodic`, `procedural`.
   - Metadata: `id`, `type`, `key`, `content`, `tags`, `access_count`, `last_used_at`, `created_at`.
   - Add/Update memory item, Search memories (FTS5 ranking + type filters), and Auto-promotion/aging logic for long-term retention.
