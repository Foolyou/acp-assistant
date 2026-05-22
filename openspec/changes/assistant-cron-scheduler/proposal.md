## Why

ACP Assistant needs a first-class way for users and operators to schedule recurring assistant work such as reminders, daily reports, workspace checks, and periodic diagnostics. External system cron cannot safely preserve assistant ownership, IM delivery routes, harness sessions, permission policy, or run history, so scheduled work should be modeled inside the assistant runtime.

## What Changes

- Add assistant-scoped cron jobs persisted in each assistant's SQLite event store.
- Add cron run history so each scheduled execution is auditable and diagnosable.
- Add a scheduler loop to `assistant serve` that claims due jobs, executes them through the configured harness, records results, and sends delivery messages when configured.
- Add owner-only `/cron` IM commands for creating, listing, pausing, resuming, removing, manually running, and inspecting scheduled jobs.
- Support `at`, `every`, and basic five-field cron schedules with timezone-aware next-run calculation.
- Support `origin` and `none` delivery modes for the first version.
- Keep scheduled runs isolated by default while allowing explicit reuse of the creator's main session.
- Prevent cron-triggered prompts from managing cron jobs recursively in the first version.

## Capabilities

### New Capabilities

- `assistant-cron-scheduler`: Defines assistant-scoped scheduled jobs, run execution, delivery, ownership, and operator visibility.

### Modified Capabilities

- None.

## Impact

- Store schema gains cron job and cron run tables through a new migration.
- `internal/model` gains cron job, schedule, delivery, and run types.
- `internal/store` gains CRUD, due-claiming, completion, and run query methods.
- `internal/assistant` gains scheduler execution and `/cron` command handling.
- `cmd/acpa assistant serve` starts and stops the cron scheduler alongside connector accounts and permission expiry.
- Tests cover schedule parsing, store claiming semantics, command authorization, execution, delivery, and run history.
