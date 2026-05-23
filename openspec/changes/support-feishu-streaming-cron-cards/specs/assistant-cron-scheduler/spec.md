## ADDED Requirements

### Requirement: Cron titles are stable execution identity
The system SHALL preserve the Cron job name created by the model as the scheduled task title and SHALL only change it when a canonical update explicitly includes a new name.

#### Scenario: Create Cron with title
- **WHEN** an owner creates a Cron job through the canonical `cron` add action
- **THEN** the system MUST persist `job.name` as the Cron title

#### Scenario: Update Cron without name
- **WHEN** an owner updates a Cron job without `patch.name`
- **THEN** the system MUST preserve the existing Cron title

#### Scenario: Rename Cron explicitly
- **WHEN** an owner updates a Cron job with a non-empty `patch.name`
- **THEN** the system MUST store the new name as the Cron title

### Requirement: Cron stream delivery is identifiable
The system SHALL identify Cron-originated streamed Feishu cards with the stored Cron title and Cron id.

#### Scenario: Cron run starts
- **WHEN** a due or manual Cron run with origin delivery starts
- **THEN** the system MUST send an immediate Feishu card before prompting the harness when the origin connector supports streaming cards
- **AND** the card MUST display the stored Cron title
- **AND** the card footer MUST identify the message as a Cron reply with the Cron id

#### Scenario: Cron text streams into card
- **WHEN** the harness emits assistant text chunks for a Cron run
- **THEN** the system MUST stream those chunks into a Cron Feishu card
- **AND** the card MUST display the stored Cron title
- **AND** the card footer MUST identify the message as a Cron reply with the Cron id

#### Scenario: Cron boundary starts identified card
- **WHEN** a Cron run receives a tool, permission, non-text, or unknown ACP boundary after assistant text has started
- **THEN** the next assistant text chunk MUST open a new Feishu card
- **AND** the new card MUST display the same stored Cron title
- **AND** the new card footer MUST identify the message as a Cron reply with the same Cron id

#### Scenario: Non-Feishu Cron delivery fallback
- **WHEN** a Cron run origin connector does not support streaming cards
- **THEN** the system MUST preserve the existing final-text Cron delivery behavior
