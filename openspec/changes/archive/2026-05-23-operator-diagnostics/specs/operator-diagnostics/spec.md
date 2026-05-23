## ADDED Requirements

### Requirement: Doctor diagnostic report
The system SHALL provide an `acpa doctor` command that evaluates assistant health and returns a diagnostic report.

#### Scenario: Running doctor for an assistant
- **WHEN** an operator runs `acpa doctor` with an assistant id, root, or configspace
- **THEN** the system SHALL resolve the assistant configuration and run diagnostic checks for configuration, storage, process state, connectors, harness launchability, and recent errors

#### Scenario: Reporting overall health
- **WHEN** diagnostic checks complete
- **THEN** the system SHALL return an overall status of pass, warn, or fail
- **AND** it SHALL include the failed or warning checks that determined that status

### Requirement: Human-readable doctor output
The system SHALL render `acpa doctor` as concise human-readable text by default.

#### Scenario: Default output
- **WHEN** an operator runs `acpa doctor` without output flags
- **THEN** the system SHALL print the overall status, key check results, important errors, and suggested next actions

#### Scenario: Verbose output
- **WHEN** an operator runs `acpa doctor --verbose`
- **THEN** the system SHALL include detailed check data such as resolved paths, command paths, connector details, recent events, and relevant log snippets

### Requirement: JSON diagnostic output
The system SHALL support structured JSON output for diagnostics.

#### Scenario: JSON output requested
- **WHEN** an operator runs `acpa doctor --json`
- **THEN** the system SHALL print the diagnostic report as JSON
- **AND** the JSON SHALL include stable fields for assistant identity, checks, severity, messages, details, and recommended actions

### Requirement: Safe lightweight probes
The system SHALL limit default diagnostic probes to safe local checks.

#### Scenario: Default probes run
- **WHEN** `acpa doctor` runs default probes
- **THEN** the system MAY check command existence, filesystem permissions, event DB health, connector configuration, harness profile resolution, process PID state, and bounded version commands
- **AND** it SHALL NOT send IM messages, create real assistant sessions, prompt a harness, trigger authorization, or modify memory files

### Requirement: Status snapshot command
The system SHALL provide a status command that returns a current-state snapshot without diagnostic interpretation.

#### Scenario: Running status
- **WHEN** an operator runs `acpa status` or the equivalent assistant-scoped status command
- **THEN** the system SHALL print assistant identity, configured channels, connector states, active session count, pending permission count, and recent error count

### Requirement: Log viewing command
The system SHALL provide a logs command focused on event and log viewing.

#### Scenario: Showing recent logs
- **WHEN** an operator runs `acpa logs`
- **THEN** the system SHALL print recent assistant events or configured assistant log lines in chronological order

#### Scenario: Following logs
- **WHEN** an operator runs `acpa logs --follow`
- **THEN** the system SHALL continue printing new events or log lines until interrupted
