## Context

ACPA already has partial inspection surfaces: `assistant inspect`, `channel status`, and `logs`. These commands expose useful pieces of state, but an operator still has to correlate registry entries, assistant config, channel config, event DB state, log files, PID files, harness profiles, and connector errors manually.

The first diagnostic layer should stay local and CLI-oriented. It should not send IM messages, create harness sessions, or mutate user-visible assistant state.

## Goals / Non-Goals

**Goals:**

- Make `acpa doctor` the primary "why is this assistant not working?" command.
- Produce both human-readable and JSON diagnostic reports from the same structured data.
- Keep `status` and `logs` narrowly focused and script-friendly.
- Run safe lightweight probes by default.
- Provide verbose diagnostic detail without overwhelming the default output.

**Non-Goals:**

- Implement a real end-to-end smoke test that sends IM messages or prompts a harness.
- Replace event logs with a full tracing system.
- Add remote observability, metrics, or alerting.
- Require the local daemon from the first implementation.

## Decisions

### Build a structured diagnostic report first

`doctor` will gather checks into a report object with stable fields for assistant identity, config paths, process state, connector state, database health, harness profile health, recent errors, and recommended actions. The CLI renderer will produce the default human-readable output from that object, and `--json` will serialize the same object.

Alternative considered: print checks directly as text. That is faster initially but makes Web UI reuse and tests harder.

### Keep default probes safe

Default probes may check filesystem permissions, open and migrate the event DB, inspect connector configuration, resolve harness launch profiles, locate harness commands, and run bounded version-style checks where available. They must not send outbound messages, create real sessions, trigger permission prompts, or modify user-visible memory.

Alternative considered: run an end-to-end test by sending a prompt. That gives stronger confidence but can pollute sessions, wake users, or trigger authorization side effects.

### Separate status, logs, and doctor responsibilities

`status` will remain a current-state snapshot. `logs` will stream or print recent events/log lines. `doctor` will interpret state and produce pass/warn/fail checks with recommendations.

Alternative considered: make `doctor` a wrapper around `status` and `logs`. That would not provide enough structured diagnosis.

### Support verbose without changing semantics

`--verbose` will include additional check details, file paths, command paths, raw recent events, and relevant stderr snippets. It will not run more invasive probes than default. Future invasive probes can use a separate `--deep` or `smoke` command.

## Risks / Trade-offs

- [Risk] Version probes for harness commands may be slow or unsupported. -> Use short timeouts and report unsupported probes as warnings instead of failures.
- [Risk] Running DB migrations during diagnostics can feel surprising. -> Treat DB open/migrate as the same local health behavior existing commands already use; do not write user-visible assistant state.
- [Risk] Human output may drift from JSON fields. -> Test report construction separately from rendering.
- [Risk] Operators may expect `doctor` to prove Feishu delivery. -> Make the non-goal explicit in output and reserve true E2E checks for a later explicit smoke command.

## Migration Plan

Add `doctor` without removing existing commands. Existing `assistant inspect`, `channel status`, and `logs` behavior remains available while new status aliases can be introduced gradually.

## Open Questions

None for the first version.
