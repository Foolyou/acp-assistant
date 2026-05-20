## ADDED Requirements

### Requirement: Assistant binds to exactly one harness
The system SHALL require every assistant instance to bind to exactly one harness adapter.

#### Scenario: Create assistant without harness
- **WHEN** the user attempts to create an assistant without a harness binding
- **THEN** the system MUST reject the creation request with an actionable error

#### Scenario: Change harness binding
- **WHEN** the user changes an assistant harness binding
- **THEN** the system MUST persist the new binding in configspace
- **AND** the system MUST require a restart before new messages use the changed binding

### Requirement: Harness adapters hide implementation-specific process details
The system SHALL define a harness adapter boundary that hides the start command, environment variables, ACP transport, capability detection, and shutdown behavior for each supported harness.

#### Scenario: Send message through harness adapter
- **WHEN** a session sends a user message to the assistant harness
- **THEN** the runtime MUST call the configured harness adapter
- **AND** channel routing code MUST NOT depend on harness-specific process details

### Requirement: Harness adapters declare capabilities
The system SHALL let harness adapters declare capabilities such as streaming output, tool invocation, memory file access, session reuse, and cancellation.

#### Scenario: Adapter lacks cancellation support
- **WHEN** a user requests cancellation for a session whose harness adapter lacks cancellation support
- **THEN** the system MUST report that cancellation is unsupported for that harness
- **AND** the assistant process MUST continue running unless stopped explicitly
