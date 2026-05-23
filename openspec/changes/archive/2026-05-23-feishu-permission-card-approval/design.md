## Context

ACP runtimes can pause a session with `session/request_permission` when an agent wants to execute an action that needs user approval. The assistant already records the pending request, sends a text prompt to the private-chat session owner, and resolves the request when the owner replies with `approve <id>` or `reject <id>`.

Feishu private-chat inbound messages are handled through the larksuite long-connection channel. The same SDK exposes card action callbacks, including the operator identity and action payload. That makes Feishu interactive cards a natural approval UI while keeping the local assistant process as the permission authority.

## Goals / Non-Goals

**Goals:**

- Use Feishu interactive message cards for ACP permission prompts in Feishu private chats.
- Keep owner-only resolution in assistant runtime, not in the connector.
- Treat card callbacks and text commands as two input forms for the same pending permission record.
- Make duplicate or stale card clicks harmless.
- Keep QQ Bot and non-card-capable senders on the existing text fallback.

**Non-Goals:**

- No Feishu OAuth user authorization or user access token storage.
- No group chat, topic, or multi-approver workflow.
- No broad redesign of ACP permission storage.
- No dependency on a public webhook server; long connection remains the first-version connector mode.

## Decisions

### Add a structured permission prompt message kind

Outbound chat currently carries only text. Introduce an optional permission prompt payload on outbound messages so the runtime can say "send this approval request" without knowing the platform UI. Plain text remains populated as fallback content.

Alternative considered: build Feishu card JSON directly in assistant runtime. Rejected because it would leak platform-specific formatting into permission policy logic.

### Route card callbacks as permission decisions

Extend the IM account boundary with a permission decision handler for card clicks. Feishu connector normalizes `CardActionEvent` into assistant-local fields: platform, account, chat, operator user, short approval id, requested option, event id, and action metadata. Assistant runtime performs idempotency, owner matching, pending-state checks, and ACP resolution.

Alternative considered: convert card clicks into synthetic text commands. Rejected because callback events have distinct ids and payloads; preserving their shape makes duplicate handling and diagnostics clearer.

### Encode only minimal card action values

Card buttons carry the short approval id and semantic option (`approve` or `reject`). The pending permission record remains the source of truth for owner, ACP request id, option list, expiration, and status.

Alternative considered: put ACP request details in card values. Rejected because cards can be forwarded or inspected and should not carry more authority than needed.

### Preserve text fallback

The runtime still sends fallback text with `approve <id>` and `reject <id>`, and the existing command path remains valid. This covers unsupported clients, malformed card callbacks, tests, and non-Feishu channels.

## Risks / Trade-offs

- Card callbacks may arrive more than once -> Reuse idempotency by callback event id and make already-resolved permissions return without sending a second ACP response.
- The card may be clicked by a non-owner if forwarded -> Runtime compares callback operator with the pending permission owner and rejects mismatches.
- Feishu card send can fail while text send would work -> The connector should fall back to sending the plain text prompt when card send fails.
- Card UI can drift from ACP options -> Runtime maps buttons through the pending permission options before resolving, preserving existing option selection behavior.

## Migration Plan

No data migration is required. Existing pending permissions continue to resolve through text commands. After deployment, new Feishu permission prompts use cards when possible and text fallback otherwise.
