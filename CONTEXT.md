# Go Agent Harness

A modular Go-based AI agent harness with an embedded React SPA, local tool & skill orchestration, SQLite persistence, and scheduled task automation.

## Language

**Cron Job**:
A persistent scheduled task definition specifying a recurrence rule and a target intent to execute.
_Avoid_: Scheduled script, periodic worker, background timer

**Intent**:
The slash command, skill invocation, or natural-language prompt to be executed by the agent harness when triggered.
_Avoid_: Payload, prompt task, instruction string

**Cron Expression**:
A standard 5-field cron time expression or supported descriptor defining the recurrence interval for a Cron Job.
_Avoid_: Recurrence string, crontab entry, cron timer

**Dedicated Cron Conversation**:
An isolated conversation thread bound to a specific Cron Job where its periodic execution turns and tool outputs are recorded.
_Avoid_: Background session, runner log, cron thread

**Single-Flight Guard**:
An execution safety policy that skips an upcoming scheduled tick if the previous execution of that Cron Job is still actively processing.
_Avoid_: Debounce, lock timeout, concurrency limiter

**Job Alias**:
A user-friendly alphanumeric name or short numeric index assigned to a Cron Job for quick reference in chat commands and the UI.
_Avoid_: Job tag, task nickname, job slug
