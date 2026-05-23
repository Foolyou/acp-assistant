## MODIFIED Requirements

### Requirement: Harness cron skill
The system SHALL expose cron natural-language management through an ACPA-managed built-in native skill and SHALL execute structured cron tool calls returned by the harness for owner/admin users.

#### Scenario: Materialize cron skill
- **WHEN** harness assets are prepared for an assistant
- **THEN** the system MUST materialize the provider-specific `acpa-cron` skill into the selected provider's native workspace skill root
- **AND** cron management protocol instructions MUST be contained in that skill or managed instructions rather than a first-turn prompt prefix

#### Scenario: Harness creates cron job
- **WHEN** an owner asks for a reminder in natural language
- **AND** the harness returns a valid `acpa-cron` create block
- **THEN** the system creates the cron job and sends a confirmation instead of relaying the raw tool block

#### Scenario: Non-owner cron tool denied
- **WHEN** a non-owner message causes the harness to return an `acpa-cron` block
- **THEN** the system rejects the tool call without creating or mutating cron jobs

### Requirement: Scheduled execution
The system SHALL execute claimed cron runs through an isolated harness session or a main harness session according to their target.

#### Scenario: Isolated execution
- **WHEN** a due job targets `isolated`
- **THEN** the system executes the prompt in a cron-specific local session separate from the creator's active chat session

#### Scenario: Main session execution
- **WHEN** a due job targets `main`
- **THEN** the system executes the prompt through the creator binding's active session

#### Scenario: Management prefix is absent
- **WHEN** a due job is prompted through the harness
- **THEN** the runtime MUST NOT prepend a cron-management prompt prefix
- **AND** cron behavior MUST rely on the already materialized built-in skill and managed instructions
