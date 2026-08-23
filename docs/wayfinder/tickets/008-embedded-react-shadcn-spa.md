# Ticket 008: Embedded React (Shadcn/UI) SPA & Complete Control UI

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should the embedded React SPA (Vite + Tailwind + shadcn/ui) be constructed to provide chat interaction, configuration drawers for LLMs & MCP servers, tool & skill enable/disable switches, conversation management with CSV export, and disclaimer banners?

## Resolution

- Implemented full React + Tailwind + Radix UI / Shadcn dashboard in `web/src/`.
- Built LLM Settings dialog (`SettingsModal.tsx`) for dynamic Base URL, Model, API Key, Temperature, and Max Tokens configuration.
- Built MCP Server Manager dialog (`MCPServersModal.tsx`) with Stdio and HTTP SSE server creation, live connect/disconnect, and status badges.
- Built Tools & Skills drawer (`ToolsSkillsModal.tsx`) allowing granular enable/disable toggles for built-in tools, MCP tools, and loaded skills.
- Built Memory Inspector (`MemoryModal.tsx`) for FTS5 cognitive search across Short-Term and Long-Term tiers, manual memory creation, and aging promotion.
- Built Conversation Manager (`ConversationsDrawer.tsx`) to switch sessions, delete history, start new chats, and export conversation records as CSV.
- Added live WebSocket message stream (`ChatMessageList.tsx`) with real-time markdown token rendering, collapsible tool call cards, parallel subflow progress cards, and memory context chips.
- Added user disclaimer footer: `"AI can make mistakes, so double-check responses"` at the bottom of the chat interface.
- Embedded `web/dist` directly into the Go backend binary via `embed.FS` with fallback client-side routing.
- Built production binary `go-harness` successfully.

## Deliverable / Acceptance Criteria

1. Chat Interface:
   - Message stream list displaying user prompts, assistant text (markdown rendered), tool call accordion cards, and subflow progress indicators.
   - Message input box with send & stop buttons.
   - Disclaimer footer: `"AI can make mistakes, so double-check responses"` in small font at the bottom of the chat view.
2. Configuration Panels / Modals:
   - **LLM Settings:** LLM URL (OpenAI / Ollama / vLLM / LocalAI), Model Name, API Key, Temperature, Max Tokens.
   - **MCP Servers:** Form to add/edit/delete/toggle HTTP and STDIO MCP servers with live connection test.
   - **Tools & Skills:** Toggle switches to enable/disable built-in tools, MCP tools, and loaded skills.
   - **Memory Inspector & Search:** Tab to query FTS5 memories, view short-term vs long-term items, and add manual memory entries.
   - **Conversation Manager:** Drawer to list, load, save, delete sessions, and export conversation history to CSV.
3. Build & Embedding:
   - Production Vite build outputting to `web/dist` embedded seamlessly into the Go binary.
