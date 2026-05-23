## Why

ACP permission requests currently rely on users typing approval commands in chat. Feishu already provides interactive message cards, which are a better fit for explicit, low-friction action approval while preserving owner-only resolution.

## What Changes

- Send Feishu ACP permission prompts as interactive message cards with approve and reject buttons.
- Handle Feishu card action callbacks from the long-connection channel and resolve the pending ACP permission.
- Verify the callback operator matches the original session owner before resolving the permission.
- Preserve text `approve <id>` and `reject <id>` commands as a fallback approval path.
- Record enough card callback metadata for idempotency and diagnostics without storing sensitive action payloads in plain logs.

## Capabilities

### New Capabilities

- `feishu-permission-card-approval`: Feishu interactive card approval for agent action permission requests.

### Modified Capabilities

- None.

## Impact

- Feishu connector long-connection event registration and callback normalization.
- Assistant permission request delivery and resolution path.
- Outbound message model and Feishu send formatting.
- SQLite permission/event metadata where needed for card message tracking.
- Tests for card prompt rendering, owner verification, duplicate callbacks, approval, rejection, and text fallback.
