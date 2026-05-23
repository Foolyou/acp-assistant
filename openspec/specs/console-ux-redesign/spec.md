# console-ux-redesign Specification

## Purpose
TBD - created by archiving change console-ux-redesign. Update Purpose after archive.
## Requirements
### Requirement: React single-file console build
The system SHALL implement the daemon Web console from React source and serve a single self-contained built HTML file at runtime.

#### Scenario: Serving console without Node runtime
- **WHEN** the daemon serves the Web console
- **THEN** it SHALL serve a self-contained HTML document with required CSS and JavaScript inlined
- **AND** it SHALL NOT require Node.js, a frontend dev server, external CDN assets, or a static asset directory at runtime

#### Scenario: Building console source
- **WHEN** a developer runs the console build command
- **THEN** the system SHALL build React console source into the single HTML artifact used by the daemon

### Requirement: Mobile-first dashboard
The Web console SHALL present a mobile-first dashboard as the first screen.

#### Scenario: Viewing on mobile width
- **WHEN** the console is opened on a narrow viewport
- **THEN** the first screen SHALL show a compact app bar, daemon health, assistant running summary, attention state when present, assistant cards, and a primary create action
- **AND** it SHALL NOT use a table as the primary assistant list

#### Scenario: Assistant card content
- **WHEN** assistants are available
- **THEN** each assistant card SHALL show assistant name, id, running state, harness provider, compact workspace path, autostart state, and contextual lifecycle actions

#### Scenario: Empty assistant state
- **WHEN** no assistants are configured
- **THEN** the dashboard SHALL show an empty state with a clear create-assistant action

### Requirement: Sheet-based mobile workflows
The Web console SHALL use sheets or modal panels for setup and detail workflows instead of placing all forms on the dashboard.

#### Scenario: Creating an assistant
- **WHEN** the user starts assistant creation from mobile
- **THEN** the console SHALL open a sheet containing the assistant creation form
- **AND** advanced fields SHALL be visually secondary to required fields

#### Scenario: Configuring Feishu
- **WHEN** the user opens Feishu setup
- **THEN** the console SHALL open a sheet with a choice between QR onboarding and manual existing-app credentials

#### Scenario: Confirming lifecycle actions
- **WHEN** the user triggers stop, restart, or another disruptive assistant action
- **THEN** the console SHALL present a confirmation sheet or modal that explains the action before submitting it

### Requirement: Feishu setup UX
The Web console SHALL provide clear Feishu setup flows for QR onboarding and manual app credentials.

#### Scenario: QR onboarding
- **WHEN** QR onboarding starts successfully
- **THEN** the console SHALL show the scan URL or code, user code when available, and registration progress until credentials are stored or the flow fails

#### Scenario: Manual existing app setup
- **WHEN** manual setup is selected
- **THEN** the console SHALL request assistant id, channel id, app id, app secret, and relevant domain options
- **AND** it SHALL report success or field-level/user-facing errors after submission

### Requirement: Desktop-compatible console
The Web console SHALL adapt the mobile dashboard to desktop without changing the primary product model.

#### Scenario: Viewing on desktop width
- **WHEN** the console is opened on a wide viewport
- **THEN** assistant cards SHALL remain the primary assistant representation
- **AND** the console MAY show a right-side detail panel or modal for the selected assistant

#### Scenario: Desktop information density
- **WHEN** desktop space is available
- **THEN** the console SHALL increase useful information density without relying on nested cards or marketing-style sections

### Requirement: Operational visual language
The Web console SHALL use a developer local tool and operations-console visual style.

#### Scenario: Visual hierarchy
- **WHEN** the dashboard is rendered
- **THEN** daemon health, assistant state, and required actions SHALL be visually more prominent than secondary configuration details

#### Scenario: Status colors
- **WHEN** running, stopped, warning, or error states are shown
- **THEN** the console SHALL use distinct status colors and text labels so color is not the only state indicator

#### Scenario: Avoiding decorative layout
- **WHEN** the console is rendered
- **THEN** it SHALL avoid marketing hero sections, decorative background blobs, oversized display typography, and one-note color palettes

### Requirement: Responsive quality
The Web console SHALL be usable across mobile and desktop viewports.

#### Scenario: Mobile overflow
- **WHEN** the console is rendered at common mobile widths
- **THEN** text, controls, and cards SHALL fit without horizontal scrolling or overlapping content

#### Scenario: Touch targets
- **WHEN** the console is rendered on mobile
- **THEN** primary actions, toggles, and sheet controls SHALL have touch-friendly dimensions and spacing

#### Scenario: Loading and error states
- **WHEN** daemon API requests are loading or fail
- **THEN** the console SHALL show loading, retry, and error states without leaving controls in an ambiguous state

### Requirement: Console accessibility basics
The Web console SHALL preserve basic keyboard and screen-reader usability.

#### Scenario: Keyboard navigation
- **WHEN** a user navigates the console with a keyboard
- **THEN** interactive controls SHALL be reachable in a logical order and show visible focus states

#### Scenario: Sheet accessibility
- **WHEN** a sheet or modal is open
- **THEN** it SHALL have an accessible label and provide a clear close or cancel action

### Requirement: UI verification
The implementation SHALL include verification for the redesigned console.

#### Scenario: Automated verification
- **WHEN** implementation is complete
- **THEN** automated tests SHALL verify the console artifact is built, served by the daemon, and includes the expected dashboard/setup entry points

#### Scenario: Browser smoke verification
- **WHEN** implementation is complete
- **THEN** the console SHALL be smoke-tested at mobile and desktop viewport widths to check for blank screens, horizontal overflow, and broken primary interactions

