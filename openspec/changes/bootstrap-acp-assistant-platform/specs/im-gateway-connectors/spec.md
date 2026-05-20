## ADDED Requirements

### Requirement: Assistant process owns IM gateway connectors
The system SHALL run IM connector accounts inside the owning assistant process.

#### Scenario: Start assistant with connectors
- **WHEN** an assistant starts with enabled Feishu or QQ Bot connector accounts
- **THEN** the assistant process MUST start each enabled connector account
- **AND** it MUST report each connector account's runtime status independently

### Requirement: Connector accounts are isolated
Each connector account SHALL own isolated credentials, token cache, connection lifecycle, logs, inbound state, and outbound API client state.

#### Scenario: Multiple QQ Bot accounts
- **WHEN** one assistant configures two QQ Bot accounts
- **THEN** each account MUST maintain a separate WebSocket connection
- **AND** OpenIDs from one account MUST NOT be used for outbound messages through another account

#### Scenario: Multiple Feishu accounts
- **WHEN** one assistant configures two Feishu accounts
- **THEN** each account MUST maintain separate credentials, API client state, and connection status

### Requirement: Feishu uses WebSocket long connection
The Feishu connector SHALL receive inbound events through Feishu WebSocket long connection and send outbound messages through Feishu OpenAPI.

#### Scenario: Receive Feishu private message
- **WHEN** a Feishu WebSocket event contains a private message
- **THEN** the connector MUST normalize it into an inbound private message event
- **AND** the assistant runtime MUST receive platform, account id, private channel id, platform user id, message id, text, timestamp, and raw payload metadata

### Requirement: QQ Bot uses official WebSocket gateway
The QQ Bot connector SHALL receive inbound events through the official QQ Bot WebSocket gateway and send outbound messages through QQ Bot APIs.

#### Scenario: Receive QQ Bot C2C message
- **WHEN** the QQ Bot WebSocket gateway delivers a C2C private message
- **THEN** the connector MUST normalize it into an inbound private message event
- **AND** the assistant runtime MUST receive platform, account id, private channel id, platform user id, message id, text, timestamp, and raw payload metadata

### Requirement: First version rejects non-private IM scopes
The first version SHALL support Feishu private chat and QQ Bot C2C private chat only.

#### Scenario: Feishu group event arrives
- **WHEN** the Feishu connector receives a group, topic, or thread event
- **THEN** the connector MUST ignore or reject the event according to configuration
- **AND** it MUST record the decision in the event index

#### Scenario: QQ Bot group or guild event arrives
- **WHEN** the QQ Bot connector receives a group, guild, or channel event
- **THEN** the connector MUST ignore or reject the event according to configuration
- **AND** it MUST record the decision in the event index

### Requirement: Connector supports reconnect and idempotency
Connectors SHALL handle long-connection failures with retry backoff and inbound message idempotency.

#### Scenario: WebSocket disconnects
- **WHEN** a connector WebSocket disconnects unexpectedly
- **THEN** the connector MUST update account status
- **AND** it MUST retry connection with bounded backoff while the assistant is running

#### Scenario: Duplicate inbound event arrives
- **WHEN** a connector receives an event id or message id already processed for that account
- **THEN** the assistant runtime MUST avoid dispatching a duplicate prompt to ACP
