# im-command-feedback Specification

## Purpose
TBD - created by archiving change im-command-feedback. Update Purpose after archive.
## Requirements
### Requirement: Command outcome feedback
The system SHALL send an explicit response for every recognized IM command outcome.

#### Scenario: Command succeeds
- **WHEN** a user sends a supported command and the command succeeds
- **THEN** the system SHALL send a concise success response

#### Scenario: Command fails
- **WHEN** a supported command cannot be completed
- **THEN** the system SHALL send a failure response that explains the reason in user-facing language

#### Scenario: Command is unknown
- **WHEN** a user sends an unknown slash command
- **THEN** the system SHALL reply that the command is unknown
- **AND** it SHALL mention `/help` as the discovery entry point

### Requirement: Command permission tiers
The system SHALL distinguish ordinary private-user commands from owner/admin commands.

#### Scenario: Ordinary private user command
- **WHEN** an ordinary private user sends `/help`, `/session`, `/status`, or `/clear`
- **THEN** the system SHALL process the command for that user's current binding

#### Scenario: Owner-only command from unauthorized user
- **WHEN** a non-owner user sends an owner/admin command such as `/mode`, `/mode default`, sensitive diagnostics, or configuration commands
- **THEN** the system SHALL reject the command
- **AND** it SHALL reply that owner permission is required

### Requirement: Mode change feedback
The system SHALL provide concise behavior and risk feedback after permission mode changes.

#### Scenario: Switching to manual mode
- **WHEN** an authorized user switches the current session to `manual`
- **THEN** the system SHALL confirm the new mode
- **AND** it SHALL state that privileged actions will request authorization

#### Scenario: Switching to yolo mode
- **WHEN** an authorized user switches the current session to `yolo`
- **THEN** the system SHALL confirm the new mode
- **AND** it SHALL warn that subsequent actions may skip authorization and should be used only for trusted tasks

#### Scenario: Switching to full_auto mode
- **WHEN** an authorized user switches the current session to `full_auto`
- **THEN** the system SHALL confirm the new mode
- **AND** it SHALL summarize the expected automatic execution behavior

### Requirement: Status command in IM
The system SHALL provide a session-scoped `/status` command in IM.

#### Scenario: User requests status
- **WHEN** a user sends `/status`
- **THEN** the system SHALL reply with that user's active session id, permission mode, harness provider, connector state when available, and pending permission count for that user

### Requirement: Skill listing command
The system SHALL provide skill listing commands in IM.

#### Scenario: User requests skills
- **WHEN** an authorized user sends `/skills`
- **THEN** the system SHALL reply with the effective skill names and short descriptions known to ACPA

#### Scenario: User requests verbose skills
- **WHEN** an authorized owner/admin sends `/skills verbose`
- **THEN** the system SHALL reply with skill names grouped by source layer
- **AND** it SHALL include source paths or generated overlay paths useful for debugging

### Requirement: Help command
The system SHALL provide a `/help` command that lists available commands for the sender.

#### Scenario: User requests help
- **WHEN** a user sends `/help`
- **THEN** the system SHALL list commands available to that sender's permission tier
- **AND** it SHALL omit or mark commands that require owner/admin permission

