# Ticket 008: Embedded React (Shadcn/UI) SPA & Complete Control UI

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Open  
**Blocked By:** [Ticket 001](001-core-architecture-websocket-hub.md), [Ticket 007](007-llm-orchestration-and-chat-loop.md)

## Question

How should the embedded React SPA (Vite + Tailwind + shadcn/ui) be constructed to provide chat interaction, configuration drawers for LLMs & MCP servers, tool & skill enable/disable switches, conversation management with CSV export, and disclaimer banners?

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
