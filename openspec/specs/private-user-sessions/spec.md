# private-user-sessions Specification

## Purpose
TBD - created by archiving change bootstrap-acp-assistant-platform. Update Purpose after archive.
## Requirements
### Requirement: Private messages route to user-scoped sessions
The system SHALL bind private IM messages to sessions keyed by assistant, platform, account, private channel, and platform user.

#### Scenario: First private message from user
- **WHEN** a private user sends the first normal text message to a connector account
- **THEN** the assistant runtime MUST create a local session for that user binding
- **AND** it MUST create or attach an ACP session for the configured harness

#### Scenario: Second user messages same account
- **WHEN** another platform user sends a private message to the same assistant connector account
- **THEN** the assistant runtime MUST create or use a different local active session
- **AND** it MUST NOT mix the two users' conversation contexts

### Requirement: Session binding reserves future routing keys
Session binding records SHALL reserve optional `conversation_key` and `thread_key` fields while first-version routing remains private-chat only.

#### Scenario: Create private-chat binding
- **WHEN** a private-chat binding is created
- **THEN** the system MUST persist assistant id, platform, account id, private channel id, platform user id, active session id, and optional empty conversation/thread keys

### Requirement: Private user can create and switch sessions
The assistant SHALL support text commands for the private message sender to create and switch their own sessions.

#### Scenario: User creates new session
- **WHEN** a private user sends `/new`
- **THEN** the assistant runtime MUST create a new local session for that same user binding
- **AND** future normal messages from that user MUST route to the new active session

#### Scenario: User switches session
- **WHEN** a private user sends a supported session switch command for one of their sessions
- **THEN** the assistant runtime MUST update only that user's active session binding

### Requirement: IM updates are aggregated for chat delivery
The assistant SHALL aggregate ACP updates into IM-appropriate messages instead of sending every token or internal event.

#### Scenario: Prompt starts
- **WHEN** a normal private message is dispatched to ACP
- **THEN** the assistant MUST send or update a concise acknowledgement according to connector capability

#### Scenario: Final assistant text is available
- **WHEN** the ACP prompt turn completes with assistant text
- **THEN** the assistant MUST send the final response to the owning private chat target

#### Scenario: Response exceeds platform limit
- **WHEN** an outbound response exceeds the connector's configured text chunk limit
- **THEN** the assistant MUST split or attach the response according to connector capability

