## Context

The project starts from an empty repository and targets a host-local personal assistant platform. It connects IM systems such as Feishu, WeCom, and QQ to local harnesses such as Codex, Claude Code, and opencode through Agent Client Protocol oriented adapter boundaries.

The first version prioritizes independent assistant processes over a central daemon. Each assistant has a fixed workspace for user-facing project and memory files, plus a fixed configspace for runtime configuration, adapter configuration, local indexes, and process state. The system should remain simple enough to run locally, but the domain model must not block later thread-scoped sessions or dashboard management.

## Goals / Non-Goals

**Goals:**

- Model each assistant as an independent local process.
- Keep workspace and configspace as explicit persistent boundaries.
- Bind each assistant to exactly one harness through a harness adapter.
- Support multiple IM channels per assistant through IM adapters.
- Route each channel to one default long-lived session in the first version.
- Reserve session fields for later conversation/thread-specific routing.
- Treat workspace memory files as shared writable state with controlled updates, revisions, and rollback.
- Use a lightweight per-assistant SQLite event/index store for audit, recovery, observability, and future dashboard support.

**Non-Goals:**

- No centralized daemon is required in the first version.
- No dashboard UI is implemented in the first version.
- No multi-user, multi-tenant, or hosted service model is included.
- No plugin marketplace or dynamic third-party adapter loading is required initially.

## Decisions

### Independent assistant process per instance

Each assistant runs as its own process. The process owns IM channel connections, session routing, harness adapter lifecycle, workspace access, configspace access, and event indexing for that assistant.

This favors operational isolation and matches the user's first-version preference. A single assistant crash should not terminate every other assistant. It also makes per-assistant workspace and configspace boundaries concrete.

Alternative considered: a central daemon that manages all assistants. That would simplify dashboard and process supervision, but it makes the first version heavier and shifts the architecture toward a platform service too early.

### File-first persistence with SQLite event/index support

Workspace and configspace remain the primary durable boundaries. Config files describe assistants, channels, harness bindings, session bindings, memory file rules, and adapter settings. A per-assistant SQLite database stores events and query-friendly indexes.

This keeps the system easy to inspect, back up, and migrate while avoiding painful ad hoc log parsing for session history, memory revisions, and future dashboard queries.

Alternative considered: making SQLite the full source of truth. That would improve querying but would make workspace/configspace less transparent and introduce consistency overhead earlier.

### Adapter boundaries for harnesses and IM channels

The runtime uses two stable adapter families:

- Harness adapters translate assistant/session messages into ACP or harness-specific interactions.
- IM adapters translate platform-specific events into normalized inbound messages and outbound responses.

The core runtime should know about normalized messages, sessions, and events, not Feishu webhook details or Codex process flags.

Alternative considered: direct first-adapter integrations in the runtime. That is faster for one harness and one IM channel but creates costly coupling once Claude Code, opencode, Feishu, WeCom, and QQ coexist.

### Channel-scoped sessions now, thread-compatible sessions later

The first version creates one default long-lived session per IM channel. The session model still includes optional `conversation_key` and `thread_key` fields so future routing can distinguish Feishu threads, group topics, or other sub-conversations without changing the persisted schema.

Alternative considered: thread-scoped sessions from the beginning. That would be more precise but would delay the first working local assistant because every IM adapter has different thread semantics.

### Controlled writable memory files

Workspace memory files are shared writable state. Users can update them through IM commands or local edits. Harnesses can update them only through approved tool or command paths that validate targets and record revisions.

This keeps memory useful while reducing the risk that one bad model response silently corrupts long-term context.

Alternative considered: user-only memory writes. That is safer but loses an important assistant capability the project explicitly wants.

## Risks / Trade-offs

- Independent processes make fleet-wide management harder -> Mitigate by writing consistent configspace state and event indexes that a later dashboard can scan.
- File-first persistence can drift from SQLite indexes -> Mitigate by treating files/config as the source of truth and making event indexes rebuildable where practical.
- Harness adapters may expose different capabilities -> Mitigate through explicit adapter capability declarations and graceful unsupported-operation errors.
- IM platforms have inconsistent threading semantics -> Mitigate by starting with channel-scoped sessions and preserving optional thread/conversation keys.
- Writable memory can be polluted by bad updates -> Mitigate with controlled update paths, revision records, and rollback.

## Migration Plan

There is no existing production state to migrate. Initial implementation should create repository structure, CLI scaffolding, configspace schema, workspace memory skeletons, adapter interfaces, and event index initialization.

Rollback for early versions is file-based: stop assistant processes, restore workspace/configspace from backup or git, and rebuild event indexes where supported.

## Open Questions

- Which language/runtime should be used for the first implementation.
- Which harness adapter should be implemented first.
- Which IM adapter should be implemented first.
- Whether assistant process supervision belongs in the CLI, shell scripts, systemd user units, or a later dashboard helper.
