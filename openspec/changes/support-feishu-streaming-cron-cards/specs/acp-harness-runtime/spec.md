## ADDED Requirements

### Requirement: Prompt turn update callbacks
The ACP runtime SHALL surface prompt-turn text chunks and response boundaries to the assistant runtime while preserving final assistant text collection.

#### Scenario: Text chunk callback
- **WHEN** an ACP `session/update` notification contains `sessionUpdate: "agent_message_chunk"` with text content for an active prompt session
- **THEN** the runtime MUST append that text to the prompt's final text collector
- **AND** it MUST invoke the prompt event callback with a text chunk event for the same ACP session

#### Scenario: Non-text update boundary
- **WHEN** an ACP `session/update` notification for an active prompt is not a text `agent_message_chunk`
- **THEN** the runtime MUST invoke the prompt event callback with a boundary event before future text chunks are surfaced
- **AND** it MUST continue emitting the original ACP event to existing event handlers

#### Scenario: Permission request boundary
- **WHEN** an ACP prompt turn emits `session/request_permission`
- **THEN** the runtime MUST flush collected text as a text event if needed
- **AND** it MUST invoke the prompt event callback with a boundary event before processing the permission request
