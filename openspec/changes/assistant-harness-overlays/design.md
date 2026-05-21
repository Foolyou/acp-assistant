## Context

ACPA currently starts ACP harnesses from a provider profile and sends prompts through the ACP runtime. Codex is launched through `codex-acp` with `-c` overrides for sandbox and approval mode. Claude Code is launched through the Claude ACP package with optional permission arguments. The runtime does not yet support provider-specific environment variables, and assistant creation initializes only assistant metadata, policies, memory files, and channel directories.

Codex and Claude Code each have their own native instructions and skill/plugin systems. ACPA should not replace those systems. It should compile ACPA-owned global and assistant-level sources into a shape each harness can load natively, and keep that compiled output isolated from the user's default `~/.codex` and `~/.claude` state.

## Goals / Non-Goals

**Goals:**

- Provide a stable place for global ACPA instructions and skills.
- Provide a stable place for each assistant's instructions and skills.
- Generate per-assistant harness overlays from those sources.
- Launch Codex ACP with an assistant-specific `CODEX_HOME`.
- Launch Claude Code ACP with an assistant-specific plugin directory.
- Keep overlays deterministic and safe to regenerate.
- Avoid implicit inheritance from the user's default harness home directories.

**Non-Goals:**

- Replace Codex or Claude Code native skill resolution.
- Implement a full skill package manager.
- Import existing user `~/.codex` or `~/.claude` plugins automatically.
- Guarantee identical skill semantics across harness providers.

## Decisions

### Generate overlays inside configspace

Generated harness files will live under the assistant configspace:

```text
config/
  instructions.md
  skills/
  harness/
    codex-home/
    claude-plugin/
```

The configspace is already assistant-scoped, durable, and excluded from workspace memory semantics. Keeping generated overlays there avoids polluting the workspace and keeps harness launch state with the assistant configuration.

Alternative considered: generate directly under `~/.acpa/global`. That would mix shared source files with per-assistant output and make it harder to isolate assistant-specific skills.

### Use ACPA home for global source files

Global source files will live under the ACPA home:

```text
~/.acpa/
  global/
    instructions.md
    skills/
```

The ACPA home already represents product-level state outside any single assistant. Empty skeleton files/directories are enough for the first implementation; later CLI commands can edit or import skills.

Alternative considered: put global files under every assistant root. That would duplicate global policy and make updates inconsistent across assistants.

### Compile rather than directly reference source directories

Before starting a harness, ACPA will sync global and assistant sources into provider-specific overlay paths. The first implementation can be simple and deterministic: remove ACPA-managed generated directories and recreate them from source directories.

This keeps source paths independent from provider layout. It also lets us adapt if Codex or Claude Code changes expected skill/plugin shapes.

Alternative considered: pass source directories directly to each harness. Codex does not expose a stable `--skill-dir` option, and Claude expects plugin structure rather than arbitrary skill folders.

### Isolate Codex with CODEX_HOME

Codex overlay generation will create:

```text
config/harness/codex-home/
  config.toml
  skills/
    acpa-global/
    acpa-assistant/
```

The runtime will launch Codex with `CODEX_HOME` set to that directory. This prevents implicit loading of the user's default `~/.codex` configuration, skills, and plugins.

The generated `config.toml` should be minimal and assistant-specific. Existing launch profile `-c` overrides remain the source of truth for permission mode.

### Load Claude Code through a generated plugin directory

Claude overlay generation will create:

```text
config/harness/claude-plugin/
  .claude-plugin/plugin.json
  skills/
    acpa-global/
    acpa-assistant/
```

The Claude launch profile will add `--plugin-dir <path>` so Claude Code loads the ACPA plugin for this assistant. The plugin directory is generated from ACPA sources and does not require modifying the user's default `~/.claude`.

### Add environment support to harness bindings

`HarnessBinding` and `LaunchProfile` will gain an `Env` map. The ACP runtime will merge those entries with the current process environment when starting the harness. This is needed for `CODEX_HOME` and is useful for future provider-specific launch state.

## Risks / Trade-offs

- [Risk] Codex or Claude Code skill/plugin directory formats may change. → Keep provider-specific generation in a small package/function and cover generated file paths with tests.
- [Risk] Setting `CODEX_HOME` hides user-level Codex auth/config that may be required to run. → Generate a minimal overlay and rely on Codex auth mechanisms that are not tied to `~/.codex`; if needed later, add explicit import/copy settings instead of implicit inheritance.
- [Risk] Removing and recreating generated directories could delete user-edited overlay files. → Treat `config/harness/*` as generated output and document that source edits belong in `global/` or assistant `config/skills`.
- [Risk] Claude ACP wrapper argument compatibility may vary. → Keep existing default args and append `--plugin-dir` only for Claude profiles.
