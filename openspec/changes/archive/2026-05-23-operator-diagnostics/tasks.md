## 1. Diagnostic Model

- [x] 1.1 Add structured diagnostic report and check result types
- [x] 1.2 Add severity aggregation for pass, warn, and fail states
- [x] 1.3 Add recommendation fields for actionable next steps

## 2. Check Collection

- [x] 2.1 Implement assistant registry and configspace resolution checks
- [x] 2.2 Implement filesystem and event DB health checks
- [x] 2.3 Implement PID/process checks for assistant serve processes
- [x] 2.4 Implement connector configuration and connector status checks
- [x] 2.5 Implement harness profile, command lookup, environment, and cwd checks
- [x] 2.6 Implement recent error and log snippet collection

## 3. CLI Commands

- [x] 3.1 Add top-level `acpa doctor` command with assistant id, root, and configspace resolution
- [x] 3.2 Add `--verbose` human output rendering
- [x] 3.3 Add `--json` rendering from the structured report
- [x] 3.4 Add or normalize top-level `acpa status` behavior for assistant snapshots
- [x] 3.5 Extend `acpa logs` with bounded recent output and follow behavior where missing

## 4. Verification

- [x] 4.1 Add unit tests for report aggregation and rendering
- [x] 4.2 Add CLI tests for text, verbose, and JSON output
- [x] 4.3 Add tests that default probes do not create sessions, send messages, or modify memory
- [x] 4.4 Run `go test ./...`
