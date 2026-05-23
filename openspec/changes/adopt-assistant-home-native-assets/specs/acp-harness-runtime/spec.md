## ADDED Requirements

### Requirement: Harness cwd is assistant workspace
The ACP runtime SHALL start harness adapter processes with cwd set to the resolved assistant workspace.

#### Scenario: Start new-layout harness
- **WHEN** a new-layout assistant starts Codex or Claude Code
- **THEN** the ACP adapter process cwd MUST be `<assistant-home>/workspace`
- **AND** `session/new.cwd` MUST be the same workspace path

#### Scenario: Start legacy-layout harness
- **WHEN** a legacy-layout assistant starts through compatibility logic
- **THEN** the ACP adapter process cwd MUST be the assistant's persisted workspace path
- **AND** `session/new.cwd` MUST be the same persisted workspace path

### Requirement: Runtime carries managed instruction metadata
The ACP runtime SHALL support provider-specific managed instruction injection without using user prompt prefix messages.

#### Scenario: Codex runtime uses launch configuration
- **WHEN** the selected provider is Codex
- **THEN** managed instructions MUST be supplied through the launch profile configuration
- **AND** the first user prompt MUST NOT include an ACPA prompt-prefix message for managed instructions

#### Scenario: Claude runtime uses session metadata
- **WHEN** the selected provider is Claude Code
- **THEN** managed instructions MUST be supplied in the `session/new` request metadata
- **AND** the first user prompt MUST NOT include an ACPA prompt-prefix message for managed instructions
