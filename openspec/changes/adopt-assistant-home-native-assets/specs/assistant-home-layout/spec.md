## ADDED Requirements

### Requirement: Assistant home derives configspace and workspace
The system SHALL model each new assistant with one assistant home path, deriving the ACPA configspace as `<assistant-home>/.acpa` and the harness workspace as `<assistant-home>/workspace`.

#### Scenario: Create assistant home layout
- **WHEN** the user creates an assistant with a home path
- **THEN** the system MUST create or validate `<assistant-home>/.acpa`
- **AND** it MUST create or validate `<assistant-home>/workspace`
- **AND** it MUST persist enough configuration to resolve both derived paths on future starts

#### Scenario: Start assistant from home
- **WHEN** the user starts an assistant by home path or assistant id
- **THEN** the system MUST resolve the assistant config from `<assistant-home>/.acpa`
- **AND** it MUST use `<assistant-home>/workspace` as the harness workspace

### Requirement: Assistant home is the primary operator path
The system SHALL expose assistant home as the primary assistant path in CLI, daemon, diagnostics, registry, and Web console surfaces.

#### Scenario: CLI command uses home
- **WHEN** an operator runs an assistant-scoped CLI command with `--home <path>`
- **THEN** the system MUST resolve the assistant using `<path>/.acpa`
- **AND** it MUST report `<path>/workspace` as the harness workspace

#### Scenario: Registry records home
- **WHEN** a new assistant is registered
- **THEN** the registry MUST record the assistant home path
- **AND** derived configspace and workspace paths MUST be computable from that home

### Requirement: Legacy path compatibility
The system SHALL retain compatibility resolution for assistants created with independent workspace and configspace paths until they are migrated or explicitly recreated.

#### Scenario: Legacy configspace command
- **WHEN** an operator runs a command with a legacy `--configspace <path>`
- **THEN** the system MUST load the legacy assistant configuration if valid
- **AND** it MUST report that the assistant is using the legacy layout

#### Scenario: Legacy assistant remains runnable
- **WHEN** a legacy assistant has valid persisted workspace and configspace paths
- **THEN** the system MUST be able to start it through compatibility logic
- **AND** it MUST NOT silently rewrite it into the new assistant home layout without an explicit migration or repair operation
