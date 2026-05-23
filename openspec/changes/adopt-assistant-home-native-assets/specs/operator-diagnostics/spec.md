## MODIFIED Requirements

### Requirement: Doctor diagnostic report
The system SHALL provide an `acpa doctor` command that evaluates assistant health and returns a diagnostic report.

#### Scenario: Running doctor for an assistant
- **WHEN** an operator runs `acpa doctor` with an assistant id, home, root, or configspace
- **THEN** the system SHALL resolve the assistant configuration and run diagnostic checks for configuration, storage, process state, connectors, harness launchability, managed native assets, managed instructions, and recent errors

#### Scenario: Reporting overall health
- **WHEN** diagnostic checks complete
- **THEN** the system SHALL return an overall status of pass, warn, or fail
- **AND** it SHALL include the failed or warning checks that determined that status

## ADDED Requirements

### Requirement: Layout diagnostics
The system SHALL diagnose assistant home layout and legacy layout compatibility.

#### Scenario: New layout is healthy
- **WHEN** doctor checks a new-layout assistant
- **THEN** it MUST verify that `<assistant-home>/.acpa` and `<assistant-home>/workspace` exist
- **AND** it MUST report both derived paths in verbose and JSON output

#### Scenario: Legacy layout warning
- **WHEN** doctor checks an assistant using independent workspace and configspace paths
- **THEN** it MUST warn that the assistant uses the legacy layout
- **AND** it MUST recommend migration or repair to an assistant home layout

### Requirement: Managed asset diagnostics
The system SHALL diagnose ACPA-managed native skill materialization and managed instruction sources.

#### Scenario: Managed assets present
- **WHEN** doctor checks an assistant with materialized ACPA skills
- **THEN** it MUST verify ownership markers for `acpa-*` skill directories that ACPA expects to own
- **AND** it MUST report missing or conflicting managed skills as warnings or failures

#### Scenario: Managed instructions present
- **WHEN** doctor checks an assistant
- **THEN** it MUST verify that provider-relevant managed instruction source files can be read
- **AND** it MUST report the provider injection mechanism that will be used
