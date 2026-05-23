## ADDED Requirements

### Requirement: Feishu permission prompts use interactive cards
The system SHALL send ACP permission requests to Feishu private-chat owners as interactive approval cards when the Feishu connector supports card messages.

#### Scenario: Send approval card for pending permission
- **WHEN** an ACP permission request is recorded for a Feishu private-user session
- **THEN** the assistant MUST send a Feishu card to the session owner's private chat
- **AND** the card MUST include approve and reject actions for the short approval id
- **AND** the outbound message MUST include plain text approval instructions as fallback content

#### Scenario: Fallback when card send fails
- **WHEN** the Feishu connector cannot send the interactive approval card
- **THEN** the connector MUST send the fallback text prompt to the same private chat target

### Requirement: Card actions resolve pending permissions
The system SHALL resolve a pending ACP permission when the Feishu session owner clicks an approval card action.

#### Scenario: Owner approves from card
- **WHEN** a Feishu card action callback selects the approve action for a pending permission
- **AND** the callback operator matches the pending permission owner
- **THEN** the assistant runtime MUST resolve the pending permission with the approved ACP option
- **AND** it MUST send the selected permission response to the ACP runtime

#### Scenario: Owner rejects from card
- **WHEN** a Feishu card action callback selects the reject action for a pending permission
- **AND** the callback operator matches the pending permission owner
- **THEN** the assistant runtime MUST resolve the pending permission with the rejected ACP option
- **AND** it MUST send the selected permission response to the ACP runtime

### Requirement: Card approval remains owner-only
The system SHALL reject Feishu card approval attempts from any user other than the pending permission owner.

#### Scenario: Non-owner clicks approval card
- **WHEN** a Feishu card action callback selects approve or reject for a pending permission
- **AND** the callback operator does not match the pending permission owner
- **THEN** the assistant runtime MUST reject the card action
- **AND** it MUST NOT send a permission response to the ACP runtime

### Requirement: Card approval is idempotent
The system SHALL handle duplicate, stale, or malformed Feishu card action callbacks without resolving a permission more than once.

#### Scenario: Duplicate card callback
- **WHEN** the same Feishu card callback event is received more than once
- **THEN** the assistant runtime MUST ignore the duplicate callback after the first handling
- **AND** it MUST NOT send a second permission response to the ACP runtime

#### Scenario: Already resolved permission
- **WHEN** a card action callback references a permission that is no longer pending
- **THEN** the assistant runtime MUST leave the permission unchanged
- **AND** it MUST NOT send another permission response to the ACP runtime

#### Scenario: Missing approval id
- **WHEN** a Feishu card action callback does not include a short approval id
- **THEN** the assistant runtime MUST reject the callback
- **AND** it MUST NOT send a permission response to the ACP runtime

### Requirement: Text approval remains available
The system SHALL continue to support text approval commands for pending permissions.

#### Scenario: Owner uses text fallback
- **WHEN** the session owner sends `approve <id>` or `reject <id>` for a pending permission
- **THEN** the assistant runtime MUST resolve the pending permission using the existing text command behavior
