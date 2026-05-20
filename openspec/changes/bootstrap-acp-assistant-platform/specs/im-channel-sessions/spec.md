## ADDED Requirements

### Requirement: Assistant supports multiple IM channels
The system SHALL allow one assistant instance to bind multiple IM channels, with each channel handled through an IM adapter.

#### Scenario: Receive message from enabled channel
- **WHEN** an enabled IM channel receives a user message
- **THEN** the channel adapter MUST normalize the message into a common inbound message format
- **AND** the assistant runtime MUST route it to that channel's active session

#### Scenario: Receive message from disabled channel
- **WHEN** a disabled IM channel receives a user message
- **THEN** the assistant runtime MUST ignore or reject the message according to the channel configuration
- **AND** it MUST record the decision in the event index

### Requirement: Channel has default long-lived session
The system SHALL create or bind one default long-lived session for each IM channel in the first version.

#### Scenario: First message on channel
- **WHEN** the first supported message arrives on a channel without an existing session binding
- **THEN** the runtime MUST create a default session for that channel
- **AND** the runtime MUST persist the channel-to-session binding

#### Scenario: User switches channel session
- **WHEN** the user issues an IM command to switch or create a session for a channel
- **THEN** the runtime MUST update the channel's active session binding
- **AND** future messages on that channel MUST route to the selected session

### Requirement: Session model reserves conversation and thread keys
The system SHALL include optional `conversation_key` and `thread_key` fields in session routing records even though first-version routing is channel-scoped.

#### Scenario: Create channel-scoped session
- **WHEN** the runtime creates a first-version default session
- **THEN** it MUST persist the channel identifier
- **AND** it MUST allow `conversation_key` and `thread_key` to be empty

#### Scenario: Future thread-scoped session data
- **WHEN** a channel adapter provides conversation or thread identifiers
- **THEN** the runtime MUST be able to persist those values without changing the session record schema
