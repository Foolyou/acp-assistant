## ADDED Requirements

### Requirement: Host cron protocol instructions
The system SHALL expose cron management guidance as host-owned instructions rather than as generated or workspace-visible skills.

#### Scenario: Cron protocol is available to harness prompts
- **WHEN** a harness session is prompted for the first time
- **THEN** the host cron protocol instructions are available in the prompt/instructions path

#### Scenario: Cron is not listed as a managed skill
- **WHEN** managed harness skills are materialized for Codex or Claude
- **THEN** the generated skill set does not include `acpa-cron` or `cron`

#### Scenario: Cron cannot be overridden by workspace skill collision
- **WHEN** a workspace or assistant skill has a cron-like name
- **THEN** the host still validates cron requests only through the canonical `cron` protocol and does not delegate execution to the skill
