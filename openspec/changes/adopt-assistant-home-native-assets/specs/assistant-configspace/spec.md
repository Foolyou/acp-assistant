## MODIFIED Requirements

### Requirement: Assistant has fixed workspace and configspace
The system SHALL model each new assistant with one fixed assistant home path and derive one fixed workspace path and one fixed configspace path from that home.

#### Scenario: Create assistant paths
- **WHEN** the user creates an assistant
- **THEN** the system MUST persist or derive the assistant home path in `assistant.yaml`
- **AND** it MUST derive the workspace path as `<assistant-home>/workspace`
- **AND** it MUST derive the configspace path as `<assistant-home>/.acpa`
- **AND** future starts of the assistant MUST use those derived paths unless the user explicitly migrates or reconfigures the assistant

#### Scenario: Load legacy assistant paths
- **WHEN** the system loads an assistant config that contains independent workspace and configspace paths
- **THEN** it MUST preserve those paths for compatibility
- **AND** it MUST surface that the assistant uses the legacy layout

### Requirement: Configspace stores durable configuration
The configspace SHALL store durable assistant configuration as human-readable YAML files under `<assistant-home>/.acpa` for new assistants.

#### Scenario: Initialize configspace layout
- **WHEN** an assistant is created
- **THEN** the system MUST create `<assistant-home>/.acpa/assistant.yaml`
- **AND** it MUST create directories for channel configs and secrets references under `<assistant-home>/.acpa`
- **AND** it MUST reserve a path for the assistant-local SQLite event database under `<assistant-home>/.acpa`
- **AND** it MUST create `<assistant-home>/.acpa/instructions/` for ACPA-managed instruction sources
- **AND** it MUST create `<assistant-home>/.acpa/manifests/` for managed asset manifests

#### Scenario: Load assistant configuration
- **WHEN** the assistant process starts
- **THEN** it MUST load assistant identity, harness binding, connector account configs, policies, and memory configuration from the resolved configspace
- **AND** for new-layout assistants the resolved configspace MUST be `<assistant-home>/.acpa`

### Requirement: Runtime state uses assistant-local SQLite
The configspace SHALL contain one assistant-local SQLite database for runtime state, events, and query indexes.

#### Scenario: Initialize event database
- **WHEN** an assistant is created
- **THEN** the system MUST initialize the SQLite database path in `<assistant-home>/.acpa`
- **AND** the database MUST be scoped to that assistant only

## ADDED Requirements

### Requirement: Configspace is outside harness cwd
For new-layout assistants, the ACPA configspace SHALL NOT be the harness cwd.

#### Scenario: Harness starts
- **WHEN** a new-layout assistant starts a Codex or Claude ACP harness
- **THEN** the harness process cwd MUST be `<assistant-home>/workspace`
- **AND** the ACPA configspace MUST remain at sibling path `<assistant-home>/.acpa`
