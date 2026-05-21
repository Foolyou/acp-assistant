## 1. Source Layout

- [ ] 1.1 Add global ACPA source initialization for `global/instructions.md` and `global/skills/`
- [ ] 1.2 Add assistant configspace source initialization for `instructions.md` and `skills/`

## 2. Overlay Generation

- [ ] 2.1 Implement deterministic harness overlay generation for Codex and Claude providers
- [ ] 2.2 Generate Codex `configspace/harness/codex-home` with minimal config and copied ACPA skill sources
- [ ] 2.3 Generate Claude `configspace/harness/claude-plugin` with plugin metadata and copied ACPA skill sources

## 3. Harness Launch

- [ ] 3.1 Add launch-profile environment support and merge it into ACP process startup
- [ ] 3.2 Prepare overlays before starting assistant harness sessions
- [ ] 3.3 Launch Codex with assistant-scoped `CODEX_HOME`
- [ ] 3.4 Launch Claude Code with assistant-scoped `--plugin-dir`

## 4. Verification

- [ ] 4.1 Add tests for source initialization and generated overlay files
- [ ] 4.2 Add tests for launch profile env/args behavior
- [ ] 4.3 Run `go test ./...`
