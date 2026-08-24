# Task Plan: Automatic Skill Companion File & Script Manifest Discovery

## Goal
Enhance the `read_skill` tool and `skills` subsystem to automatically discover and return a structured file manifest of all available helper scripts, reference documents, and templates in the skill folder when a skill is loaded by the LLM.

## Phases

- [x] **Phase 1: Skill Companion Files Discovery (`pkg/skills/skills.go`)** (Complete)
  - Defined `SkillFile` struct (`path`, `category`, `size_bytes`).
  - Implemented `discoverSkillFiles(skillDir string) []SkillFile` that scans `scripts/`, `references/`, `templates/`, `assets/`, etc., ignoring hidden files, `.git`, `node_modules`, and `SKILL.md`.
  - Updated `read_skill` tool handler to return `directory` and `available_files` manifest.
  - *Verify:* `go test ./pkg/skills/...` passes.

- [x] **Phase 2: Unit Testing for Manifest Discovery (`pkg/skills/skills_test.go`)** (Complete)
  - Added comprehensive test case creating a multi-file skill folder containing `scripts/check_cluster.sh`, `scripts/deploy.py`, `references/schema.json`, `templates/service.yaml`.
  - Executed `read_skill` tool and verified `available_files` is accurately populated and categorized.
  - *Verify:* `go test -v ./pkg/skills/...` passes.

- [x] **Phase 3: Integration & UI Enhancements** (Complete)
  - Updated TypeScript definitions in `web/src/types/index.ts` with `SkillFile`.
  - Updated `web/src/components/ToolsSkillsModal.tsx` with script & reference count chips.
  - Verified `npm --prefix web run build` compiles with 0 errors.
  - Verified `go build -o go-harness .` compiles with 0 errors.

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| (None) | - | - |
