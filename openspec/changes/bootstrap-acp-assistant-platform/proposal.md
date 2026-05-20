## Why

This project needs a clear first-version contract before implementation because it combines local assistant lifecycle management, Agent Client Protocol integration, multiple harnesses, multiple IM adapters, session routing, and persistent memory. Establishing the platform boundaries now keeps the first version useful for a single host while preserving clean extension points for future harnesses, IM channels, thread-scoped sessions, and dashboard management.

## What Changes

- Introduce a local assistant instance model where each assistant runs as an independent process.
- Define fixed `workspace` and `configspace` boundaries for every assistant.
- Require every assistant to bind to one harness through a harness adapter.
- Allow every assistant to expose multiple IM channels through IM adapters.
- Bind each IM channel to a default long-lived session in the first version.
- Reserve `conversation_key` and `thread_key` fields so sessions can later evolve toward thread, topic, or conversation-level routing.
- Define controlled shared memory files in each workspace that both users and harnesses can update through auditable paths.
- Add a lightweight per-assistant SQLite event/index store for channel, session, message summary, memory revision, lifecycle, and error records.
- Leave dashboard implementation out of the first implementation while preserving the data model and status surfaces it will need.

## Capabilities

### New Capabilities

- `assistant-lifecycle`: Creating, configuring, starting, stopping, and inspecting independent local assistant instances.
- `harness-binding`: Binding an assistant to one host-local harness such as Codex, Claude Code, or opencode through a stable adapter boundary.
- `im-channel-sessions`: Managing IM channels, channel-scoped default sessions, and future-compatible conversation/thread session keys.
- `workspace-memory`: Maintaining fixed workspace memory files with user and harness write paths, revision history, and rollback support.
- `event-index`: Recording lightweight assistant-local events and indexes for audit, recovery, observability, and future dashboard support.

### Modified Capabilities

- None.

## Impact

- Affects the initial repository architecture, CLI command surface, local file layout, adapter interfaces, assistant process runtime, session routing behavior, memory update rules, and persistence strategy.
- Adds OpenSpec-driven implementation artifacts under `openspec/changes/bootstrap-acp-assistant-platform/`.
- Does not require a centralized daemon in the first version.
- Does not implement the dashboard in the first version, but the event/index model must support it later.
