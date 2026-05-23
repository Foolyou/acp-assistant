## ADDED Requirements

### Requirement: ACPA-managed native skill materialization
The system SHALL materialize ACPA-managed built-in skills into provider-native workspace skill roots before starting a harness.

#### Scenario: Materialize Codex built-in skills
- **WHEN** an assistant bound to Codex is prepared for launch
- **THEN** the system MUST ensure ACPA-managed built-in skills exist under `<workspace>/.agents/skills/acpa-*`
- **AND** each managed skill MUST be a direct child of `.agents/skills`

#### Scenario: Materialize Claude built-in skills
- **WHEN** an assistant bound to Claude Code is prepared for launch
- **THEN** the system MUST ensure ACPA-managed built-in skills exist under `<workspace>/.claude/skills/acpa-*`
- **AND** each managed skill MUST be a direct child of `.claude/skills`

### Requirement: Managed skill ownership markers
The system SHALL mark each materialized ACPA-managed skill directory with ownership metadata and SHALL overwrite only directories it owns.

#### Scenario: Update owned managed skill
- **WHEN** materialization finds an `acpa-*` skill directory with a valid ACPA ownership marker
- **THEN** the system MAY replace that directory with the bundled version for the current ACPA binary
- **AND** it MUST refresh the ownership marker with the current provider, asset version, and content hash

#### Scenario: Detect unowned collision
- **WHEN** materialization finds an `acpa-*` skill directory without a valid ACPA ownership marker
- **THEN** the system MUST fail harness preparation with a conflict diagnostic
- **AND** it MUST NOT overwrite the directory

### Requirement: Workspace ignore rules for managed assets
The system SHALL automatically add workspace ignore rules for ACPA-managed materialized skill directories.

#### Scenario: Add missing ignore rules
- **WHEN** built-in skill materialization runs for a workspace
- **THEN** the system MUST ensure `.gitignore` contains `.agents/skills/acpa-*/` and `.claude/skills/acpa-*/`
- **AND** it MUST preserve existing `.gitignore` content

### Requirement: Managed instruction sources
The system SHALL store ACPA-managed instruction sources under the assistant configspace and render them per provider.

#### Scenario: Render Codex managed instructions
- **WHEN** an assistant bound to Codex is prepared for launch
- **THEN** the system MUST render `<configspace>/instructions/common.md` and `<configspace>/instructions/codex.md` into one managed instruction string

#### Scenario: Render Claude managed instructions
- **WHEN** an assistant bound to Claude Code is prepared for launch
- **THEN** the system MUST render `<configspace>/instructions/common.md` and `<configspace>/instructions/claude.md` into one managed instruction string

### Requirement: Provider-specific managed instruction injection
The system SHALL inject rendered managed instructions using the provider mechanism verified for the selected ACP adapter.

#### Scenario: Inject Codex instructions
- **WHEN** the selected harness provider is Codex
- **THEN** the launch profile MUST include a Codex config override for `developer_instructions` containing the rendered managed instructions
- **AND** it MUST NOT rely on `session/new._meta.systemPrompt` for Codex managed instructions

#### Scenario: Inject Claude instructions
- **WHEN** the selected harness provider is Claude Code
- **THEN** the ACP session creation request MUST include `_meta.systemPrompt` append content containing the rendered managed instructions

### Requirement: Workspace-native project instructions
The system SHALL treat `AGENTS.md` in the harness workspace as the canonical shared project instruction file.

#### Scenario: Initialize workspace instructions
- **WHEN** a new assistant workspace is initialized
- **THEN** the system MUST ensure `AGENTS.md` exists or preserve the existing `AGENTS.md`
- **AND** it MUST ensure Claude can load shared project guidance through `CLAUDE.md`

#### Scenario: Guide harness edits
- **WHEN** managed instructions are rendered for any provider
- **THEN** they MUST tell the harness to update `AGENTS.md` for shared project guidance
- **AND** for Claude Code they MUST NOT instruct the harness to write shared guidance directly into `CLAUDE.md`

## REMOVED Requirements

### Requirement: Global instruction and skill sources
**Reason**: ACPA no longer merges user-editable global instruction and skill source directories into generated harness overlays.
**Migration**: Built-in ACPA assets are bundled and materialized into native workspace paths; user skills remain in harness-native user or workspace locations.

### Requirement: Assistant instruction and skill sources
**Reason**: Assistant-specific `instructions.md` and `skills/` under configspace are replaced by managed `.acpa/instructions` and native workspace skill roots.
**Migration**: Move shared project guidance to workspace `AGENTS.md`; install user skills into `.agents/skills` or `.claude/skills` as appropriate.

### Requirement: Per-assistant harness overlay generation
**Reason**: Generated overlay directories are replaced by native materialization and provider-specific managed instruction injection.
**Migration**: Generate only ACPA-managed native skill directories and instruction injection payloads.

### Requirement: Codex isolated home overlay
**Reason**: Setting `CODEX_HOME` hides normal Codex user state and fights native workspace discovery.
**Migration**: Launch Codex without an ACPA `CODEX_HOME` override and materialize built-in Codex skills under `<workspace>/.agents/skills`.

### Requirement: Claude plugin overlay
**Reason**: The Claude ACP adapter does not use CLI `--plugin-dir` in its normal SDK session path for ACPA skill loading.
**Migration**: Materialize built-in Claude skills under `<workspace>/.claude/skills` and inject managed instructions via `_meta.systemPrompt`.
