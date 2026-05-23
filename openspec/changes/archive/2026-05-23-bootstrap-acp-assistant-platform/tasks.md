## 1. Go Project Foundation

- [x] 1.1 Create the Go module and `cmd/acpa` single-binary entrypoint.
- [x] 1.2 Create package structure for `internal/acp`, `internal/harness`, `internal/im`, `internal/assistant`, `internal/configspace`, `internal/workspace`, `internal/store`, and `internal/model`.
- [x] 1.3 Add build, format, lint, test, and OpenSpec validation commands.
- [x] 1.4 Add SQLite migration embedding and migration runner.

## 2. Single-Binary CLI

- [x] 2.1 Implement `acpa assistant create` to initialize workspace and configspace.
- [x] 2.2 Implement `acpa assistant list`, `inspect`, `start`, `stop`, and `remove`.
- [x] 2.3 Implement `acpa channel add feishu` and `acpa channel add qqbot` interactive onboarding.
- [x] 2.4 Implement terminal link and QR rendering support for onboarding URLs.
- [x] 2.5 Implement `acpa channel status` and `acpa logs --follow` using configspace and event index data.

## 3. Configspace And Workspace

- [x] 3.1 Define `assistant.yaml`, per-channel YAML files, `policies.yaml`, secret references, and `events.db` placement.
- [x] 3.2 Implement config loading, validation, and atomic config writes.
- [x] 3.3 Initialize workspace memory file skeletons without overwriting existing files.
- [x] 3.4 Implement secret reference resolution for env vars and file-backed secrets.

## 4. ACP Harness Runtime

- [x] 4.1 Copy and trim ACP stdio JSON-RPC peer behavior from `acp-webui`.
- [x] 4.2 Implement ACP initialize, capabilities, session/new, session/list, session/load, session/prompt, and session/cancel.
- [x] 4.3 Implement incoming session/update and session/request_permission handling as assistant-level events.
- [x] 4.4 Implement workspace-confined fs/read_text_file.
- [x] 4.5 Implement Codex launch profiles for manual, full_auto, yolo, reasoning effort, and response mode.
- [x] 4.6 Implement Claude Code launch profiles for manual and yolo, and reject full_auto.

## 5. IM Gateway Connectors

- [x] 5.1 Define connector account interfaces for start, stop, send, status, logs, token refresh, and inbound normalization.
- [x] 5.2 Implement Feishu WebSocket long-connection inbound and OpenAPI outbound for private chat.
- [x] 5.3 Implement QQ Bot official WebSocket gateway inbound and QQ Bot API outbound for C2C private chat.
- [x] 5.4 Implement per-account connection lifecycle, reconnect backoff, token cache, idempotency, and account-scoped logging.
- [x] 5.5 Ensure group chat, Feishu topic, QQ group, and guild/channel messages are ignored or rejected in first-version connectors.

## 6. Private User Sessions

- [x] 6.1 Define session and binding schema keyed by assistant, platform, account, private channel, and platform user.
- [x] 6.2 Implement first-message active session creation for a private user.
- [x] 6.3 Implement `/new`, `/session`, and session switch commands for the message sender.
- [x] 6.4 Persist ACP session id, external session id, local session id, permission mode, and launch profile key.
- [x] 6.5 Reserve conversation_key and thread_key fields without enabling group/thread routing.

## 7. Permission Mode Policy

- [x] 7.1 Define assistant defaults, account/channel policy, user policy, allowed modes, default mode, and can_set_default_mode.
- [x] 7.2 Implement `/mode manual|full_auto|yolo` for current session mode changes.
- [x] 7.3 Implement `/mode default manual|full_auto|yolo` for channel-user default mode changes.
- [x] 7.4 Validate every mode change against policy and bound harness capability.
- [x] 7.5 Implement mode switching with ACP session/load when available and new ACP session fallback when load is unavailable.

## 8. Owner Permission Resolution

- [x] 8.1 Store ACP permission requests with short approval ids and owner user identity.
- [x] 8.2 Send permission prompts only to the owning private chat target.
- [x] 8.3 Implement `/approve <id>` and `/reject <id>` for the session owner.
- [x] 8.4 Reject permission resolution attempts from non-owner identities.
- [x] 8.5 Implement pending permission timeout and cancellation behavior.

## 9. Workspace Memory

- [x] 9.1 Implement controlled memory update APIs for user-originated updates.
- [x] 9.2 Implement controlled memory update APIs for harness-originated updates.
- [x] 9.3 Validate memory update targets against the configured memory file set.
- [x] 9.4 Record memory revision metadata for each update.
- [x] 9.5 Implement rollback to a previous memory revision.

## 10. Event Index And Verification

- [x] 10.1 Create SQLite schema for events, connector status, sessions, bindings, ACP mappings, permissions, memory revisions, errors, and idempotency keys.
- [x] 10.2 Record lifecycle, connector, session, prompt, ACP update, permission, memory, and error events.
- [x] 10.3 Add query helpers for CLI status/logs and future dashboard use.
- [x] 10.4 Add tests for CLI creation, onboarding config writes, ACP runtime behavior, IM connector normalization, session routing, mode policy, permission ownership, memory revisions, and event index writes.
- [x] 10.5 Run Go tests and OpenSpec validation.
