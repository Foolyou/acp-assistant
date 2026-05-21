## 1. Daemon Foundation

- [x] 1.1 Add daemon package with local endpoint, pidfile, readiness, and shutdown handling
- [x] 1.2 Add `acpa daemon start`, `stop`, `restart`, and `status` commands
- [x] 1.3 Add lazy daemon client startup for commands that require the control plane
- [x] 1.4 Add loopback default binding and `--insecure` non-local confirmation

## 2. Assistant Supervision

- [x] 2.1 Add `autostart` to assistant configuration with default true for new assistants
- [x] 2.2 Move assistant start/stop/restart logic behind a reusable supervisor API
- [x] 2.3 Have daemon launch and track assistant serve worker processes
- [x] 2.4 Start all `autostart=true` assistants when daemon becomes ready
- [x] 2.5 Add CLI actions to disable or enable assistant autostart

## 3. Local API

- [x] 3.1 Add daemon API endpoints for assistant list, create, start, stop, restart, and status
- [x] 3.2 Add daemon API endpoints for Feishu QR onboarding lifecycle
- [x] 3.3 Add daemon API endpoint for manual Feishu app credential setup
- [x] 3.4 Ensure API handlers reuse existing configspace and Feishu onboarding validation logic

## 4. Web Console

- [x] 4.1 Add embedded Web assets served by the daemon
- [x] 4.2 Add assistant list and lifecycle controls
- [x] 4.3 Add assistant creation/setup flow
- [x] 4.4 Add Feishu QR onboarding flow with registration status
- [x] 4.5 Add manual existing Feishu app setup flow

## 5. Verification

- [x] 5.1 Add tests for daemon start/stop/status and lazy startup
- [x] 5.2 Add tests for assistant supervision and autostart behavior
- [x] 5.3 Add tests for local bind safety and insecure confirmation
- [x] 5.4 Add tests for Web/API setup validation and Feishu setup persistence
- [x] 5.5 Run `go test ./...`
