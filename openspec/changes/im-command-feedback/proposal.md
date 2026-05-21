## Why

IM users need immediate, predictable feedback when they issue assistant commands. Missing or vague command responses make it difficult to know whether mode changes, session actions, and skill inspections succeeded.

## What Changes

- Add explicit success, failure, unknown-command, and permission-denied replies for supported IM commands.
- Split command availability between ordinary private users and owner/admin users.
- Return concise behavior and risk hints after permission mode changes.
- Add `/skills` and `/skills verbose` behavior so users can inspect effective skills and operators can debug skill sources.
- Keep first-version command responses as text rather than cards.

## Capabilities

### New Capabilities

- `im-command-feedback`: Defines IM command permissions, command feedback requirements, mode-change replies, status output, and skill listing behavior.

### Modified Capabilities

- None.

## Impact

- Affected code:
  - IM message command parsing
  - private-user session command handling
  - owner/admin permission checks
  - harness skill inventory reporting
  - tests for Feishu command responses and authorization behavior
- Feishu is the first target channel; the command contract should stay channel-neutral where practical.
