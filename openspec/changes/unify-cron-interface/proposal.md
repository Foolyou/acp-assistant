## Why

The current cron interface is ACPA-specific (`acpa-cron`, `schedule_type`, `message`, and a built-in skill), while the reference systems use cron as a host/tool protocol. Aligning P0 on an OpenClaw-style structured `cron` interface gives the assistant a cleaner contract and removes the ambiguity of exposing a privileged scheduler as a skill.

## What Changes

- **BREAKING** Replace the harness-facing `acpa-cron` fenced block with a `cron` fenced JSON block.
- **BREAKING** Replace `create`/`delete` with `add`/`remove`, and require job fields under `job` for `add`.
- **BREAKING** Replace top-level `schedule_type`, `schedule_expr`, `message`, `target`, and string `delivery` with canonical `schedule`, `payload`, `sessionTarget`, and `delivery` objects.
- **BREAKING** Stop injecting cron as a built-in harness skill; expose the cron protocol through host instructions/prompt prefix instead.
- Keep the host as the only executor: it validates schema, authorization, nested cron restrictions, persistence, and delivery.
- Keep the existing scheduler runtime semantics for P0: assistant-scoped SQLite persistence, `main`/`isolated` execution targets, `agentTurn` payload execution, and origin/no delivery behavior mapped through the new schema.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `assistant-cron-scheduler`: Replace the harness-facing cron management contract with a canonical OpenClaw-style `cron` protocol.
- `harness-overlays`: Remove cron from generated built-in skills and move privileged cron protocol instructions into the host prompt/instructions path.

## Impact

- Affected runtime code: cron fenced block parsing, harness cron call normalization, `/cron` management shape, scheduler job creation, and tests.
- Affected overlay code: managed Codex and Claude overlays should no longer install an `acpa-cron` skill.
- Affected docs/specs: `docs/cron.md`, assistant cron scheduler requirements, and harness overlay requirements.
- No new external dependencies are required.
