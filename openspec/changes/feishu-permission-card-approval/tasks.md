## 1. Permission Decision Boundary

- [ ] 1.1 Add model types for structured permission prompts and permission decisions.
- [ ] 1.2 Add assistant runtime handling for permission decisions with owner checks, option mapping, stale-state handling, and idempotency.
- [ ] 1.3 Keep existing text approval and rejection commands working as fallback.

## 2. Feishu Card Delivery

- [ ] 2.1 Render Feishu permission prompts as interactive approval card payloads with fallback text.
- [ ] 2.2 Send card prompts through Feishu when possible and fall back to text on card send failure.
- [ ] 2.3 Register Feishu card action callbacks on the long-connection channel and normalize them into permission decisions.

## 3. Verification

- [ ] 3.1 Add tests for card prompt rendering and text fallback.
- [ ] 3.2 Add tests for owner approval, owner rejection, non-owner rejection, duplicate callbacks, stale callbacks, and text fallback.
- [ ] 3.3 Run OpenSpec validation and Go tests.
