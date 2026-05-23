## Why

Feishu users currently receive assistant responses only after the ACP turn finishes, which makes long-running prompts feel stalled and causes tool-separated output to collapse into one final message. Cron runs also need an immediate, identifiable notification that preserves the job title created by the model before the scheduled task executes.

## What Changes

- Stream ACP assistant text chunks to Feishu as updating message cards when the connector supports card streaming.
- Keep ordinary Feishu assistant replies titleless once text starts rendering.
- Split streamed Feishu output into a new card whenever ACP emits a non-text/tool/permission boundary so later text is not appended to earlier content.
- Deliver Cron runs by first sending the stored job title without waiting for the model, then streaming the model response into one or more Cron-identified Feishu cards.
- Require every Cron stream card, including cards after tool boundaries, to carry the stored Cron title and Cron id.
- Extend Cron update support so a job can be renamed intentionally while preserving the existing title by default.
- Preserve final-text delivery and text fallback for connectors that do not support streaming cards.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `acp-harness-runtime`: Surface ACP prompt chunk and boundary events to the assistant runtime during a prompt turn.
- `private-user-sessions`: Stream and segment assistant output for IM delivery while preserving final response fallback.
- `im-gateway-connectors`: Add Feishu streaming card delivery and update behavior.
- `assistant-cron-scheduler`: Preserve Cron titles, send immediate Cron run title cards, and identify every Cron stream card with title and id.

## Impact

- Affected packages: `internal/acp`, `internal/assistant`, `internal/im`, `internal/store`, `cmd/acpa`, and Cron documentation.
- Uses existing `github.com/larksuite/oapi-sdk-go/v3` Feishu `StreamController`/card update support.
- Adds internal streaming callback interfaces but does not change external CLI command syntax except allowing canonical Cron `patch.name`.
- Requires focused tests for ACP chunk callbacks, assistant segmentation, Feishu card streaming, and Cron streaming delivery.
