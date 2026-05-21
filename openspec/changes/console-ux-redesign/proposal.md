## Why

The current local Web console is functionally useful but visually and structurally immature: it exposes a table and stacked forms instead of a mobile-first operations workflow. ACPA needs a professional local console experience that helps users understand daemon health, assistant state, and setup actions quickly on mobile while remaining efficient on desktop.

## What Changes

- Redesign the daemon console as a mobile-first dashboard with assistant cards and action sheets.
- Introduce a React-based console source project that builds to a single self-contained HTML file embedded by Go.
- Keep runtime deployment simple: no Node runtime, no static asset directory, and no external network assets.
- Add UX states for loading, empty data, daemon health, assistant lifecycle actions, Feishu setup, errors, and confirmations.
- Add responsive desktop behavior with a higher-density dashboard and assistant detail panel.
- Add visual and interaction acceptance criteria for mobile and desktop.

## Capabilities

### New Capabilities

- `console-ux-redesign`: Defines the professional mobile-first console UX, React single-file build model, dashboard/sheet interactions, desktop compatibility, and UI quality requirements.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - daemon console HTML embedding
  - new Web console source/build tooling
  - daemon console tests
  - development and build documentation
- Runtime behavior remains local-first and daemon-owned.
- Existing daemon API endpoints should be reused; new endpoints are allowed only for clearly missing status or doctor data.
