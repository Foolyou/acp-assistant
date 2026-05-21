## 1. Daemon Foundation

- [ ] 1.1 Add daemon package with local endpoint, pidfile, readiness, and shutdown handling
- [ ] 1.2 Add `acpa daemon start`, `stop`, `restart`, and `status` commands
- [ ] 1.3 Add lazy daemon client startup for commands that require the control plane
- [ ] 1.4 Add loopback default binding and `--insecure` non-local confirmation

## 2. Assistant Supervision

- [ ] 2.1 Add `autostart` to assistant configuration with default true for new assistants
- [ ] 2.2 Move assistant start/stop/restart logic behind a reusable supervisor API
- [ ] 2.3 Have daemon launch and track assistant serve worker processes
- [ ] 2.4 Start all `autostart=true` assistants when daemon becomes ready
- [ ] 2.5 Add CLI actions to disable or enable assistant autostart

## 3. Local API

- [ ] 3.1 Add daemon API endpoints for assistant list, create, start, stop, restart, and status
- [ ] 3.2 Add daemon API endpoints for Feishu QR onboarding lifecycle
- [ ] 3.3 Add daemon API endpoint for manual Feishu app credential setup
- [ ] 3.4 Ensure API handlers reuse existing configspace and Feishu onboarding validation logic

## 4. Web Console

- [ ] 4.1 Add embedded Web assets served by the daemon
- [ ] 4.2 Add assistant list and lifecycle controls
- [ ] 4.3 Add assistant creation/setup flow
- [ ] 4.4 Add Feishu QR onboarding flow with registration status
- [ ] 4.5 Add manual existing Feishu app setup flow

## 5. Verification

- [ ] 5.1 Add tests for daemon start/stop/status and lazy startup
- [ ] 5.2 Add tests for assistant supervision and autostart behavior
- [ ] 5.3 Add tests for local bind safety and insecure confirmation
- [ ] 5.4 Add tests for Web/API setup validation and Feishu setup persistence
- [ ] 5.5 Run `go test ./...`
