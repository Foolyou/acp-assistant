# local-daemon-console Specification

## Purpose
TBD - created by archiving change local-daemon-console. Update Purpose after archive.
## Requirements
### Requirement: Local daemon lifecycle
The system SHALL provide a local daemon that supervises assistant processes.

#### Scenario: Starting the daemon manually
- **WHEN** a user runs `acpa daemon start`
- **THEN** the system SHALL start the daemon in the background
- **AND** it SHALL record daemon connection metadata such as pidfile and local endpoint

#### Scenario: Stopping the daemon manually
- **WHEN** a user runs `acpa daemon stop`
- **THEN** the system SHALL stop the daemon
- **AND** it SHALL stop or detach supervised assistant processes according to the daemon shutdown policy documented by the command output

#### Scenario: Checking daemon status
- **WHEN** a user runs `acpa daemon status`
- **THEN** the system SHALL report whether the daemon is reachable, its endpoint, and supervised assistant counts

### Requirement: Lazy daemon startup
The system SHALL lazily start the daemon for CLI commands that require daemon services.

#### Scenario: Daemon command requires control plane
- **WHEN** a user runs a command such as `acpa console` or assistant lifecycle command and no daemon is reachable
- **THEN** the CLI SHALL start the local daemon automatically
- **AND** it SHALL retry the original request after the daemon becomes ready

#### Scenario: Lazy startup fails
- **WHEN** the CLI cannot start or connect to the daemon
- **THEN** it SHALL print a clear error and a manual recovery command

### Requirement: Assistant supervision
The daemon SHALL manage assistant start, stop, restart, and status operations.

#### Scenario: Starting an assistant
- **WHEN** a client requests assistant start through the daemon
- **THEN** the daemon SHALL launch the assistant serve worker for that assistant configspace
- **AND** it SHALL track process id, running state, and last error

#### Scenario: Restarting an assistant
- **WHEN** a client requests assistant restart
- **THEN** the daemon SHALL stop the current assistant worker if running
- **AND** it SHALL start a new worker and report the resulting state

#### Scenario: Stopping an assistant
- **WHEN** a client requests assistant stop
- **THEN** the daemon SHALL stop the assistant worker
- **AND** it SHALL NOT disable the assistant's autostart setting unless the request explicitly asks to do so

### Requirement: Assistant autostart
The system SHALL support assistant autostart controlled by assistant configuration.

#### Scenario: Creating an assistant
- **WHEN** a new assistant is created through CLI or Web setup
- **THEN** its configuration SHALL default `autostart` to true

#### Scenario: Daemon starts
- **WHEN** the daemon starts
- **THEN** it SHALL automatically start configured assistants with `autostart=true`

#### Scenario: Disabling autostart
- **WHEN** a user disables autostart for an assistant
- **THEN** the daemon SHALL persist `autostart=false`
- **AND** future daemon starts SHALL NOT automatically start that assistant

### Requirement: Local Web console
The daemon SHALL serve a local Web console.

#### Scenario: Opening the console
- **WHEN** a user runs `acpa console`
- **THEN** the CLI SHALL ensure the daemon is running
- **AND** it SHALL print or open the daemon's local console URL

#### Scenario: Viewing lifecycle controls
- **WHEN** the user opens the Web console
- **THEN** the console SHALL show configured assistants and allow creating, starting, stopping, and restarting assistants

### Requirement: Local bind safety
The daemon SHALL bind locally by default and guard non-local binding.

#### Scenario: Default bind
- **WHEN** the daemon starts without bind flags
- **THEN** it SHALL listen only on a loopback address

#### Scenario: Non-local bind requested
- **WHEN** a user requests a non-loopback bind address
- **THEN** the daemon SHALL require an explicit `--insecure` flag
- **AND** it SHALL require an interactive console confirmation before binding

### Requirement: Web assistant setup
The Web console SHALL provide an assistant setup flow.

#### Scenario: Creating assistant from Web
- **WHEN** a user completes the Web assistant setup form
- **THEN** the system SHALL create assistant configuration with id, name, workspace, configspace, harness provider, permission defaults, and autostart setting

#### Scenario: Default workspace and configspace
- **WHEN** the user does not provide explicit workspace or configspace paths
- **THEN** the system SHALL create them under the default ACPA assistant root layout

### Requirement: Web Feishu setup
The Web console SHALL support Feishu setup through New Feishu Bot setup and manual existing-app credentials.

#### Scenario: New Feishu Bot setup
- **WHEN** a user selects New Feishu Bot in the Web console
- **THEN** the system SHALL start Feishu registration
- **AND** the Web console SHALL show the setup URL, user code when available, and registration status until credentials are stored or the flow fails

#### Scenario: Manual app setup
- **WHEN** a user enters existing Feishu app credentials manually
- **THEN** the system SHALL store the credentials as assistant configspace secrets
- **AND** it SHALL create the Feishu channel configuration using those secrets
