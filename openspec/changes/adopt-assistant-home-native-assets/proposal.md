## Why

The current assistant layout separates workspace and configspace as arbitrary paths, then compensates with generated harness overlays, prompt prefixes, and provider home overrides. This fights Codex and Claude Code native discovery and makes built-in ACPA behavior harder to reason about.

This change standardizes each assistant around one assistant home with a fixed `.acpa` configspace and `workspace` harness root, while moving ACPA-managed skills and instructions onto native harness loading paths.

## What Changes

- **BREAKING**: New assistants use `<assistant-home>/.acpa` for ACPA configuration and `<assistant-home>/workspace` as the harness cwd instead of accepting independent configspace and workspace paths.
- Introduce `--home` as the primary assistant-scoped CLI path, with legacy `--root`, `--configspace`, and `--workspace` retained only for migration and compatibility.
- Remove Codex `CODEX_HOME` overlay generation and rely on the user's normal Codex home plus workspace-native `.agents/skills`.
- Remove Claude generated plugin overlay for ACPA built-in skills and rely on workspace-native `.claude/skills`.
- Materialize only ACPA-owned built-in skills into workspace native skill roots, using `acpa-*` directories with ownership markers and conflict checks.
- Replace `Instructions.md` and prompt-prefix instruction injection with managed instructions rendered from `.acpa/instructions` and injected at session or launch time.
- Keep workspace project instructions in `AGENTS.md`; use `CLAUDE.md` only as a bridge to `AGENTS.md` and instruct harnesses to update `AGENTS.md` for shared project guidance.
- Convert cron management guidance into the built-in `acpa-cron` skill instead of prompt-prefix protocol injection.
- Automatically add workspace ignore rules for ACPA-managed materialized assets.

## Capabilities

### New Capabilities
- `assistant-home-layout`: Defines the assistant home layout, derived `.acpa` configspace, derived `workspace`, and path resolution behavior.

### Modified Capabilities
- `assistant-configspace`: Replace arbitrary workspace/configspace paths with assistant-home-derived `.acpa` and `workspace` paths for new assistants and runtime loading.
- `harness-overlays`: Replace generated provider home/plugin overlays with native skill materialization and provider-specific managed instruction injection.
- `assistant-cron-scheduler`: Remove prompt-prefix cron protocol injection and define cron as a built-in native skill.
- `acp-harness-runtime`: Ensure harness processes start with cwd set to the derived assistant workspace and support provider-specific managed instruction metadata/configuration.
- `workspace-memory`: Clarify that memory files live under the derived assistant workspace.
- `operator-diagnostics`: Resolve and report assistant home, derived `.acpa`, derived workspace, materialized asset state, and legacy path compatibility.
- `local-daemon-console`: Update assistant lifecycle and setup flows to create and operate on assistant home paths.
- `console-ux-redesign`: Update console path display and assistant creation fields from independent workspace/configspace paths to assistant home with derived paths.

## Impact

- CLI assistant creation, start/stop/status, doctor, logs, daemon registry, and Web console setup must accept and display assistant home as the primary path.
- `internal/configspace`, assistant model structs, registry entries, diagnostics, harness profile resolution, and runtime launch code must support derived `.acpa` and workspace paths.
- `internal/harness/overlay.go`, `internal/harness/skills.go`, launch profile generation, and related tests must be rewritten around native materialization instead of `CODEX_HOME`, Claude plugin dirs, copied user skills, and prompt prefix.
- Cron tests and runtime behavior must shift from prefix protocol injection to built-in skill availability.
- Existing assistants need a migration path or compatibility loader so old configspace/workspace layouts can still be inspected and started during transition.
