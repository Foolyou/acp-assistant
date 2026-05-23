# owner-permission-resolution Specification

## Purpose
TBD - created by archiving change bootstrap-acp-assistant-platform. Update Purpose after archive.
## Requirements
### Requirement: ACP permission requests belong to session owner
The system SHALL bind each ACP permission request to the local session owner identity.

#### Scenario: Permission request arrives
- **WHEN** the ACP runtime receives `session/request_permission`
- **THEN** the assistant runtime MUST create a pending permission record
- **AND** the record MUST include local session id, owner platform, account id, private channel id, platform user id, ACP request id, options, and short approval id

### Requirement: Permission prompts are delivered to owner target
The assistant SHALL send permission prompts only to the private chat target owned by the session owner.

#### Scenario: Prompt owner for permission
- **WHEN** a permission request is recorded for a private-user session
- **THEN** the assistant MUST send a prompt to that user's private chat target
- **AND** the prompt MUST include commands for approving or rejecting the short approval id

### Requirement: Only owner resolves permission
The system SHALL allow only the session owner identity to approve or reject a pending ACP permission request.

#### Scenario: Owner approves
- **WHEN** the session owner sends `/approve <id>`
- **THEN** the assistant runtime MUST resolve the matching pending permission
- **AND** it MUST send the selected ACP permission response to the runtime

#### Scenario: Non-owner attempts approval
- **WHEN** a different user identity sends `/approve <id>` or `/reject <id>`
- **THEN** the system MUST reject the command
- **AND** it MUST NOT send a permission response to ACP

### Requirement: Pending permissions expire
Pending permission requests SHALL expire after a configured timeout.

#### Scenario: Permission times out
- **WHEN** a pending permission exceeds its timeout before owner approval or rejection
- **THEN** the system MUST cancel or reject the permission according to configured timeout behavior
- **AND** it MUST record the timeout in the event index

