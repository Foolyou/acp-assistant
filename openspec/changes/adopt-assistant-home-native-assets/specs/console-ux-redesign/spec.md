## MODIFIED Requirements

### Requirement: Mobile-first dashboard
The Web console SHALL present a mobile-first dashboard as the first screen.

#### Scenario: Viewing on mobile width
- **WHEN** the console is opened on a narrow viewport
- **THEN** the first screen SHALL show a compact app bar, daemon health, assistant running summary, attention state when present, assistant cards, and a primary create action
- **AND** it SHALL NOT use a table as the primary assistant list

#### Scenario: Assistant card content
- **WHEN** assistants are available
- **THEN** each assistant card SHALL show assistant name, id, running state, harness provider, compact assistant home or workspace path, autostart state, and contextual lifecycle actions

#### Scenario: Empty assistant state
- **WHEN** no assistants are configured
- **THEN** the dashboard SHALL show an empty state with a clear create-assistant action

### Requirement: Sheet-based mobile workflows
The Web console SHALL use sheets or modal panels for setup and detail workflows instead of placing all forms on the dashboard.

#### Scenario: Creating an assistant
- **WHEN** the user starts assistant creation from mobile
- **THEN** the console SHALL open a sheet containing the assistant creation form
- **AND** assistant home SHALL be the primary advanced path field for new assistants
- **AND** legacy workspace and configspace fields SHALL NOT be primary fields for new assistants

#### Scenario: Configuring Feishu
- **WHEN** the user opens Feishu setup
- **THEN** the console SHALL open a sheet with a choice between QR onboarding and manual existing-app credentials

#### Scenario: Confirming lifecycle actions
- **WHEN** the user triggers stop, restart, or another disruptive assistant action
- **THEN** the console SHALL present a confirmation sheet or modal that explains the action before submitting it
