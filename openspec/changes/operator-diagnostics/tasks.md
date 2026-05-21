## 1. Diagnostic Model

- [ ] 1.1 Add structured diagnostic report and check result types
- [ ] 1.2 Add severity aggregation for pass, warn, and fail states
- [ ] 1.3 Add recommendation fields for actionable next steps

## 2. Check Collection

- [ ] 2.1 Implement assistant registry and configspace resolution checks
- [ ] 2.2 Implement filesystem and event DB health checks
- [ ] 2.3 Implement PID/process checks for assistant serve processes
- [ ] 2.4 Implement connector configuration and connector status checks
- [ ] 2.5 Implement harness profile, command lookup, environment, and cwd checks
- [ ] 2.6 Implement recent error and log snippet collection

## 3. CLI Commands

- [ ] 3.1 Add top-level `acpa doctor` command with assistant id, root, and configspace resolution
- [ ] 3.2 Add `--verbose` human output rendering
- [ ] 3.3 Add `--json` rendering from the structured report
- [ ] 3.4 Add or normalize top-level `acpa status` behavior for assistant snapshots
- [ ] 3.5 Extend `acpa logs` with bounded recent output and follow behavior where missing

## 4. Verification

- [ ] 4.1 Add unit tests for report aggregation and rendering
- [ ] 4.2 Add CLI tests for text, verbose, and JSON output
- [ ] 4.3 Add tests that default probes do not create sessions, send messages, or modify memory
- [ ] 4.4 Run `go test ./...`
