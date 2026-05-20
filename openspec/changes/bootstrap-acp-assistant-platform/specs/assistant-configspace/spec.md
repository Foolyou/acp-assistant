## ADDED Requirements

### Requirement: Assistant has fixed workspace and configspace
The system SHALL model each assistant with one fixed workspace path and one fixed configspace path.

#### Scenario: Create assistant paths
- **WHEN** the user creates an assistant
- **THEN** the system MUST persist the workspace and configspace paths in `assistant.yaml`
- **AND** future starts of the assistant MUST use those persisted paths unless the user explicitly reconfigures them

### Requirement: Configspace stores durable configuration
The configspace SHALL store durable assistant configuration as human-readable YAML files.

#### Scenario: Initialize configspace layout
- **WHEN** an assistant is created
- **THEN** the system MUST create `assistant.yaml`
- **AND** it MUST create directories for channel configs and secrets references
- **AND** it MUST reserve a path for the assistant-local SQLite event database

#### Scenario: Load assistant configuration
- **WHEN** the assistant process starts
- **THEN** it MUST load assistant identity, harness binding, connector account configs, policies, and memory configuration from configspace

### Requirement: Secrets are referenced rather than embedded by default
The system SHALL support environment-variable and file-backed secret references for connector credentials.

#### Scenario: Credential uses environment variable
- **WHEN** a channel config references a secret by environment variable name
- **THEN** the assistant process MUST read the secret from that environment variable at runtime
- **AND** it MUST NOT write the resolved secret back into the YAML config

#### Scenario: Credential uses file reference
- **WHEN** a channel config references a secret by file path
- **THEN** the assistant process MUST read the secret from that file at runtime
- **AND** it MUST report a configuration error if the file is missing or unreadable

### Requirement: Runtime state uses assistant-local SQLite
The configspace SHALL contain one assistant-local SQLite database for runtime state, events, and query indexes.

#### Scenario: Initialize event database
- **WHEN** an assistant is created
- **THEN** the system MUST initialize the SQLite database path in the assistant configspace
- **AND** the database MUST be scoped to that assistant only
