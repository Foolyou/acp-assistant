## ADDED Requirements

### Requirement: Assistant-scoped cron jobs
The system SHALL persist cron jobs per assistant with schedule, prompt, creator binding, execution target, delivery mode, enabled state, next run time, and timestamps.

#### Scenario: Create a recurring job
- **WHEN** an owner creates a cron job with a valid fixed interval schedule and prompt
- **THEN** the system stores the job for that assistant with an enabled state and a computed UTC next run time

#### Scenario: Reject invalid schedule
- **WHEN** an owner creates a cron job with an unsupported schedule expression
- **THEN** the system rejects the job and does not persist it

### Requirement: Cron command authorization
The system SHALL restrict `/cron` management commands to owner/admin users.

#### Scenario: Owner manages cron
- **WHEN** an owner/admin user sends `/cron list`
- **THEN** the system returns that assistant's cron jobs

#### Scenario: Non-owner is denied
- **WHEN** a non-owner user sends `/cron add --every 1h --message "check status"`
- **THEN** the system rejects the command without creating a job

### Requirement: Harness cron skill
The system SHALL inject a built-in cron skill into managed harness overlays, include the cron tool protocol in the harness prompt prefix, and execute structured cron tool calls returned by the harness for owner/admin users.

#### Scenario: Harness creates cron job
- **WHEN** an owner asks for a reminder in natural language
- **AND** the harness returns a valid `acpa-cron` create block
- **THEN** the system creates the cron job and sends a confirmation instead of relaying the raw tool block

#### Scenario: Non-owner cron tool denied
- **WHEN** a non-owner message causes the harness to return an `acpa-cron` block
- **THEN** the system rejects the tool call without creating or mutating cron jobs

### Requirement: Due job claiming
The system SHALL atomically claim due enabled jobs before execution so a job run is started at most once per due time.

#### Scenario: Claim due job
- **WHEN** a job is enabled, not already running, and its next run time is at or before the scheduler time
- **THEN** the system creates a running cron run record and marks the job as running

#### Scenario: Skip already running job
- **WHEN** a job has an active running cron run
- **THEN** the system does not claim another run for the same job

### Requirement: Scheduled execution
The system SHALL execute claimed cron runs through the assistant's configured harness using either isolated or main session targeting.

#### Scenario: Isolated execution
- **WHEN** a due job targets `isolated`
- **THEN** the system executes the prompt in a cron-specific local session separate from the creator's active chat session

#### Scenario: Main session execution
- **WHEN** a due job targets `main`
- **THEN** the system executes the prompt through the creator binding's active session

### Requirement: Run history
The system SHALL record each cron run with status, timestamps, final text, error text, and related session identifiers.

#### Scenario: Successful run
- **WHEN** a claimed job completes with final text
- **THEN** the system records the run as succeeded with the final text and finish time

#### Scenario: Failed run
- **WHEN** a claimed job fails during session creation or prompting
- **THEN** the system records the run as failed with an error and finish time

### Requirement: Delivery behavior
The system SHALL deliver cron results according to the job delivery mode and failure rules.

#### Scenario: Origin delivery
- **WHEN** a job with origin delivery succeeds with non-empty final text
- **THEN** the system sends the final text to the creator's IM route

#### Scenario: Silent success
- **WHEN** a job succeeds with final text beginning with `[SILENT]`
- **THEN** the system records the run and suppresses successful delivery

#### Scenario: No delivery
- **WHEN** a job with none delivery succeeds
- **THEN** the system records the run without sending a success message

#### Scenario: Failure delivery
- **WHEN** a job fails and has an origin route
- **THEN** the system sends a failure notification to the origin route

### Requirement: Schedule advancement
The system SHALL advance or disable jobs after each claimed run finishes.

#### Scenario: One-time job completion
- **WHEN** an `at` job finishes
- **THEN** the system disables the job and clears its next run time

#### Scenario: Recurring job completion
- **WHEN** an `every` or `cron` job finishes
- **THEN** the system computes and stores the next run time in UTC

### Requirement: Manual controls
The system SHALL allow owner/admin users to list jobs, inspect run history, pause jobs, resume jobs, remove jobs, and manually run a job.

#### Scenario: Pause and resume job
- **WHEN** an owner pauses and then resumes a cron job
- **THEN** the system updates the enabled state and recomputes next run time on resume

#### Scenario: Manual run
- **WHEN** an owner manually runs an existing cron job
- **THEN** the system starts a cron run regardless of the stored next run time
