# 0001. Cron Scheduling Architecture

We decided to implement the background cron scheduling subsystem using `github.com/robfig/cron/v3` backed by a SQLite `cron_jobs` table, with each job execution isolated into its own dedicated conversation and guarded by a single-flight overlap policy. This preserves schedule persistence across harness restarts, avoids polluting or racing against the user's interactive chat sessions, prevents runaway LLM token spend from long-running tasks, and makes the full unified tool registry (filesystem tools, progressive skills, cognitive memory, and MCP servers) seamlessly available to recurring automated intents.

## Considered Options

- **Execution in origin conversation vs. dedicated conversation**: Running inside the active user conversation would interleave automated outputs with human prompts and disrupt user flow. Dedicated conversations provide clean, isolated audit histories per job.
- **Queueing / concurrent ticks vs. single-flight skip**: If an LLM-driven skill run takes longer than the scheduled interval, running concurrently or queueing could trigger an unbounded pile-up of expensive LLM completions and rate-limit errors. Single-flight skipping ensures stability under slow network or complex reasoning loops.
- **In-memory scheduler vs. SQLite persistence**: Storing cron entries only in memory would lose all user schedules upon server restart. SQLite persistence with WAL mode guarantees reliability.
