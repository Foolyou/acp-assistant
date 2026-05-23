## MODIFIED Requirements

### Requirement: IM updates are aggregated for chat delivery
The assistant SHALL aggregate ACP updates into IM-appropriate messages instead of sending every token or internal event, and SHALL stream segmented updates when the connector supports streaming delivery.

#### Scenario: Prompt starts
- **WHEN** a normal private message is dispatched to ACP
- **THEN** the assistant MUST send or update a concise acknowledgement according to connector capability

#### Scenario: Text chunks stream to capable connector
- **WHEN** the ACP prompt turn emits assistant text chunks and the target connector supports streaming delivery
- **THEN** the assistant MUST stream those chunks into the current outbound message segment
- **AND** it MUST avoid sending a duplicate final response for the same streamed segment

#### Scenario: Boundary starts a new message segment
- **WHEN** a streamed prompt turn receives a tool, permission, non-text, or unknown ACP boundary after assistant text has started
- **THEN** the assistant MUST stop appending future text to the previous outbound message segment
- **AND** the next assistant text chunk MUST open a new outbound message segment

#### Scenario: Ordinary streamed Feishu message has no title
- **WHEN** an ordinary private Feishu response is streamed after the first assistant text chunk is available
- **THEN** the visible message card MUST render the assistant text without a title or header

#### Scenario: Final assistant text is available
- **WHEN** the ACP prompt turn completes with assistant text and streaming delivery was not used successfully
- **THEN** the assistant MUST send the final response to the owning private chat target

#### Scenario: Response exceeds platform limit
- **WHEN** an outbound response exceeds the connector's configured text chunk limit
- **THEN** the assistant MUST split or attach the response according to connector capability
