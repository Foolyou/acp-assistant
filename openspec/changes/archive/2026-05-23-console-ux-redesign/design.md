## Context

The daemon currently serves an embedded HTML string from `internal/daemon/console.go`. That page provides assistant lifecycle controls, assistant creation, manual Feishu setup, and QR setup, but the UI is a basic table plus forms. It is not mobile-first, does not guide the user through the local-operations workflow, and will become difficult to maintain as more interactions are added.

The console is a local control surface, not a public SaaS app. It should feel like a developer local tool with operational clarity: compact, direct, readable, and reliable.

## Goals / Non-Goals

**Goals:**

- Redesign the console around a mobile-first dashboard.
- Make the first screen answer: is the daemon healthy, which assistants are running, and what needs attention?
- Replace the table-first layout with assistant cards and contextual actions.
- Use bottom sheets on mobile for create/setup/doctor/confirmation workflows.
- Use a desktop detail panel or modal pattern for the same workflows on wider screens.
- Implement the UI with React for maintainability, while building to a single self-contained HTML file for Go embedding.
- Define concrete visual, responsive, accessibility, and verification expectations.

**Non-Goals:**

- Add login, permissions, or remote console hardening.
- Add deep session management, log browsing, memory editing, skill management, or permission history.
- Add a public marketing-style landing page.
- Add runtime dependence on Node.js or external CDN assets.
- Replace daemon lifecycle APIs without a functional reason.

## Decisions

### Mobile dashboard plus sheets

The mobile home screen will be a dashboard with daemon health, assistant summary, assistant cards, and one primary create action. Create assistant, Feishu setup, doctor details, and destructive confirmations open in bottom sheets. This keeps the first screen focused and keeps forms out of the default scroll path.

Alternative considered: a bottom tab app. Tabs are clean, but for this v1 they fragment a small set of operations and make setup feel more distant than necessary.

### Desktop keeps the dashboard model

Desktop will not revert to a table as the primary view. It will use a wider dashboard with denser cards and a right-side assistant detail panel where practical. Tables may appear only for dense detail data, not as the main assistant list.

Alternative considered: desktop table plus mobile cards. That creates two product models and tends to make the mobile version feel secondary.

### React source, single-file runtime artifact

Console source will live in a Web source directory and use React components. The build will inline CSS and JavaScript into one HTML artifact, which Go embeds or imports as the console payload. The daemon must serve the built single file without requiring Node at runtime.

Alternative considered: keep editing a Go string. That avoids tooling but makes component state, sheet flows, and responsive polish harder to maintain.

### Developer local tool visual language

The visual design will combine developer-tool restraint with operations-console clarity. It should use neutral surfaces, clear status colors, compact typography, strong hierarchy, and minimal ornament. It must avoid hero sections, marketing composition, oversized decorative cards, and one-note palettes.

### Build accessible controls from native patterns

The UI will use native buttons, inputs, selects, checkboxes/toggles, and dialogs/sheets with clear focus management. Icons may be used for compact actions, but text labels must remain available for commands where ambiguity is possible.

## UX Model

### Mobile Home

1. Compact app bar with `ACPA Console`, daemon health pill, and refresh action.
2. Health strip showing daemon status, running assistant count, and last refresh.
3. Attention area for failed assistants, pending setup, or recent daemon/API errors.
4. Assistant card list:
   - name and id
   - running/stopped/failed state
   - harness provider
   - compact workspace path
   - autostart toggle
   - primary action based on current state
   - overflow actions for stop, restart, setup Feishu, and doctor
5. Floating or fixed primary create action.

### Mobile Sheets

- Create assistant sheet: required fields first, advanced fields collapsed.
- Feishu setup sheet: segmented control for QR onboarding vs manual app credentials.
- Doctor sheet: summary at top, expandable check details.
- Confirmation sheet: stop/restart/destructive actions with clear consequences.

### Desktop

- App shell constrained to a useful max width.
- Dashboard cards remain the primary assistant representation.
- A right-side panel shows selected assistant detail and actions.
- Create/setup/doctor may use side panel or modal presentation depending on content length.

## Data Flow

The React app will use the existing daemon HTTP API:

- `GET /api/status` for daemon and assistant state.
- `POST /api/assistants` for assistant creation.
- `POST /api/assistants/{id}/start|stop|restart|autostart` for lifecycle actions.
- `POST /api/setup/feishu/manual` for manual setup.
- `POST /api/setup/feishu/qr/begin` and `POST /api/setup/feishu/qr/complete` for QR onboarding.

If doctor data is needed for the sheet and no API exists yet, the implementation may add a daemon API that returns the existing structured diagnostic report.

## Risks / Trade-offs

- [Risk] React tooling adds build complexity. -> Keep runtime output to one HTML file and add simple build/test commands.
- [Risk] Single-file inlining can grow large. -> Accept this for local console v1; avoid large assets and external fonts.
- [Risk] Bottom sheet interactions can be fragile on mobile. -> Use simple state transitions, native focusable controls, and viewport tests.
- [Risk] Styling can drift into decorative SaaS patterns. -> Keep acceptance criteria focused on operational density, readable status, and first-screen utility.
- [Risk] Existing daemon API may not expose enough diagnostic data. -> Add only narrow API endpoints backed by existing diagnostics, not broad new management features.

## Verification Strategy

- Unit or component tests for state transitions where the chosen toolchain supports them.
- Go tests confirming the daemon serves the built console file and no longer depends on a Go string UI.
- Browser smoke checks at mobile and desktop widths after implementation.
- Manual acceptance using the local daemon: load console, inspect mobile layout, create/open sheets, and confirm no horizontal overflow.

## Open Questions

None for this spec. Exact colors and component names can be chosen during implementation as long as they satisfy the design requirements.
