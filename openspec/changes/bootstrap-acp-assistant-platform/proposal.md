## Why

This project needs a fresh first-version contract because the desired product is no longer a generic multi-IM harness wrapper. It is a Go single-binary, host-local assistant system that reuses the proven ACP runtime shape from `acp-webui` and adapts OpenClaw-style IM gateway, account, onboarding, routing, and channel operation patterns for Feishu and QQ Bot private-chat assistants.

## What Changes

- Use Go as the first-version implementation language and ship the system as one `acpa` binary.
- Provide CLI-first assistant creation, configuration, channel onboarding, status, logs, start, and stop workflows.
- Model each assistant as an independent local process with one fixed workspace, one fixed configspace, and one bound ACP harness provider.
- Support only Codex and Claude Code in the first version, and require both to connect through ACP stdio JSON-RPC adapters.
- Copy and trim the mature ACP runtime behavior from `acp-webui` instead of inventing a separate harness protocol.
- Support only Feishu private chat and QQ Bot C2C private chat in the first version.
- Run Feishu and QQ Bot as long-connection IM connectors inside the assistant process, following OpenClaw's channel/account/gateway model.
- Provide onboarding commands for Feishu and QQ Bot that write configspace files and show setup links plus terminal QR codes when a setup or pairing URL is available.
- Bind sessions by assistant, platform, account, private channel, and platform user instead of only by channel.
- Support session-scoped permission mode switching and channel-user default permission mode configuration.
- Gate permission mode changes through channel-user policy and harness capability checks.
- Resolve ACP permission requests only through the user who owns the session.
- Keep workspace memory files writable through controlled, revisioned paths.
- Keep config as YAML files and runtime/event/query state in a per-assistant SQLite database.
- Defer group chat, dashboard UI, opencode, non-ACP harness integration, and non-Feishu/QQ IM platforms.

## Capabilities

### New Capabilities

- `single-binary-cli`: Managing assistants, harnesses, IM onboarding, channels, status, and logs through one `acpa` binary.
- `assistant-configspace`: Defining assistant workspace/configspace layout, YAML configuration, secrets references, and SQLite placement.
- `acp-harness-runtime`: Running Codex and Claude Code through ACP stdio JSON-RPC using the proven `acp-webui` runtime model.
- `im-gateway-connectors`: Running Feishu and QQ Bot long-connection connector accounts inside the assistant process with OpenClaw-style account isolation.
- `private-user-sessions`: Routing private IM messages to sessions keyed by assistant, platform, account, private channel, and user.
- `permission-mode-policy`: Supporting session mode changes and channel-user default mode changes under explicit policy and harness capability checks.
- `owner-permission-resolution`: Delivering and resolving ACP permission requests only through the owning IM user.
- `workspace-memory`: Maintaining fixed workspace memory files with controlled user/harness write paths, revision history, and rollback support.
- `event-index`: Recording assistant-local events and query indexes for audit, recovery, diagnostics, and future dashboard support.

### Modified Capabilities

- None.

## Impact

- Replaces the earlier generic channel-scoped spec with a first-version architecture centered on Go, ACP-only harnesses, Feishu/QQBot private-chat connectors, single-binary onboarding, channel-user sessions, and policy-controlled mode switching.
- Affects the initial repository architecture, CLI command surface, local file layout, ACP runtime boundary, IM connector boundary, assistant process runtime, session routing behavior, memory update rules, and persistence strategy.
- Adds OpenSpec-driven implementation artifacts under `openspec/changes/bootstrap-acp-assistant-platform/`.
- Does not require a centralized daemon in the first version; the IM gateway layer runs inside each assistant process.
- Does not implement the dashboard in the first version, but the event/index model must support it later.
