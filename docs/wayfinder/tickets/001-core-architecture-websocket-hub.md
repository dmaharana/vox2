# Ticket 001: Core Server Architecture & WebSocket Hub

**Labels:** `wayfinder:task`  
**Parent Map:** [docs/wayfinder/map.md](../map.md)  
**Status:** Closed  
**Assignee:** Agent  
**Blocked By:** None  

## Question

How should the Go backend HTTP router (Chi), zerolog structured logger, `.env` configuration manager, and bidirectional WebSocket client hub be structured for robust message dispatch and SPA embedding?

## Resolution

- Implemented `pkg/config` with `.env` loader, defaults, clone, and thread-safe dynamic updates.
- Implemented `pkg/logger` configuring zerolog for console and JSON structured outputs.
- Implemented `pkg/ws` containing the WebSocket Hub, client lifecycle, cancellation management, and structured JSON messaging protocol.
- Implemented `pkg/server` containing Chi HTTP router, CORS/middleware, health checks, runtime settings REST API, and `SPAHandler` for single-page app embedding with index.html routing fallback.
- Added comprehensive unit tests in `pkg/config`, `pkg/server`, and `pkg/ws`, all passing.

## Deliverable / Acceptance Criteria

1. Config module loading `.env` and defaults (Port, Log level, Skills dir, Memory DB path, OTel configs, LLM defaults).
2. Zerolog setup with pretty console logging in dev and JSON in prod.
3. Chi router with CORS, recovery, request logging, and health endpoint.
4. WebSocket hub supporting client connections, JSON message protocol (`user_message`, `cancel`, `settings_update`), and thread-safe broadcast / targeted client streams.
5. Base embedding structure ready for `web/dist` via Go `embed.FS`.
