## 1. Assistant Home Layout

- [x] 1.1 Add assistant home path helpers that derive `<home>/.acpa` and `<home>/workspace`.
- [x] 1.2 Update assistant config model and persistence to support new-layout assistant home while retaining legacy workspace/configspace fields.
- [x] 1.3 Update assistant creation defaults to create `<home>/.acpa` and `<home>/workspace`.
- [x] 1.4 Add CLI `--home` resolution for assistant-scoped commands.
- [x] 1.5 Preserve compatibility resolution for legacy `--root`, `--configspace`, `--workspace`, and registry entries.
- [x] 1.6 Update registry entries to record assistant home for new assistants.

## 2. Native Managed Skills

- [x] 2.1 Define bundled ACPA built-in skill sources by provider, including `acpa-cron`.
- [x] 2.2 Implement Codex materialization into `<workspace>/.agents/skills/acpa-*`.
- [x] 2.3 Implement Claude materialization into `<workspace>/.claude/skills/acpa-*`.
- [x] 2.4 Add `.acpa-managed.json` ownership marker generation and validation.
- [x] 2.5 Fail safely on unowned `acpa-*` directory collisions.
- [x] 2.6 Automatically append missing workspace `.gitignore` rules for `.agents/skills/acpa-*/` and `.claude/skills/acpa-*/`.

## 3. Managed Instructions

- [x] 3.1 Initialize `.acpa/instructions/common.md`, `.acpa/instructions/codex.md`, and `.acpa/instructions/claude.md`.
- [x] 3.2 Render provider managed instructions from common plus provider-specific files.
- [x] 3.3 Inject Codex managed instructions through `developer_instructions` launch config override.
- [x] 3.4 Inject Claude managed instructions through `session/new._meta.systemPrompt` append metadata.
- [x] 3.5 Remove first-turn prompt-prefix usage for managed instructions.
- [x] 3.6 Ensure managed instructions direct all harnesses to update `AGENTS.md` for shared project guidance.

## 4. Workspace Instructions

- [x] 4.1 Initialize or preserve workspace `AGENTS.md` for new assistants.
- [x] 4.2 Initialize or repair `CLAUDE.md` as a bridge to `AGENTS.md` without treating it as shared guidance storage.
- [x] 4.3 Remove `Instructions.md` creation and loading paths.
- [x] 4.4 Update `/skills`, `/help`, and related operator text to reference native skill roots and `AGENTS.md`.

## 5. Harness Launch Cleanup

- [x] 5.1 Remove Codex `CODEX_HOME` overlay generation and auth/config copying.
- [x] 5.2 Remove Claude generated plugin directory as the ACPA skill-loading mechanism.
- [x] 5.3 Remove user global/configspace skill copying into generated overlays.
- [x] 5.4 Start harness adapter processes with cwd set to the resolved assistant workspace.
- [x] 5.5 Remove `runtime-cwd` usage for harness process cwd.
- [x] 5.6 Update launch profile tests for provider-specific managed instruction injection.

## 6. Cron Skill Migration

- [x] 6.1 Move cron natural-language protocol guidance into provider-specific `acpa-cron` skill content.
- [x] 6.2 Remove cron-management prompt-prefix protocol injection.
- [x] 6.3 Keep structured `acpa-cron` block parsing and owner/admin authorization unchanged.
- [x] 6.4 Update cron scheduler tests for built-in skill materialization and absence of prompt prefix.

## 7. Diagnostics And Console

- [x] 7.1 Update doctor to report assistant home, derived `.acpa`, derived workspace, and legacy layout warnings.
- [x] 7.2 Add doctor checks for managed skill markers, missing managed assets, and instruction source readability.
- [x] 7.3 Update status/logs output to prefer assistant home for new-layout assistants.
- [x] 7.4 Update daemon API payloads to include assistant home while preserving legacy path fields.
- [x] 7.5 Update Web console assistant creation to use assistant home as the primary advanced path.
- [x] 7.6 Update assistant cards and detail views to show assistant home or derived workspace clearly.

## 8. Verification And Migration

- [x] 8.1 Add unit tests for assistant home derivation and legacy compatibility resolution.
- [x] 8.2 Add unit tests for native skill materialization, marker overwrite rules, collision failures, and `.gitignore` updates.
- [x] 8.3 Add integration-style tests for Codex managed instructions through `developer_instructions`.
- [x] 8.4 Add runtime tests for Claude `_meta.systemPrompt` session metadata generation.
- [x] 8.5 Add tests proving `PromptPrefix` is no longer sent for managed instructions or cron execution.
- [x] 8.6 Run `go test ./...`.
- [x] 8.7 Run `openspec validate adopt-assistant-home-native-assets --strict`.
