## Context

The runtime already treats slash-prefixed text as commands and supports session, mode, approval, and memory operations. Current command responses are inconsistent: some commands return terse English text, some send manually from inside the handler, unknown commands expose raw errors, and mode changes do not explain behavior or risk.

Feishu is the immediate target, but the command semantics belong in the assistant runtime so future IM connectors can reuse them.

## Goals / Non-Goals

**Goals:**

- Ensure every command has an explicit success, failure, unknown-command, or permission-denied response.
- Split command permissions between ordinary private users and owner/admin users.
- Provide concise risk and behavior hints for permission mode changes.
- Add effective skill listing and verbose skill-source inspection.
- Keep v1 responses as text.

**Non-Goals:**

- Build card-based command UI.
- Implement full role management.
- Add group chat command semantics beyond current private-user sessions.
- Implement a skill package manager.

## Decisions

### Centralize command result handling

Command handlers will return a structured result containing response text, visibility, and error category. The runtime will be responsible for sending exactly one response for command outcomes.

Alternative considered: continue returning raw strings and errors. That keeps the code smaller but makes permission-denied and unknown-command behavior harder to standardize.

### Use policy-derived owner/admin checks first

For v1, owner/admin capability will be derived from existing channel options and policy configuration where available. Commands that change mode defaults, inspect verbose skill paths, or expose sensitive diagnostics will require owner/admin access. Ordinary private users can use low-risk session-local commands.

Alternative considered: add a full role model now. That is premature without a broader admin UI and audit design.

### Keep mode responses concise

`/mode <mode>` will return a short confirmation plus a behavior/risk hint. Full session details belong in `/status`, not in every command response.

Alternative considered: return a detailed status card after every mode change. That gives more information but adds noise in chat.

### Build skill inventory as a runtime service

`/skills` should report the effective skill set the harness is expected to see. `/skills verbose` should include source layers and paths so operators can diagnose leaks such as skills discovered from an unintended parent directory.

Alternative considered: ask the harness itself to list skills. That may be useful later, but v1 can report ACPA-managed sources deterministically and include harness overlay paths.

## Risks / Trade-offs

- [Risk] Owner detection may be incomplete for manually configured Feishu apps. -> Fall back to explicit policy configuration and document when owner-only commands are unavailable.
- [Risk] Skill inventory may not exactly match provider-native hidden/system skills. -> Label ACPA-managed skills and provider/system skills separately when known.
- [Risk] More command replies may feel noisy. -> Keep success text short and reserve full details for explicit status/verbose commands.
- [Risk] Existing tests may depend on old English response strings. -> Update tests around semantic outcomes instead of incidental wording where possible.

## Migration Plan

Preserve existing command names. Improve response text and permission behavior in place. Unknown commands will remain non-harness commands and will not be forwarded as normal prompts.

## Open Questions

None for the first version.
