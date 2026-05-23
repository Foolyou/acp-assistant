## Context

ACPA cron currently exposes a host-managed scheduler through two surfaces: owner/admin `/cron` commands and a managed `acpa-cron` skill injected into Codex and Claude overlays. The harness skill asks models to emit legacy `acpa-cron` fenced JSON with `schedule_type`, `schedule_expr`, and `message` fields. This works, but it makes a privileged host protocol look like a skill and leaves the interface incompatible with the OpenClaw-style cron API chosen for P0.

The scheduler, SQLite persistence, run execution, and delivery code already live in the assistant runtime and remain the correct ownership boundary. This change replaces only the management protocol and instruction placement.

## Goals / Non-Goals

**Goals:**

- Make the canonical cron management protocol a `cron` fenced JSON block with OpenClaw-style `action`, `job`, `schedule`, `payload`, `sessionTarget`, and `delivery` fields.
- Remove the generated built-in `acpa-cron` skill from harness overlays.
- Keep cron protocol guidance in host-managed instructions/prompt prefix, not in workspace skills.
- Make `/cron` creation use the same canonical JSON schema instead of old flag parsing.
- Keep owner/admin authorization, nested cron rejection, assistant-scoped persistence, `main`/`isolated` targets, run history, and delivery behavior.

**Non-Goals:**

- Supporting backward compatibility for `acpa-cron`, `create`/`delete`, `schedule_type`, `schedule_expr`, top-level `message`, or `/cron add --every ...`.
- Adding Hermes advanced fields such as `script`, `no_agent`, `skills`, `workdir`, `profile`, or `context_from`.
- Adding OpenClaw fields that need broader runtime changes, such as `current`/`session:<id>` targets, `systemEvent`, webhook delivery, wake queue semantics, or failure alerts.
- Changing cron job/run storage tables.

## Decisions

### Canonical host protocol

The host accepts fenced JSON blocks labelled `cron` and rejects the old `acpa-cron` label. The accepted action set is `status`, `list`, `get`, `add`, `update`, `remove`, `run`, and `runs`; P0 implements these by mapping to existing runtime operations where possible. `pause` and `resume` are not actions; callers use `update` with `patch.enabled`.

Alternative considered: keep old `acpa-cron` as an alias. The user explicitly rejected compatibility, and keeping aliases would preserve two mental models.

### OpenClaw-shaped job schema with P0 subset

`add` requires `job.schedule`, `job.payload`, `job.sessionTarget`, and optional `job.delivery`. P0 accepts:

- `schedule.kind`: `at`, `every`, or `cron`
- `payload.kind`: `agentTurn`
- `sessionTarget`: `isolated` or `main`
- `delivery.mode`: `announce` or `none`
- `delivery.target`: `origin` for announce delivery

The runtime stores these values in the existing `CronJob` fields. `schedule.kind=every` uses `everyMs` as canonical input and converts it to a Go duration for the existing scheduler. `schedule.kind=cron` uses `expr` and optional `tz`. `schedule.kind=at` uses `at`.

Alternative considered: use Hermes string schedules for convenience. That is ergonomic, but it weakens the P0 goal of a single canonical protocol.

### Instructions instead of skill

The cron protocol text remains host-owned and is included through the harness prompt prefix / managed instructions path. Generated skill directories no longer include `acpa-cron`, and cron does not appear in `/skills`. This keeps privileged scheduling distinct from user/workspace skills and avoids skill selection or skill override issues.

Alternative considered: rename the skill to `cron`. That still treats a host capability as a skill and preserves the same reliability problem.

### JSON `/cron` command surface

Owner/admin IM users can send `/cron <json>` using the same action schema as the harness. Lightweight read shorthands such as `/cron list` are not part of the canonical creation API, so P0 keeps command handling focused on JSON plus direct action dispatch from parsed JSON. This prevents the old flag parser from becoming a second interface.

Alternative considered: keep `/cron add --every`. That directly conflicts with the no-compatibility decision.

## Risks / Trade-offs

- Existing reminders created through old harness prompts will fail until harness sessions receive the new instructions -> Use host prompt prefix text with explicit examples and update tests around first-prompt behavior.
- Removing the built-in skill reduces discoverability in `/skills` -> Cron is a host protocol, so docs and `/help` should point users to `/cron <json>` and natural language scheduling through the harness.
- P0 exposes an OpenClaw-shaped schema but only implements a subset -> Reject unsupported values with clear errors rather than accepting and ignoring them.
- Existing users of `/cron add --every` lose a terse command -> This is intentional for interface cleanliness; JSON examples in docs cover the replacement.

## Migration Plan

Deploy as a breaking change. Existing persisted cron jobs continue to run because storage is unchanged. New or modified jobs must use the canonical `cron` protocol. Rollback is code-only: older binaries can still read the unchanged SQLite cron tables, but they will not understand any new docs/instructions generated by this change.
