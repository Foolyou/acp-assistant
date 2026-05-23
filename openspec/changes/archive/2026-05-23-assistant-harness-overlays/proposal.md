## Why

Assistants need shared platform-level instructions and skills while still allowing each assistant to define its own behavior. Passing those files directly to Codex or Claude Code would either pollute the user's native home directories or depend on harness-specific defaults that are hard to reason about.

This change introduces assistant-scoped harness overlays so ACPA can compile global and assistant instructions/skills into the native loading shape expected by each harness while keeping each assistant isolated.

## What Changes

- Add ACPA-managed global instructions and skills under the ACPA home directory.
- Add assistant-scoped instructions and skills under each assistant configspace.
- Generate a per-assistant harness overlay from global and assistant sources.
- Launch Codex ACP with an assistant-specific `CODEX_HOME` so Codex loads only the generated overlay.
- Launch Claude Code ACP with an assistant-specific plugin directory so Claude Code loads the generated ACPA plugin.
- Preserve harness-native behavior inside the generated overlay instead of trying to replace each harness skill system.
- Do not implicitly inherit the user's default `~/.codex` or `~/.claude` skill/plugin state.

## Capabilities

### New Capabilities

- `harness-overlays`: Defines how ACPA composes global and assistant instructions/skills into isolated Codex and Claude Code launch overlays.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - assistant creation and configspace initialization
  - harness launch profile resolution
  - ACP runtime process environment
  - tests for assistant layout, profile args, and runtime launch behavior
- New generated files under assistant configspace:
  - `instructions.md`
  - `skills/`
  - `harness/codex-home/`
  - `harness/claude-plugin/`
- New generated files under ACPA home:
  - `global/instructions.md`
  - `global/skills/`
