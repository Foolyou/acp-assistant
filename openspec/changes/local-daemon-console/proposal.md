## Why

Running assistants from individual CLI processes makes lifecycle state difficult to reason about and leaves setup spread across commands and files. ACPA needs a local control plane that can configure assistants, keep them running, and expose a simple browser-based setup flow.

## What Changes

- Add a local daemon responsible for assistant lifecycle and process supervision.
- Add lazy daemon startup for CLI commands that need the control plane.
- Add explicit `acpa daemon start|stop|restart|status` commands.
- Make daemon startup automatically launch assistants with `autostart=true`; new assistants default to autostart.
- Embed a local Web console in the daemon.
- Add a Web setup flow for creating assistants and configuring Feishu through either QR onboarding or manual existing-app credentials.
- Bind locally by default and require `--insecure` plus console confirmation for non-local binding.

## Capabilities

### New Capabilities

- `local-daemon-console`: Defines the local daemon, lazy startup, assistant supervision, autostart behavior, local Web console, and Feishu setup flows.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - daemon process and local API
  - assistant lifecycle management
  - CLI daemon client behavior
  - assistant config persistence
  - Feishu onboarding integration
  - local Web UI assets and handlers
  - tests for daemon lifecycle, autostart, binding safety, and setup flows
- Future CLI and Web management features should use the daemon API instead of independently supervising assistant processes.
