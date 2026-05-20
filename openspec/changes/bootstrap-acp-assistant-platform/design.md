## Context

The project targets a host-local personal assistant platform controlled from IM private chats. It should be implemented in Go, distributed as a single `acpa` binary, and use that binary for assistant creation, channel onboarding, assistant operation, status, and logs.

The ACP side should follow `acp-webui`, which already runs Codex and Claude Code through ACP stdio JSON-RPC. `acp-assistant` should copy and trim the mature runtime pieces rather than creating a new harness protocol.

The IM side should follow OpenClaw's product and technical model where useful: channels are connector accounts, routing is deterministic host configuration, accounts own isolated long-connection state, onboarding is CLI-driven, and channel operations are visible through status/log commands. Unlike OpenClaw's general gateway, the first version keeps the gateway layer inside each assistant process to preserve the chosen model that one assistant is one independent process.

## Goals / Non-Goals

**Goals:**

- Use Go and ship one `acpa` binary.
- Create, configure, onboard, start, stop, inspect, and diagnose assistants through `acpa`.
- Model each assistant as an independent local process with fixed workspace and configspace.
- Bind each assistant to exactly one ACP harness provider: Codex or Claude Code.
- Reuse `acp-webui`'s ACP stdio JSON-RPC runtime shape for initialize, session lifecycle, prompt turns, session updates, cancellation, and permission requests.
- Support Feishu private chat through WebSocket long connection.
- Support QQ Bot C2C private chat through the official QQ Bot WebSocket gateway.
- Support multiple connector accounts per assistant with account-isolated credentials, token cache, logs, and connection status.
- Route private-chat messages to sessions keyed by assistant, platform, account, private channel, and platform user.
- Support current-session permission mode switching and channel-user default permission mode switching.
- Enforce mode policy by user/channel and harness capability.
- Resolve ACP permission requests only through the session owner.
- Treat workspace memory files as shared writable state with controlled updates, revisions, and rollback.
- Use configspace YAML for durable configuration and assistant-local SQLite for runtime/event/query state.
- Preserve enough state for a future dashboard without building the dashboard now.

**Non-Goals:**

- No centralized daemon or shared gateway is required in the first version.
- No dashboard UI is implemented in the first version.
- No multi-user, multi-tenant, or hosted service model is included.
- No group chat, mention routing, Feishu topic routing, QQ group routing, or guild/channel routing is included.
- No opencode or non-ACP harness integration is included.
- No plugin marketplace or dynamic third-party connector loading is required initially.

## Decisions

### Go single binary

The project should produce one Go binary named `acpa`. It should contain the CLI, assistant process runtime, ACP client runtime, IM connector gateway layer, onboarding helpers, SQLite migrations, and operational commands.

This matches the host-local deployment target and keeps install, service setup, and operational troubleshooting simple. It also aligns with `acp-webui`, which already provides useful Go implementation patterns for ACP runtime and SQLite persistence.

Alternative considered: a TypeScript/Node implementation. Node would fit some IM SDK ecosystems, but the first-version IM targets have workable Go paths and the system needs stable long-running local processes.

### Independent assistant process per instance

Each assistant runs as its own process. The process owns its Feishu/QQBot connector accounts, deterministic message routing, ACP harness runtime manager, workspace access, configspace access, memory updates, and event indexing.

This preserves isolation. A broken connector or failed harness for one assistant should not stop other assistants. A future dashboard can discover and manage multiple assistant processes through configspace and status APIs, but it is not the first-version runtime owner.

Alternative considered: an OpenClaw-style single shared gateway for all assistants. That may become useful later, but it conflicts with the first-version decision that each assistant owns one fixed workspace and configspace.

### ACP-only harness integration

Harness integration must be ACP-only. The first implementation should copy and adapt the proven `acp-webui` runtime shape:

- start a configured ACP adapter child process;
- communicate over stdio JSON-RPC;
- call `initialize`;
- call `session/new`, `session/list`, `session/load`, `session/prompt`, and `session/cancel` where supported;
- handle `session/update`;
- handle `session/request_permission`;
- expose `fs/read_text_file` constrained to the session workspace;
- parse and store ACP capabilities.

Codex defaults to `codex-acp`. Claude Code defaults to `npx --yes @agentclientprotocol/claude-agent-acp`. Codex supports `manual`, `full_auto`, and `yolo` launch profiles. Claude Code supports `manual` and `yolo`; `full_auto` must be rejected for Claude even if user policy allows it.

Alternative considered: parsing the normal Codex or Claude CLI. That is explicitly rejected because ACP is the required harness protocol.

