## MODIFIED Requirements

### Requirement: Assistant supervision
The daemon SHALL manage assistant start, stop, restart, and status operations using assistant home as the primary assistant path for new-layout assistants.

#### Scenario: Starting an assistant
- **WHEN** a client requests assistant start through the daemon
- **THEN** the daemon SHALL launch the assistant serve worker for that assistant home or compatible legacy configspace
- **AND** it SHALL track process id, running state, and last error

#### Scenario: Restarting an assistant
- **WHEN** a client requests assistant restart
- **THEN** the daemon SHALL stop the current assistant worker if running
- **AND** it SHALL start a new worker and report the resulting state

#### Scenario: Stopping an assistant
- **WHEN** a client requests assistant stop
- **THEN** the daemon SHALL stop the assistant worker
- **AND** it SHALL NOT disable the assistant's autostart setting unless the request explicitly asks to do so

### Requirement: Web assistant setup
The Web console SHALL provide an assistant setup flow based on assistant home for new assistants.

#### Scenario: Creating assistant from Web
- **WHEN** a user completes the Web assistant setup form
- **THEN** the system SHALL create assistant configuration with id, name, assistant home, derived workspace, derived configspace, harness provider, permission defaults, and autostart setting

#### Scenario: Default workspace and configspace
- **WHEN** the user does not provide an explicit assistant home path
- **THEN** the system SHALL create an assistant home under the default ACPA assistant root layout
- **AND** it SHALL derive `<assistant-home>/.acpa` and `<assistant-home>/workspace`

## ADDED Requirements

### Requirement: Assistant home API surface
Daemon and console APIs SHALL expose assistant home as the primary path for new-layout assistants while preserving legacy path fields for compatibility.

#### Scenario: Status API returns paths
- **WHEN** the Web console requests assistant status
- **THEN** the API response MUST include assistant home for new-layout assistants
- **AND** it MAY include derived workspace and configspace paths for display and diagnostics

#### Scenario: Legacy API response
- **WHEN** the Web console requests status for a legacy-layout assistant
- **THEN** the API response MUST include the legacy workspace and configspace paths
- **AND** it MUST indicate that no assistant home is available unless migration metadata exists
