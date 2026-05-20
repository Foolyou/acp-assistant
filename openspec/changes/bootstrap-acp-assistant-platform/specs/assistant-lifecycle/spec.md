## ADDED Requirements

### Requirement: Assistant instances are independent process units
The system SHALL model each assistant as an independently runnable local process with a unique identifier, display name, workspace path, configspace path, harness binding, and enabled IM channels.

#### Scenario: Create assistant instance metadata
- **WHEN** the user creates an assistant with a name, workspace path, configspace path, and harness binding
- **THEN** the system MUST persist assistant metadata in the assistant configspace
- **AND** the persisted metadata MUST include a stable assistant identifier

#### Scenario: Start one assistant without starting others
- **WHEN** the user starts one assistant instance
- **THEN** the system MUST start only that assistant process
- **AND** other registered assistant instances MUST remain stopped unless explicitly started

### Requirement: Assistant CLI exposes lifecycle operations
The system SHALL provide CLI operations to create, list, start, stop, inspect, and remove assistant instances.

#### Scenario: Inspect assistant state
- **WHEN** the user inspects an assistant instance
- **THEN** the CLI MUST report its configured workspace, configspace, harness binding, enabled channels, default sessions, and last known runtime status

#### Scenario: Remove assistant preserves user data unless forced
- **WHEN** the user removes an assistant without a force option
- **THEN** the system MUST unregister the assistant from local management
- **AND** the system MUST NOT delete the assistant workspace or configspace

### Requirement: Configspace stores assistant configuration
The system SHALL use configspace as the durable source for assistant configuration.

#### Scenario: Restart assistant from configspace
- **WHEN** an assistant process restarts
- **THEN** it MUST reconstruct its harness binding, IM channels, session bindings, and memory configuration from configspace