### In-process IM gateway connectors

The assistant process contains an IM gateway layer with connector accounts. A connector account owns its platform connection lifecycle, API client, token cache, credentials reference, media/cache root, logs, and status. This follows OpenClaw's account-isolated gateway model while keeping the gateway local to one assistant.

Feishu uses WebSocket long connection for inbound events and OpenAPI for outbound messages. QQ Bot uses the official QQ Bot WebSocket gateway for inbound events and QQ Bot APIs for outbound messages. Webhook mode is not part of the first version.

Alternative considered: all connectors through webhooks. That adds public callback setup and is unnecessary for the selected first-version private-chat targets.

### CLI onboarding with links and QR codes

Assistant and channel setup must be CLI-first:

```text
acpa assistant create
acpa channel add feishu
acpa channel add qqbot
acpa assistant start
acpa channel status
acpa logs --follow
```

Onboarding should show human-openable setup links and terminal QR codes when a platform setup or pairing URL is available. QQ Bot should support a QR pairing flow when available, while also supporting manual `AppID:AppSecret` or secret-reference entry. Feishu should show Open Platform links, required permissions, event subscription steps, and QR codes for available setup URLs or generated local guide pages; it should also support manual app credential entry.

Alternative considered: editing config files by hand as the primary path. Config files remain supported, but they should not be the first-run user experience.

### Private user sessions

The first version supports private chats only. A session binding is keyed by:

```text
assistant_id + platform + account_id + private_channel_id + platform_user_id
```

The record reserves `conversation_key` and `thread_key`, but group, topic, and thread routing are out of scope. This improves on the earlier channel-only model and avoids context leakage when multiple users message the same assistant account.

### Controlled writable memory files

Workspace memory files are shared writable state. Users can update them through IM commands or local edits. Harnesses can update them only through approved tool or command paths that validate targets and record revisions.

This keeps memory useful while reducing the risk that one bad model response silently corrupts long-term context.

Alternative considered: user-only memory writes. That is safer but loses an important assistant capability the project explicitly wants.

### File configuration with SQLite runtime indexes

Configuration truth lives in configspace YAML files. Runtime/session/event truth lives in a per-assistant SQLite database. Memory truth lives in workspace files plus revision metadata.

Suggested layout:

```text
<configspace>/
  assistant.yaml
  channels/
    feishu-main.yaml
    qqbot-main.yaml
  policies.yaml
  events.db
  secrets/

<workspace>/
  memory/
    identity.md
    preferences.md
    facts.md
    project.md
  memory/.revisions/
  artifacts/
  inbox/
```

YAML keeps the durable assistant definition inspectable and repairable. SQLite handles frequent state changes, idempotency, permission requests, session bindings, ACP mapping, connector status, and future dashboard queries.

## Risks / Trade-offs

- Independent processes make fleet-wide management harder -> Mitigate with consistent configspace layout, status commands, event indexes, and future dashboard-readable state.
- Copying `acp-webui` runtime code can drift over time -> Mitigate by preserving package boundaries and documenting which runtime behaviors are intentionally shared.
- ACP harness capability differences can confuse users -> Mitigate through explicit harness capability checks before mode changes, load/resume, list, cancel, or media prompts.
- Long-connection IM connectors need robust reconnect behavior -> Mitigate with per-account lifecycle state, backoff, token refresh, idempotency keys, and status/log commands.
- QR/link onboarding may not be equally rich on every platform -> Mitigate with manual credential fallback and explicit platform-specific setup checklist output.
- Writable memory can be polluted by bad updates -> Mitigate with controlled update paths, revision records, and rollback.
- SQLite can drift from YAML/memory files -> Mitigate by treating YAML and workspace memory as durable sources and keeping runtime indexes rebuildable where practical.

## Migration Plan

There is no existing production state to migrate. The existing coarse OpenSpec artifacts are replaced by this first-version architecture spec. Initial implementation should create the Go module, CLI skeleton, configspace schema, SQLite migrations, ACP runtime copy/trim, Feishu connector, QQBot connector, assistant runtime, session routing, policy evaluator, memory revision system, and tests.

Rollback for early versions is file-based: stop assistant processes, restore workspace/configspace from backup or git, and rebuild event indexes where supported.

## Open Questions

- Whether `acpa assistant install-service` should be part of the first implementation or a follow-up.
- Whether Feishu onboarding should generate a local HTML setup page for QR display and checklist presentation.
- Whether QQ Bot QR onboarding can be implemented completely through available official APIs or needs a manual credential fallback as the default.
