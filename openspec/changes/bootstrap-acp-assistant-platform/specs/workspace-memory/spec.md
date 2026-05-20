## ADDED Requirements

### Requirement: Workspace contains fixed memory files
The system SHALL maintain a fixed set of assistant memory files in each assistant workspace.

#### Scenario: Initialize workspace memory
- **WHEN** the user creates an assistant with an empty workspace
- **THEN** the system MUST create the configured memory file skeletons
- **AND** it MUST NOT overwrite existing memory files unless the user explicitly requests replacement

### Requirement: Users and harnesses can update memory through controlled paths
The system SHALL allow both users and harnesses to update workspace memory files through controlled update paths.

#### Scenario: User appends memory from IM command
- **WHEN** the user issues a supported memory update command from an IM channel
- **THEN** the runtime MUST apply the update to the target memory file
- **AND** it MUST record a memory revision event

#### Scenario: Harness updates memory
- **WHEN** the harness requests a memory update through an approved tool or command path
- **THEN** the runtime MUST validate the requested target file
- **AND** it MUST record the update as harness-originated

### Requirement: Memory updates are revisioned and recoverable
The system SHALL keep enough revision metadata to audit and roll back memory file changes.

#### Scenario: Roll back memory update
- **WHEN** the user requests rollback to a previous memory revision
- **THEN** the system MUST restore the target memory file to the selected revision
- **AND** it MUST record the rollback as a new memory revision event
