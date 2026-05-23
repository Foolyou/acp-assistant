# permission-mode-policy Specification

## Purpose
TBD - created by archiving change bootstrap-acp-assistant-platform. Update Purpose after archive.
## Requirements
### Requirement: Sessions have current permission mode
Each local session SHALL store its current permission mode and launch profile key.

#### Scenario: New session uses channel-user default
- **WHEN** a private-user binding creates a new session
- **THEN** the session MUST use that user's effective default permission mode for the connector account

#### Scenario: User changes current session mode
- **WHEN** the session owner sends `/mode yolo`, `/mode manual`, or `/mode full_auto`
- **THEN** the assistant runtime MUST attempt to change only the current session's permission mode

### Requirement: Channel-user defaults are configurable by command
The system SHALL support changing the default permission mode for future sessions for a platform/account/user binding.

#### Scenario: User sets default mode
- **WHEN** the user sends `/mode default yolo`, `/mode default manual`, or `/mode default full_auto`
- **THEN** the assistant runtime MUST update that user's default mode only if policy permits default-mode changes
- **AND** existing sessions MUST keep their current permission mode unless explicitly changed

### Requirement: Mode changes require policy approval
Every current-session or default-mode change SHALL be evaluated against effective assistant, connector account, and user policy.

#### Scenario: User requests disallowed mode
- **WHEN** a user requests a permission mode not included in their effective allowed modes
- **THEN** the system MUST reject the change
- **AND** it MUST leave the existing session or default mode unchanged

#### Scenario: User lacks default-mode permission
- **WHEN** a user requests `/mode default yolo` but `can_set_default_mode` is false
- **THEN** the system MUST reject the change
- **AND** it MUST leave the channel-user default mode unchanged

### Requirement: Mode changes require harness support
Every permission mode change SHALL be validated against the assistant's bound harness provider.

#### Scenario: Claude user requests full_auto
- **WHEN** an assistant bound to Claude Code receives `/mode full_auto`
- **THEN** the system MUST reject the request because Claude Code does not support `full_auto`

#### Scenario: Codex user requests full_auto
- **WHEN** an assistant bound to Codex receives `/mode full_auto` and policy allows it
- **THEN** the system MUST allow the session mode change

### Requirement: Mode switching preserves local session identity
Changing permission mode SHALL preserve the local session id while allowing the underlying ACP runtime/profile to change.

#### Scenario: ACP load supported
- **WHEN** a session switches to a mode that requires a different runtime profile and the adapter supports `session/load`
- **THEN** the system MUST attempt to load the external ACP session in the target runtime profile
- **AND** the local session id MUST remain unchanged

#### Scenario: ACP load unavailable
- **WHEN** a session switches to a mode that requires a different runtime profile and the adapter cannot load the existing ACP session
- **THEN** the system MUST create a new ACP session under the same local session
- **AND** it MUST record a profile-switch event

