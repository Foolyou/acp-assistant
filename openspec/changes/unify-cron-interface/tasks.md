## 1. Protocol Tests

- [x] 1.1 Add failing runtime tests for canonical `cron` fenced blocks, legacy `acpa-cron` rejection, and legacy field rejection.
- [x] 1.2 Add failing runtime tests for `/cron` canonical JSON creation and old flag rejection.
- [x] 1.3 Add failing harness overlay tests proving cron is delivered through instructions and no managed cron skill is materialized.

## 2. Runtime Protocol

- [x] 2.1 Replace `acpa-cron` parsing with canonical `cron` request parsing and validation.
- [x] 2.2 Implement canonical actions: `status`, `list`, `get`, `add`, `update`, `remove`, `run`, and `runs`.
- [x] 2.3 Map P0 schedule, payload, session target, and delivery objects into the existing cron job model while rejecting unsupported fields or values.

## 3. Command And Overlay Surface

- [x] 3.1 Change `/cron` to accept canonical JSON and reject the old `add --every/--message` form.
- [x] 3.2 Remove generated built-in cron skills and move the cron protocol text to host-managed instructions/prompt prefix.
- [x] 3.3 Update `/help`, `/skills`, and cron docs to describe the new canonical interface.

## 4. Verification

- [x] 4.1 Run focused cron, harness, and store tests.
- [x] 4.2 Run the full Go test suite.
- [x] 4.3 Validate the OpenSpec change and mark all tasks complete.
