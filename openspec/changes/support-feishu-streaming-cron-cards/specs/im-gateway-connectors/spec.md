## ADDED Requirements

### Requirement: Feishu supports streaming assistant cards
The Feishu connector SHALL support streaming assistant replies by sending an initial interactive card and updating that card while text chunks arrive.

#### Scenario: Start ordinary stream
- **WHEN** the assistant starts a stream for an ordinary Feishu private-chat response
- **THEN** the Feishu connector MUST send an interactive card to the target private chat
- **AND** the card MUST be updateable by subsequent stream segment updates

#### Scenario: Update ordinary stream content
- **WHEN** assistant text is appended to an ordinary Feishu stream segment
- **THEN** the Feishu connector MUST update the current card with the accumulated text
- **AND** the updated card MUST NOT include a visible title or header for the ordinary response

#### Scenario: Start new card after boundary
- **WHEN** the assistant opens a new Feishu stream segment after an ACP boundary
- **THEN** the Feishu connector MUST create a new interactive card instead of updating the previous card

#### Scenario: Streaming fallback
- **WHEN** the Feishu connector cannot start or update a streaming card
- **THEN** the assistant MUST be able to fall back to the existing text delivery path for the final response
