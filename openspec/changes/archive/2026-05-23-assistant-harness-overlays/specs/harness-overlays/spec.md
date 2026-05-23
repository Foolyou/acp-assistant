## ADDED Requirements

### Requirement: Global instruction and skill sources
The system SHALL maintain ACPA-owned global instruction and skill source locations outside any single assistant.

#### Scenario: Initializing global sources
- **WHEN** the system needs global ACPA instruction or skill sources
- **THEN** it SHALL ensure the ACPA home contains `global/instructions.md` and `global/skills/`

### Requirement: Assistant instruction and skill sources
The system SHALL maintain assistant-owned instruction and skill source locations in each assistant configspace.

#### Scenario: Creating an assistant
- **WHEN** an assistant is created
- **THEN** the assistant configspace SHALL contain `instructions.md` and `skills/`

### Requirement: Per-assistant harness overlay generation
The system SHALL generate harness-specific overlays from global and assistant instruction and skill sources before starting a harness.

#### Scenario: Generating overlays for an assistant
- **WHEN** an assistant harness is prepared for launch
- **THEN** the system SHALL create generated provider-specific files under the assistant configspace `harness/` directory

#### Scenario: Preserving source directories
- **WHEN** overlay generation runs
- **THEN** the system SHALL NOT modify global source files or assistant source files

### Requirement: Codex isolated home overlay
The system SHALL launch Codex ACP with an assistant-specific Codex home generated under the assistant configspace.

#### Scenario: Launching Codex ACP
- **WHEN** the selected harness provider is Codex
- **THEN** the launch environment SHALL include `CODEX_HOME` pointing at `configspace/harness/codex-home`
- **AND** the Codex overlay SHALL include generated `config.toml` and `skills/` entries for ACPA global and assistant skills

#### Scenario: User Codex home is not implicitly inherited
- **WHEN** Codex ACP is launched through ACPA
- **THEN** the launch environment SHALL NOT rely on the user's default `~/.codex` directory for ACPA-managed instructions or skills

### Requirement: Claude plugin overlay
The system SHALL launch Claude Code ACP with an assistant-specific generated plugin directory.

#### Scenario: Launching Claude Code ACP
- **WHEN** the selected harness provider is Claude Code
- **THEN** the launch arguments SHALL include `--plugin-dir` pointing at `configspace/harness/claude-plugin`
- **AND** the plugin overlay SHALL include `.claude-plugin/plugin.json` and generated skill entries for ACPA global and assistant skills

### Requirement: Harness launch environment
The system SHALL support provider profile environment variables when starting an ACP process.

#### Scenario: Starting a harness with environment entries
- **WHEN** a launch profile includes environment entries
- **THEN** the ACP runtime SHALL start the harness process with those entries merged into the inherited process environment
