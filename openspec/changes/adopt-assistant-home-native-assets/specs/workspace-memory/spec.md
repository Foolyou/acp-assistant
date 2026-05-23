## MODIFIED Requirements

### Requirement: Workspace contains fixed memory files
The system SHALL maintain a configured fixed set of assistant memory files in each assistant workspace.

#### Scenario: Initialize workspace memory
- **WHEN** the user creates a new-layout assistant with an empty derived workspace
- **THEN** the system MUST create the configured memory file skeletons under `<assistant-home>/workspace`
- **AND** it MUST NOT overwrite existing memory files unless the user explicitly requests replacement

#### Scenario: Initialize legacy workspace memory
- **WHEN** the user creates or starts a legacy-layout assistant
- **THEN** the system MUST continue using the assistant's persisted workspace path for memory files
