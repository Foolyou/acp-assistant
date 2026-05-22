## 1. Data Model And Schedule Engine

- [x] 1.1 Add cron model types for jobs, schedules, delivery, run status, and command options.
- [x] 1.2 Add SQLite migration for cron jobs and cron runs with indexes for due-job claiming and run history.
- [x] 1.3 Implement schedule parsing and next-run calculation for `at`, `every`, and five-field cron expressions.
- [x] 1.4 Add store methods for creating, listing, updating, removing, claiming, completing, and querying cron jobs/runs.

## 2. Runtime Execution

- [x] 2.1 Add assistant runtime support for executing one cron run through isolated or main session targeting.
- [x] 2.2 Add scheduler loop that periodically claims due jobs and dispatches bounded cron executions.
- [x] 2.3 Add result delivery rules for origin, none, silent success, and failure notifications.

## 3. User Commands

- [x] 3.1 Add owner/admin-only `/cron` command parsing for add, list, pause, resume, remove, run, and runs.
- [x] 3.2 Format cron command responses with job IDs, schedules, enabled state, next run time, and recent run status.
- [x] 3.3 Prevent cron-originated prompts from managing cron jobs recursively.

## 4. Integration And Verification

- [x] 4.1 Start and stop the cron scheduler from `assistant serve`.
- [x] 4.2 Add unit tests for schedule parsing, store claiming, runtime execution, delivery, and command authorization.
- [x] 4.3 Run OpenSpec validation and the project test suite.
- [x] 4.4 Commit the implementation and push the branch.
