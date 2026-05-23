## 1. Assistant Home Layout

- [ ] 1.1 Add assistant home path helpers that derive `<home>/.acpa` and `<home>/workspace`.
- [ ] 1.2 Update assistant config model and persistence to support new-layout assistant home while retaining legacy workspace/configspace fields.
- [ ] 1.3 Update assistant creation defaults to create `<home>/.acpa` and `<home>/workspace`.
- [ ] 1.4 Add CLI `--home` resolution for assistant-scoped commands.
- [ ] 1.5 Preserve compatibility resolution for legacy `--root`, `--configspace`, `--workspace`, and registry entries.
- [ ] 1.6 Update registry entries to record assistant home for new assistants.

## 2. Native Managed Skills

- [ ] 2.1 Define bundled ACPA built-in skill sources by provider, including `acpa-cron`.
- [ ] 2.2 Implement Codex materialization into `<workspace>/.agents/skills/acpa-*`.
- [ ] 2.3 Implement Claude materialization into `<workspace>/.claude/skills/acpa-*`.
- [ ] 2.4 Add `.acpa-managed.json` ownership marker generation and validation.
- [ ] 2.5 Fail safely on unowned `acpa-*` directory collisions.
- [ ] 2.6 Automatically append missing workspace `.gitignore` rules for `.agents/skills/acpa-*/` and `.claude/skills/acpa-*/`.

## 3. Managed Instructions

- [ ] 3.1 Initialize `.acpa/instructions/common.md`, `.acpa/instructions/codex.md`, and `.acpa/instructions/claude.md`.
- [ ] 3.2 Render provider managed instructions from common plus provider-specific files.
- [ ] 3.3 Inject Codex managed instructions through `developer_instructions` launch config override.
- [ ] 3.4 Inject Claude managed instructions through `session/new._meta.systemPrompt` append metadata.
- [ ] 3.5 Remove first-turn prompt-prefix usage for managed instructions.
- [ ] 3.6 Ensure managed instructions direct all harnesses to update `AGENTS.md` for shared project guidance.

## 4. Workspace Instructions

- [ ] 4.1 Initialize or preserve workspace `AGENTS.md` for new assistants.
- [ ] 4.2 Initialize or repair `CLAUDE.md` as a bridge to `AGENTS.md` without treating it as shared guidance storage.
- [ ] 4.3 Remove `Instructions.md` creation and loading paths.
- [ ] 4.4 Update `/skills`, `/help`, and related operator text to reference native skill roots and `AGENTS.md`.

## 5. Harness Launch Cleanup

- [ ] 5.1 Remove Codex `CODEX_HOME` overlay generation and auth/config copying.
- [ ] 5.2 Remove Claude generated plugin directory as the ACPA skill-loading mechanism.
- [ ] 5.3 Remove user global/configspace skill copying into generated overlays.
- [ ] 5.4 Start harness adapter processes with cwd set to the resolved assistant workspace.
- [ ] 5.5 Remove `runtime-cwd` usage for harness process cwd.
- [ ] 5.6 Update launch profile tests for provider-specific managed instruction injection.

## 6. Cron Skill Migration

- [ ] 6.1 Move cron natural-language protocol guidance into provider-specific `acpa-cron` skill content.
- [ ] 6.2 Remove cron-management prompt-prefix protocol injection.
- [ ] 6.3 Keep structured `acpa-cron` block parsing and owner/admin authorization unchanged.
- [ ] 6.4 Update cron scheduler tests for built-in skill materialization and absence of prompt prefix.

## 7. Diagnostics And Console

- [ ] 7.1 Update doctor to report assistant home, derived `.acpa`, derived workspace, and legacy layout warnings.
- [ ] 7.2 Add doctor checks for managed skill markers, missing managed assets, and instruction source readability.
- [ ] 7.3 Update status/logs output to prefer assistant home for new-layout assistants.
- [ ] 7.4 Update daemon API payloads to include assistant home while preserving legacy path fields.
- [ ] 7.5 Update Web console assistant creation to use assistant home as the primary advanced path.
- [ ] 7.6 Update assistant cards and detail views to show assistant home or derived workspace clearly.

## 8. Verification And Migration

- [ ] 8.1 Add unit tests for assistant home derivation and legacy compatibility resolution.
- [ ] 8.2 Add unit tests for native skill materialization, marker overwrite rules, collision failures, and `.gitignore` updates.
- [ ] 8.3 Add integration-style tests for Codex managed instructions through `developer_instructions`.
- [ ] 8.4 Add runtime tests for Claude `_meta.systemPrompt` session metadata generation.
- [ ] 8.5 Add tests proving `PromptPrefix` is no longer sent for managed instructions or cron execution.
- [ ] 8.6 Run `go test ./...`.
- [ ] 8.7 Run `openspec validate adopt-assistant-home-native-assets --strict`.
