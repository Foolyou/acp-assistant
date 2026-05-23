## Context

ACPA currently models each assistant with independent workspace and configspace paths. Harness launch then creates provider-specific overlays under configspace, sets `CODEX_HOME` for Codex, passes a generated Claude plugin dir, and sends prompt-prefix text once per ACP session. That made sense while ACPA owned a synthetic harness view, but it now conflicts with the desired native model: Codex should discover `.agents/skills` from the workspace, Claude should discover `.claude/skills`, and both should load workspace instructions through native files.

Recent verification established three constraints:

- Codex discovers workspace `.agents/skills/<skill>/SKILL.md` and `.codex/skills/<skill>/SKILL.md`, but not nested hidden directories such as `.agents/skills/.acpa/<skill>`.
- Codex ACP accepts `session/new._meta.systemPrompt` but does not make that content visible to the model in practice; Codex `developer_instructions` config override does work end to end.
- Claude Agent ACP reads `session/new._meta.systemPrompt` and passes it into Claude Agent SDK.

## Goals / Non-Goals

**Goals:**
- Make assistant home the primary operator path and derive `.acpa` configspace plus `workspace` from it.
- Start Codex and Claude ACP processes with cwd set to the derived workspace.
- Materialize only ACPA-managed built-in skills into harness-native workspace skill roots.
- Replace `Instructions.md` and prompt prefix with managed instructions in `.acpa/instructions` plus provider-specific injection.
- Preserve shared project guidance in `AGENTS.md`, with Claude bridged through `CLAUDE.md`.
- Keep legacy path flags/loaders usable long enough to inspect, migrate, or run existing assistants.

**Non-Goals:**
- Do not implement a marketplace or user skill installer.
- Do not copy user skills between Codex and Claude roots.
- Do not make Claude read `.agents` or Codex read `.claude`.
- Do not store resolved connector secrets in `.acpa`.
- Do not change cron scheduler persistence or run-claim semantics except for how the natural-language management skill is exposed.

## Decisions

### Assistant Home Layout

New assistants use this fixed layout:

```text
<assistant-home>/
  .acpa/
    assistant.yaml
    channels/
    policies.yaml
    events.db
    instructions/
      common.md
      codex.md
      claude.md
    manifests/
  workspace/
    AGENTS.md
    CLAUDE.md
    .agents/skills/acpa-*/
    .claude/skills/acpa-*/
    memory/
```

The CLI and daemon should treat `--home` as the primary assistant selector. `ConfigspacePath` and `WorkspacePath` can remain as internal derived fields during migration, but new configs should either persist `home_path` or persist paths that validate as `<home>/.acpa` and `<home>/workspace`.

Alternative considered: make `<assistant-home>` itself the harness cwd and store config in `<assistant-home>/.acpa`. That would make native discovery simplest, but it also places ACPA state in the harness project root. The derived `workspace` subdirectory keeps config out of project discovery and preserves a clean harness root.

### Native Skill Materialization

ACPA-managed built-in skills are generated directly into provider-native workspace roots:

- Codex: `<workspace>/.agents/skills/acpa-*`
- Claude: `<workspace>/.claude/skills/acpa-*`

Each managed skill directory includes a marker such as `.acpa-managed.json` with asset id, provider, source version, and content hash. Startup/repair may overwrite only directories with a valid ACPA marker. If an `acpa-*` directory exists without a valid marker, startup should fail with a conflict diagnostic rather than overwrite user content.

ACPA should automatically add these ignore rules to the workspace `.gitignore`:

```gitignore
.agents/skills/acpa-*/
.claude/skills/acpa-*/
```

Alternative considered: keep generated skills under `.agents/skills/.acpa/`. Codex does not discover nested hidden skill directories, so direct child directories are required.

### Managed Instructions

ACPA-managed instructions live in configspace:

- `<assistant-home>/.acpa/instructions/common.md`
- `<assistant-home>/.acpa/instructions/codex.md`
- `<assistant-home>/.acpa/instructions/claude.md`

At runtime, ACPA renders `common + provider-specific` into a `ManagedInstructions` string.

Provider injection remains implementation-specific:

- Codex launch args include `-c developer_instructions="<managed instructions>"`.
- Claude session creation includes `_meta.systemPrompt` append content for the rendered instructions.

Workspace guidance remains native:

- `AGENTS.md` is the canonical shared project instruction file.
- `CLAUDE.md` is a bridge to `AGENTS.md`, not a separate source of shared truth.
- Managed instructions explicitly tell harnesses to update `AGENTS.md` for shared project guidance, including Claude sessions.

Alternative considered: use ACP `_meta.systemPrompt` for both providers. End-to-end Codex ACP verification showed `_meta.systemPrompt` did not affect model-visible behavior, while `developer_instructions` did.

### Prompt Prefix Removal

The existing prompt prefix should be removed as a general mechanism. It currently mixes durable ACPA behavior, cron protocol text, and first-turn prompt mutation. Built-in behavior should instead be either:

- a managed instruction injected at launch/session creation, or
- a provider-native built-in skill materialized in the workspace.

Cron management moves to the `acpa-cron` built-in skill, with provider-specific skill bodies as needed.

## Risks / Trade-offs

- [Risk] Existing assistants use arbitrary workspace/configspace paths. -> Keep compatibility resolution for `--configspace`, registry entries, and old assistant YAML while adding a migration/repair path to the new assistant home layout.
- [Risk] Auto-writing `.gitignore` may modify user workspace files. -> Append only missing ACPA-managed ignore lines, preserve existing content, and report the change.
- [Risk] `acpa-*` user directories may collide with managed names. -> Require marker validation before overwrite and surface a clear conflict with repair instructions.
- [Risk] Provider instruction injection has different precedence and transport behavior. -> Hide this behind `ManagedInstructions` and verify each provider with focused tests.
- [Risk] Claude users may put shared guidance in `CLAUDE.md`. -> Keep `CLAUDE.md` as a bridge and include managed instructions telling Claude to edit `AGENTS.md` for shared guidance.
- [Risk] Removing prompt prefix changes first-turn behavior. -> Move all required protocol text into skills or managed instructions before deleting prefix plumbing.

## Migration Plan

1. Add assistant home path resolution and derived `.acpa`/`workspace` helpers while retaining legacy configspace/workspace loading.
2. Update create flows to write new assistant home layout and registry entries keyed by home.
3. Implement native materialization for ACPA built-in skills and managed `.gitignore` updates.
4. Implement managed instruction rendering and provider-specific injection.
5. Move cron prompt-prefix protocol into provider-specific `acpa-cron` skill bodies.
6. Remove generated `CODEX_HOME`, Claude plugin, user skill copy, `Instructions.md`, `runtime-cwd`, and prompt prefix behavior after tests cover the new path.
7. Add doctor checks for layout validity, materialized assets, instruction injection config, and legacy layout warnings.

Rollback during implementation is possible by retaining the legacy overlay path behind compatibility code until all new tests pass.

## Open Questions

- Should old assistants be automatically migrated on first start, or should migration require an explicit repair command?
- Should `.gitignore` auto-write be skipped when the workspace is not a Git repository, or should ACPA still create the file in anticipation of future Git use?
