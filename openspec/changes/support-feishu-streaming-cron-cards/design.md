## Context

ACPA already receives ACP `session/update` notifications and collects `agent_message_chunk` text in `internal/acp`, but the assistant runtime only sees final prompt text except for the special permission-request flush path. Feishu outbound delivery already sends interactive cards for permission prompts, and the pinned `larksuite/oapi-sdk-go/v3` dependency includes a Channel `StreamController` that can PATCH an interactive card by message id.

Cron jobs already persist `name`, `prompt`, creator route, delivery mode, and run history. Current Cron execution waits for the harness to finish, then sends the final text. The new behavior needs an immediate host-generated Cron title card, followed by streamed model content that remains identifiable even when tool boundaries split the response into multiple cards.

## Goals / Non-Goals

**Goals:**

- Stream Feishu assistant text while an ACP prompt is running.
- Keep ordinary streamed Feishu replies titleless once the first text chunk is rendered.
- Split streamed output into a new card after tool, permission, or other non-text ACP boundaries.
- Send Cron run title cards before model execution and preserve the stored Cron title for every streamed Cron card.
- Mark every Cron card footer with the Cron id so split cards remain identifiable.
- Keep non-streaming connectors and fallback paths compatible with the existing final-text delivery behavior.

**Non-Goals:**

- Streaming QQ Bot responses.
- Rendering detailed tool call cards in Feishu.
- Changing ACP protocol semantics or requiring provider-specific APIs outside existing ACP notifications.
- Re-summarizing a Cron title at execution time.
- Automatically renaming Cron jobs on every update; renames must be explicit through `patch.name`.

## Decisions

### Streaming is modeled as prompt events plus final text

`internal/acp` will keep collecting final text, but `PromptOptions` gains an optional event callback. `agent_message_chunk` text produces append events. Permission requests and non-text or unknown `session/update` notifications produce boundary events before normal request/event handling continues.

Alternative considered: stream by polling the final collector from the assistant layer. That would miss explicit non-text boundaries and would couple timing to implementation details inside `internal/acp`.

### Assistant owns segmentation

The assistant runtime converts prompt events into a small stream protocol: start, append text, boundary, finish, fail. A boundary closes the current segment; the next text append opens a new segment. This keeps connector code focused on rendering cards and avoids teaching Feishu-specific code about ACP event shapes.

Alternative considered: let Feishu connector infer boundaries. That would require passing raw ACP notifications through the generic sender boundary and would make fallback behavior harder to test.

### Feishu streaming is capability-based

`model.OutboundMessage` will carry optional stream metadata, and `Sender` implementations can expose a streaming interface. Feishu long-connection accounts use SDK `Stream(ctx, input)` with an initial interactive card and update each segment through `UpdateCard`. If streaming cannot start or update fails, the runtime falls back to existing final text delivery.

Alternative considered: always send final Feishu text and add a separate progress card. That improves perceived responsiveness only a little and still leaves long outputs collapsed.

### Card rendering differs by stream kind

Ordinary assistant streams render no header/title after content begins. Cron streams render the stored job name as the header on every segment card and render a footer such as `Cron reply · cron_xxx`. Cron title delivery happens before the harness prompt, so the user sees the scheduled task identity immediately even if the model or tools are slow.

Alternative considered: show a generic "Assistant reply" title on all cards. The user explicitly requested no title for ordinary messages once the reply starts.

### Cron title changes are explicit

Cron creation continues to use `job.name` as the model-summarized title. Cron updates preserve the existing name unless the canonical update patch includes `name`. This gives stable execution identity while still allowing intentional rename operations.

Alternative considered: require the model to summarize a new title for every update. That makes pause/resume and small schedule edits unnecessarily mutate the user's recognizable title.

## Risks / Trade-offs

- [Risk] Feishu card PATCH calls can be rate-limited or fail mid-turn. -> Use throttled updates and fall back to final text delivery when streaming fails.
- [Risk] ACP providers may emit tool boundaries with shapes not yet seen in tests. -> Treat any non-text or unknown `session/update` as a boundary, and keep raw ACP event recording unchanged.
- [Risk] Long responses can exceed Feishu card content limits. -> Keep existing final text chunking as fallback and split stream segments when renderer limits are reached.
- [Risk] Duplicate delivery could happen if both streaming finish and existing final send run. -> The assistant stream manager returns whether it delivered streamed content; final text send only runs when streaming is unavailable or failed before user-visible content.

## Migration Plan

Existing Cron jobs keep their current `name` values and need no migration. Deploy as a backward-compatible runtime change: Feishu accounts gain streaming cards when long connection streaming is available, while QQ Bot and HTTP fallback keep final text messages. Rollback is code-only because no required schema migration is introduced.
