## ADDED Requirements

### Requirement: Assistant owns a lightweight event index
The system SHALL maintain a lightweight per-assistant SQLite event/index store.

#### Scenario: Initialize event index
- **WHEN** an assistant is created
- **THEN** the system MUST initialize an event/index database in the assistant configspace
- **AND** the database MUST be scoped to that assistant instance

### Requirement: Event index records operational events
The event index SHALL record assistant lifecycle events, channel events, session bindings, message summaries, memory revisions, harness lifecycle events, and runtime errors.

#### Scenario: Record message summary
- **WHEN** the runtime routes an inbound IM message to a session
- **THEN** it MUST record a message summary event
- **AND** it MUST avoid storing secrets or full message content unless configured explicitly

#### Scenario: Record runtime error
- **WHEN** an adapter or harness operation fails
- **THEN** the runtime MUST record the error category, related assistant, related channel or session when known, and timestamp

### Requirement: Event index supports future dashboard queries
The event index SHALL expose enough structured state to support a future dashboard without making the dashboard part of the first implementation.

#### Scenario: Query assistant status
- **WHEN** a management surface queries assistant state
- **THEN** it MUST be able to derive last known process status, channel status, active session bindings, recent errors, and recent memory revisions from configspace and the event index
