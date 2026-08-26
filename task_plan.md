# Task Plan: Enhance Skills & Implement File Edit Tool (from Pi Agent)

## Goals
1. Review how skills are handled in `pi` agent (`/home/titu/sources/node-workspace/pi`). [COMPLETE]
2. Review how the file edit tool is implemented in `pi` agent (`/home/titu/sources/node-workspace/pi`). [COMPLETE]
3. Examine current architecture of `go-harness` to understand how tools, skills, and prompts are structured. [COMPLETE]
4. Implement skill handling enhancements in `go-harness` inspired by `pi` agent. [COMPLETE]
5. Implement a file edit tool in `go-harness` inspired by `pi` agent. [COMPLETE]
6. Test and verify all changes. [COMPLETE]

## Phases
- [x] **Phase 1: Research & Discovery**
  - Explore `pi` codebase for skill management and file edit tool implementation.
  - Explore `go-harness` codebase for existing tool definitions, skill loading, prompt generation, and execution.
  - Document findings in `findings.md`.
- [x] **Phase 2: Design & Alignment**
  - Define exact skill model changes in `go-harness` (multi-path discovery, frontmatter validation, Agent Skills XML format, `disable-model-invocation`, diagnostics).
  - Define file edit tool interface and implementation in Go (multi-edit array, exact/fuzzy matching, line endings, BOM, collision/overlap checking, diff & unified patch generation).
  - Document design decisions in `task_plan.md` & `findings.md`.
- [x] **Phase 3: Implementation - Skill Handling**
  - Enhanced `pkg/skills/skills.go`:
    - Spec-compliant YAML frontmatter parser and validation rules (name, description, `disable-model-invocation`, `license`, `compatibility`, `allowed-tools`, `metadata`).
    - Multi-directory loader with priority (global user skills, project skills, custom paths) and collision diagnostics.
    - Recursive `SKILL.md` detection + `.gitignore` / hidden file filtering.
    - Agent Skills standard XML progressive disclosure system prompt generator (`<available_skills>...`).
    - Enhanced `read_skill` tool and companion files support.
  - Added comprehensive unit tests in `pkg/skills/skills_test.go`.
- [x] **Phase 4: Implementation - File Edit Tool**
  - Implemented `pkg/tools/diff.go`:
    - LCS diff algorithm for line-by-line diffing.
    - Unified patch generator (`GenerateUnifiedPatch`) with hunk headers and context lines.
    - Visual line-numbered diff generator (`GenerateDisplayDiff`) with first changed line detection.
  - Implemented `pkg/tools/edit_file.go`:
    - Multi-edit schema: `path`, `edits: [{oldText, newText}]` + permissive legacy/single edit adapter.
    - CRLF/LF line-ending detection, normalization, and restoration.
    - BOM detection and restoration.
    - Exact match + fuzzy normalization (NFKC, smart quotes, dashes, unicode spaces, line trailing spaces).
    - Validation: non-empty, uniqueness count check, non-overlapping check against original base content, no-op change check.
    - Registered both `edit` and `edit_file` in builtin tools (`RegisterBuiltinTools`).
  - Added comprehensive unit tests in `pkg/tools/tools_test.go`.
- [x] **Phase 5: Verification & System Integration**
  - Updated `pkg/llm/orchestrator.go`:
    - Added `/skill:<name>` and argument forwarding support for slash commands.
    - Added `FILE EDITING & MUTATION RULES` to the system prompt.
  - Executed `go test -count=1 ./...` with 100% passing test coverage.
  - Verified `go build main.go` succeeds cleanly.

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Missing description fallback on non-frontmatter root markdown files in server test | 1 | Added smart fallback description extraction for standalone .md files while continuing to filter common documentation docs (README.md, AGENTS.md, etc.) |
