## Why

When an assistant stops replying, operators currently have to inspect config files, logs, process state, connector state, and harness behavior manually. ACPA needs a first-class diagnostic surface that explains whether an assistant is healthy and what action should be taken next.

## What Changes

- Add an operator-facing `acpa doctor` command as the primary troubleshooting entry point.
- Keep `status` focused on current state snapshots and `logs` focused on log viewing.
- Make diagnostics available as both human-readable output and structured JSON.
- Run safe lightweight probes by default, without sending user-visible messages or creating real sessions.
- Provide verbose output for complete diagnostic details.

## Capabilities

### New Capabilities

- `operator-diagnostics`: Defines CLI diagnostics, status snapshots, log access, structured diagnostic reports, and safe probe boundaries.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - CLI command routing and output rendering
  - assistant/configspace loading
  - store health checks and recent event access
  - connector status inspection
  - harness profile and command probing
  - tests for CLI diagnostics and structured reports
- No new external service dependency is required.
