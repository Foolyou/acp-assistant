## 1. ACP Prompt Events

- [x] 1.1 Add tests proving ACP prompt text chunks invoke a prompt event callback while still returning final text.
- [x] 1.2 Add tests proving non-text session updates and permission requests emit stream boundary events.
- [x] 1.3 Implement ACP prompt event types and callback plumbing in `internal/acp`.

## 2. Assistant Stream Segmentation

- [x] 2.1 Add tests proving ordinary streamed private messages append text, split on boundaries, and suppress duplicate final sends.
- [x] 2.2 Add tests proving non-streaming senders keep existing final-text behavior.
- [x] 2.3 Implement assistant stream context, stream sender interface, and segmentation manager.
- [x] 2.4 Wire normal inbound prompt handling to the stream manager.

## 3. Feishu Streaming Cards

- [x] 3.1 Add connector tests for ordinary Feishu card streams with no visible title after text starts.
- [x] 3.2 Add connector tests for opening a new Feishu card after a stream boundary.
- [x] 3.3 Extend the Feishu long-connection abstraction and SDK adapter with streaming card support.
- [x] 3.4 Implement Feishu stream card rendering and text fallback on stream failure.

## 4. Cron Titles And Streaming Delivery

- [x] 4.1 Add tests proving Cron update preserves name unless `patch.name` is supplied.
- [x] 4.2 Implement canonical Cron `patch.name` parsing and store update support.
- [x] 4.3 Add tests proving Cron runs send an immediate title card and stream every segment with title plus Cron id.
- [x] 4.4 Wire Cron execution to the stream manager with Cron stream context.

## 5. Documentation And Validation

- [x] 5.1 Update Cron docs and harness cron instructions to describe title creation, explicit rename, and streaming delivery semantics.
- [x] 5.2 Run focused tests for `internal/acp`, `internal/assistant`, `internal/im`, and `internal/store`.
- [x] 5.3 Run full Go test suite.
- [x] 5.4 Validate the OpenSpec change.
