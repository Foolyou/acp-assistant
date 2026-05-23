## ADDED Requirements

### Requirement: Canonical cron protocol
The system SHALL accept host cron management requests only through the canonical `cron` fenced JSON protocol using OpenClaw-style action, job, schedule, payload, session target, and delivery fields.

#### Scenario: Harness creates cron job with canonical schema
- **WHEN** an owner asks for a reminder in natural language
- **AND** the harness returns a valid `cron` fenced block with `action: "add"` and a canonical `job`
- **THEN** the system creates the cron job and sends a confirmation instead of relaying the raw tool block

#### Scenario: Reject legacy cron block
- **WHEN** a harness response contains an `acpa-cron` fenced block
- **THEN** the system treats it as ordinary harness output and does not create or mutate cron jobs

#### Scenario: Reject legacy job fields
- **WHEN** a `cron` fenced block uses legacy fields such as `schedule_type`, `schedule_expr`, or top-level `message`
- **THEN** the system rejects the request without creating or mutating cron jobs

#### Scenario: Update enabled state through patch
- **WHEN** an owner sends a canonical `cron` request with `action: "update"` and `patch.enabled: false`
- **THEN** the system pauses the target job and reports the updated state

### Requirement: Canonical cron command input
The system SHALL make `/cron` creation and mutation use the same canonical JSON action schema as harness cron management.

#### Scenario: Owner creates job through JSON command
- **WHEN** an owner/admin user sends `/cron {"action":"add","job":{...}}` with a valid canonical job object
- **THEN** the system creates the job for that assistant

#### Scenario: Reject legacy add flags
- **WHEN** an owner/admin user sends `/cron add --every 1h --message "check status"`
- **THEN** the system rejects the command without creating a job

## MODIFIED Requirements

### Requirement: Cron command authorization
The system SHALL restrict `/cron` management commands to owner/admin users.

#### Scenario: Owner manages cron
- **WHEN** an owner/admin user sends `/cron {"action":"list"}`
- **THEN** the system returns that assistant's cron jobs

#### Scenario: Non-owner is denied
- **WHEN** a non-owner user sends `/cron {"action":"add","job":{"name":"check","schedule":{"kind":"every","everyMs":3600000},"sessionTarget":"isolated","payload":{"kind":"agentTurn","message":"check status"},"delivery":{"mode":"announce","target":"origin"}}}`
- **THEN** the system rejects the command without creating a job

### Requirement: Manual controls
The system SHALL allow owner/admin users to list jobs, inspect run history, pause jobs, resume jobs, remove jobs, and manually run a job through canonical cron actions.

#### Scenario: Pause and resume job
- **WHEN** an owner disables and then enables a cron job using canonical `update` actions
- **THEN** the system updates the enabled state and recomputes next run time on enable

#### Scenario: Manual run
- **WHEN** an owner manually runs an existing cron job with canonical `run`
- **THEN** the system starts a cron run regardless of the stored next run time

## REMOVED Requirements

### Requirement: Harness cron skill
**Reason**: Cron is a privileged host scheduler protocol, not a domain skill. P0 moves the model-facing contract to host-managed instructions and a `cron` fenced JSON protocol.

**Migration**: Harnesses must return canonical `cron` fenced JSON blocks. The host no longer accepts or installs `acpa-cron` as a managed skill.
