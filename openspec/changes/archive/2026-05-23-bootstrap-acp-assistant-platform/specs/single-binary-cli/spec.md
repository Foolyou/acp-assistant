## ADDED Requirements

### Requirement: System is operated through one binary
The system SHALL provide one `acpa` binary for assistant creation, configuration, channel onboarding, runtime operation, status, and logs.

#### Scenario: User creates an assistant from the binary
- **WHEN** the user runs `acpa assistant create` with a name, workspace, configspace, and harness provider
- **THEN** the binary MUST initialize the assistant configspace and workspace
- **AND** it MUST NOT require another project-specific executable

#### Scenario: User starts an assistant from the binary
- **WHEN** the user runs `acpa assistant start <assistant-id>`
- **THEN** the binary MUST start that assistant process using its configspace
- **AND** the process MUST own that assistant's ACP runtime, IM connector accounts, memory manager, and event index

### Requirement: CLI supports IM connector onboarding
The system SHALL provide CLI onboarding commands for first-version Feishu and QQ Bot connector accounts.

#### Scenario: Add Feishu connector account
- **WHEN** the user runs `acpa channel add feishu`
- **THEN** the CLI MUST collect or reference Feishu app credentials
- **AND** it MUST write a Feishu connector account config under the assistant configspace

#### Scenario: Add QQ Bot connector account
- **WHEN** the user runs `acpa channel add qqbot`
- **THEN** the CLI MUST collect or reference QQ Bot app credentials
- **AND** it MUST write a QQ Bot connector account config under the assistant configspace

### Requirement: Onboarding shows links and QR codes
The CLI SHALL show setup links and terminal QR codes during channel onboarding when a setup, guide, or pairing URL is available.

#### Scenario: Platform setup URL is available
- **WHEN** onboarding has a URL that the user can open on desktop or mobile
- **THEN** the CLI MUST print the URL
- **AND** it MUST render a terminal QR code for the same URL

#### Scenario: QR onboarding is unavailable
- **WHEN** the selected platform or current environment cannot provide a usable setup or pairing URL
- **THEN** the CLI MUST continue with a manual credential fallback
- **AND** it MUST explain which credential values are required

### Requirement: CLI exposes operational diagnostics
The system SHALL provide status and log commands that work from configspace and event index data.

#### Scenario: User checks channel status
- **WHEN** the user runs `acpa channel status`
- **THEN** the CLI MUST show configured connector accounts, enabled state, last known runtime state, and recent connection errors

#### Scenario: User follows logs
- **WHEN** the user runs `acpa logs --follow`
- **THEN** the CLI MUST stream assistant, connector, ACP runtime, session, permission, and error events from the assistant event index
