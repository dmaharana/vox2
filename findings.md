# Findings & Architecture Insights

## Skill Companion File Manifest Architecture
1. **Skill Folder Organization**:
   - Skills can be organized as modular folders:
     - `skills/<skill_name>/SKILL.md` (Main instructions & frontmatter)
     - `skills/<skill_name>/scripts/*` (Executable scripts e.g. `.sh`, `.py`, `.js`)
     - `skills/<skill_name>/references/*` (Reference data, schemas, policies e.g. `.json`, `.yaml`, `.md`)
     - `skills/<skill_name>/templates/*` (File templates e.g. `.yaml`, `.j2`, `.env.example`)
2. **Dynamic Manifest in `read_skill`**:
   - When the LLM loads a skill via `read_skill`, `pkg/skills/skills.go` will inspect the skill's root folder and return:
     - `directory`: Full filesystem path to the skill directory.
     - `available_files`: List of `{ path: "scripts/deploy.sh", category: "script", size_bytes: 1234 }`.
   - Categories:
     - `script`: in `scripts/` or extensions `.sh`, `.py`, `.js`, `.ts`, `.bash`, `.rb`
     - `template`: in `templates/`
     - `reference`: in `references/` or docs/data formats
     - `asset`: images/binaries
     - `other`: general files
   - This provides zero-effort script and reference discovery for LLMs without requiring the skill author to manually list every file.
