## Context

Each ACP Assistant instance has a fixed configspace, workspace, harness binding, SQLite event store, IM channel set, and `assistant serve` process. Inbound IM messages already flow through `assistant.Runtime`, which owns session selection, command handling, permission policy, harness prompting, and outbound delivery. The daemon supervises assistant worker processes and exposes lifecycle APIs, but it does not own assistant-specific conversation state.

Cron must therefore live inside the assistant worker runtime rather than in system cron or the daemon supervisor. This keeps scheduled work close to the harness, store, permission policy, and sender while still allowing daemon/console inspection later.

## Goals / Non-Goals

**Goals:**

- Persist scheduled jobs and runs in each assistant's SQLite store.
- Execute due jobs from `assistant serve` using the configured harness.
- Provide owner/admin-only `/cron` commands through IM.
- Provide a built-in harness cron skill for model-mediated create/delete/list operations.
- Support one-time, fixed-interval, and five-field cron schedules.
- Make each run auditable with status, timestamps, final text, and errors.
- Deliver successful results back to the creating IM route by default, with opt-out delivery.
- Keep first-version execution deterministic and conservative.

**Non-Goals:**

- Host-side natural-language schedule parsing.
- Script-only jobs, pre-run shell hooks, job chaining, or per-job toolset controls.
- Console UI for managing jobs.
- Cross-assistant or global cron jobs.
- Cron jobs that can create or mutate other cron jobs during their own run.

## Decisions

### Assistant-scoped SQLite persistence

Cron jobs and runs will be stored in new tables in the assistant event DB. This matches existing sessions, permissions, connector statuses, and events, and avoids introducing another persistence format under configspace. A store-level claim method will atomically select due jobs and mark them running so repeated scheduler ticks do not double-start the same due execution.

Alternative considered: JSON files under configspace, like Hermes. JSON is easy to inspect, but this project already uses SQLite for event/state data and needs atomic claiming.

### Scheduler runs in `assistant serve`

`assistant serve` will start a cron scheduler goroutine alongside connector accounts and the permission expiry ticker. The scheduler ticks at a fixed interval, claims due jobs, starts bounded goroutines for runs, and stops when the assistant process receives shutdown.

Alternative considered: daemon-owned scheduler. That would require the daemon to understand harness sessions, IM delivery, and assistant store details, expanding its responsibility beyond lifecycle supervision.

### Conservative execution targets

First version supports:

- `direct`: deliver the stored prompt text directly without invoking the harness.
- `isolated`: create or use a cron-specific local session for scheduled work.
- `main`: reuse the creator's owner binding active session.

`direct` is preferred for simple reminders because the expected output is already known and should not be reinterpreted by a harness. `isolated` is the default for scheduled assistant work because scheduled prompts must be self-contained and should not unexpectedly pollute user chat context. `main` remains available for jobs that intentionally continue the owner conversation.

### Harness-facing built-in cron skill

Cron is exposed to the harness as a built-in `acpa-cron` skill injected into managed Codex and Claude overlays. The same cron protocol is also included in the first prompt sent to each ACP session so the behavior does not depend on provider-specific skill auto-selection, while avoiding repeated instruction pollution in the long-lived session history. When the user asks for reminders or scheduled work in natural language, the harness is responsible for interpreting the request and returning a fenced `acpa-cron` JSON block. The assistant runtime treats that block as a host tool call, validates authorization and fields, persists the job, and sends the final confirmation or error.

The supported harness actions are:

- `create`: create a job with `schedule_type`, `schedule_expr`, `timezone`, `message`, `target`, and `delivery`.
- `delete`: remove a job by `job_id`.
- `list`: list assistant cron jobs.

This avoids a narrow runtime regex parser while keeping persistence, authorization, and execution deterministic in the host.

### Command management surface

The deterministic operator surface is `/cron` inside IM:

- `/cron add --every <duration> --name <name> --message <prompt>`
- `/cron add --at <time> --name <name> --message <prompt>`
- `/cron add --cron <expr> --name <name> --message <prompt>`
- `/cron list`
- `/cron run <id>`
- `/cron pause <id>`
- `/cron resume <id>`
- `/cron remove <id>`
- `/cron runs <id>`

Only owner/admin users can mutate jobs. Listing and runs are also owner/admin-only in the first version because jobs may contain private prompts.

The command surface remains useful for exact operator control and debugging even though the harness skill is the preferred path for natural-language scheduling requests.

### Delivery modes

First version supports:

- `origin`: send the final result to the IM route that created the job.
- `none`: record the run but do not send successful output.

Failures are sent to the origin route when available. If successful output begins with `[SILENT]`, the run is recorded but no success message is sent.

### Schedule calculation

The scheduler will store `next_run_at` in UTC. `at` jobs run once and are disabled after completion. `every` jobs calculate the next run from the current scheduled run time, not the finish time, so delayed ticks do not permanently drift. Basic cron expressions support five fields: minute, hour, day-of-month, month, and day-of-week. Timezone defaults to `UTC` unless set by command.

## Risks / Trade-offs

- Cron syntax ambiguity -> Keep host schedule parsing small, let the harness interpret natural language, document five-field support, and reject unsupported structured forms with explicit errors.
- Duplicate execution on fast ticks or slow runs -> Use store-level claiming and per-job max concurrency of one.
- Long-running jobs blocking scheduler progress -> Run claimed jobs in separate goroutines and mark timed-out jobs when context deadline is reached.
- Prompts lacking context -> Default to isolated sessions and require `/cron add --message` prompts to be self-contained.
- Sensitive output delivery -> Default delivery only to creator origin and allow `--deliver none`.
- Recursive cron mutation -> Mark cron-generated inbound context and reject `/cron` management commands during scheduled runs.

## Migration Plan

Add a new SQL migration for cron tables and indexes. Existing assistants migrate automatically through `Store.Migrate` when `assistant serve` starts. Rollback for code deployment is safe as old code ignores the new tables; rollback for data is not required because tables are additive.
