## 1. Project Foundation

- [ ] 1.1 Choose and document the first implementation language/runtime.
- [ ] 1.2 Create the initial source, test, config, and documentation directory structure.
- [ ] 1.3 Add project build, lint, format, and test commands.
- [ ] 1.4 Add repository-level developer instructions for running and verifying the project.

## 2. Assistant Lifecycle

- [ ] 2.1 Define assistant metadata schema with id, name, workspace, configspace, harness binding, channels, and runtime status.
- [ ] 2.2 Implement CLI command to create an assistant and initialize configspace.
- [ ] 2.3 Implement CLI commands to list and inspect assistant instances.
- [ ] 2.4 Implement CLI commands to start and stop one assistant process.
- [ ] 2.5 Implement assistant removal that unregisters the assistant without deleting workspace or configspace by default.

## 3. Workspace And Configspace

- [ ] 3.1 Define configspace file layout for assistant configuration, channel bindings, session bindings, and adapter settings.
- [ ] 3.2 Define workspace memory file skeletons.
- [ ] 3.3 Implement workspace initialization without overwriting existing memory files by default.
- [ ] 3.4 Implement configspace loading on assistant process startup.

## 4. Harness Binding

- [ ] 4.1 Define the harness adapter interface for start, send, stream, cancel, shutdown, and capability declaration.
- [ ] 4.2 Implement harness binding validation during assistant creation.
- [ ] 4.3 Add an initial stub or mock harness adapter for local testing.
- [ ] 4.4 Add unsupported-capability handling for adapter operations such as cancellation.

## 5. IM Channels And Sessions

- [ ] 5.1 Define the IM adapter interface for inbound normalization, outbound messages, commands, and channel status.
- [ ] 5.2 Define normalized inbound and outbound message types.
- [ ] 5.3 Implement channel registration in assistant configspace.
- [ ] 5.4 Implement default long-lived session creation for first message on a channel.
- [ ] 5.5 Implement IM command handling to switch or create the active session for a channel.
- [ ] 5.6 Persist optional `conversation_key` and `thread_key` fields in session records.

## 6. Workspace Memory

- [ ] 6.1 Implement controlled memory update APIs for user-originated updates.
- [ ] 6.2 Implement controlled memory update APIs for harness-originated updates.
- [ ] 6.3 Validate memory update targets against the configured memory file set.
- [ ] 6.4 Record memory revision metadata for each update.
- [ ] 6.5 Implement rollback to a previous memory revision.

## 7. Event Index

- [ ] 7.1 Create the per-assistant SQLite event/index schema.
- [ ] 7.2 Initialize the event index during assistant creation.
- [ ] 7.3 Record lifecycle, channel, session, message summary, memory revision, harness, and runtime error events.
- [ ] 7.4 Add query helpers for last known assistant status, active sessions, recent errors, and recent memory revisions.
- [ ] 7.5 Ensure event index records avoid full message content unless explicitly configured.

## 8. Verification

- [ ] 8.1 Add tests for assistant creation, configspace persistence, and inspect output.
- [ ] 8.2 Add tests for channel default session creation and session switching.
- [ ] 8.3 Add tests for harness adapter capability handling.
- [ ] 8.4 Add tests for memory revisions and rollback.
- [ ] 8.5 Add tests for event index writes and dashboard-oriented queries.
- [ ] 8.6 Run OpenSpec validation for the change.
