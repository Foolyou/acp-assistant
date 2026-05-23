# event-index Specification

## Purpose
TBD - created by archiving change bootstrap-acp-assistant-platform. Update Purpose after archive.
## Requirements
### Requirement: Event index records runtime facts
The assistant-local SQLite event index SHALL record lifecycle, connector, session, prompt, ACP, permission, memory, error, and idempotency events.

#### Scenario: Record inbound message
- **WHEN** a connector delivers a normalized private message
- **THEN** the system MUST record an inbound message summary and idempotency key
- **AND** it MUST avoid storing full message content unless configured explicitly

#### Scenario: Record connector status
- **WHEN** a connector account changes connection state
- **THEN** the system MUST record platform, account id, state, timestamp, and error message when present

#### Scenario: Record ACP event
- **WHEN** the ACP runtime starts, initializes, fails, sends a prompt, receives an update, or closes
- **THEN** the system MUST record the event with assistant id, harness provider, launch profile, session id when known, and timestamp

### Requirement: Event index supports status and logs commands
The event index SHALL provide enough structured state to power `acpa channel status`, `acpa assistant inspect`, and `acpa logs --follow`.

#### Scenario: Query status
- **WHEN** the CLI queries assistant status
- **THEN** it MUST be able to derive last known assistant status, connector account status, active sessions, pending permissions, recent errors, and recent memory revisions

#### Scenario: Follow logs
- **WHEN** the CLI follows logs
- **THEN** it MUST stream new event records in timestamp order

### Requirement: Event index supports future dashboard
The event index SHALL preserve structured projections for a future dashboard without making the dashboard part of the first implementation.

#### Scenario: Dashboard reads projections later
- **WHEN** a future dashboard reads the assistant configspace and event index
- **THEN** it MUST be able to discover configured channels, connector status, active user sessions, permission waits, recent failures, and memory revision history without parsing raw log text

