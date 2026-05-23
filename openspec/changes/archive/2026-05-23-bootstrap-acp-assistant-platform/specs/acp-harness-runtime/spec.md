## ADDED Requirements

### Requirement: Harness integration is ACP-only
The system SHALL connect to Codex and Claude Code only through ACP stdio JSON-RPC adapter processes.

#### Scenario: Start Codex ACP runtime
- **WHEN** an assistant bound to Codex needs a harness runtime
- **THEN** the system MUST start the configured Codex ACP command
- **AND** it MUST communicate with the process over stdio JSON-RPC

#### Scenario: Start Claude Code ACP runtime
- **WHEN** an assistant bound to Claude Code needs a harness runtime
- **THEN** the system MUST start the configured Claude Code ACP adapter command
- **AND** it MUST communicate with the process over stdio JSON-RPC

#### Scenario: Non-ACP harness command is requested
- **WHEN** a harness binding does not describe an ACP adapter
- **THEN** the system MUST reject the binding as unsupported

### Requirement: ACP runtime follows acp-webui behavior
The ACP runtime SHALL reuse the proven behavior shape from `acp-webui` for process lifecycle, request/response correlation, capability parsing, sessions, prompt turns, cancellation, and incoming notifications.

#### Scenario: Initialize ACP runtime
- **WHEN** an ACP adapter process starts
- **THEN** the runtime MUST send `initialize`
- **AND** it MUST persist discovered agent info and capabilities

#### Scenario: Send prompt
- **WHEN** a private-user session sends a prompt to a live ACP session
- **THEN** the runtime MUST call `session/prompt`
- **AND** incoming `session/update` events MUST be emitted to the assistant runtime

#### Scenario: Cancel prompt
- **WHEN** the user requests cancellation for a running prompt and the adapter supports cancellation
- **THEN** the runtime MUST call `session/cancel`

### Requirement: Runtime manages multiple ACP sessions
The system SHALL allow one ACP runtime process to manage multiple ACP sessions when they share the same harness provider and launch profile.

#### Scenario: Second user uses same launch profile
- **WHEN** two private users send messages through sessions that use the same harness provider and launch profile
- **THEN** the system MUST reuse the same compatible ACP runtime process
- **AND** it MUST keep their ACP session ids mapped to distinct local sessions

### Requirement: Harness capabilities gate operations
The system SHALL reject operations that the bound ACP adapter does not support.

#### Scenario: Session load unsupported
- **WHEN** mode switching or recovery requires `session/load` but the adapter does not advertise load support
- **THEN** the system MUST avoid calling `session/load`
- **AND** it MUST fall back to creating a new ACP session when that is allowed

### Requirement: fs/read_text_file is workspace confined
The ACP client-side `fs/read_text_file` method SHALL only read files inside the owning assistant workspace.

#### Scenario: Harness reads workspace file
- **WHEN** the ACP adapter calls `fs/read_text_file` for a path inside the workspace
- **THEN** the runtime MUST return bounded text content from that file

#### Scenario: Harness reads outside workspace
- **WHEN** the ACP adapter calls `fs/read_text_file` for a path outside the workspace
- **THEN** the runtime MUST return an ACP error
- **AND** it MUST NOT return file contents
