# ACPA Cron

ACPA cron is an assistant-owned scheduler for reminders and recurring assistant work. Jobs are persisted in the assistant SQLite store, executed by `assistant serve`, and delivered back through the creator's IM route unless delivery is disabled.

## Surfaces

ACPA exposes cron through one canonical host protocol:

- Harness responses return a fenced `cron` JSON block.
- Owner/admin IM commands pass the same JSON after `/cron`.

Cron is not a skill. The host injects concise cron protocol instructions into the managed harness instructions path and remains the only executor. The runtime does not parse arbitrary natural-language reminders itself; natural-language understanding belongs to the harness, while the host validates and executes structured cron requests.

## Protocol

Create:

```cron
{"action":"add","job":{"name":"sleep reminder","schedule":{"kind":"at","at":"2099-05-23T01:10:00+08:00"},"sessionTarget":"isolated","payload":{"kind":"agentTurn","message":"提醒用户：该睡觉啦！"},"delivery":{"mode":"announce","target":"origin"}}}
```

`job.name` is the stable title shown when the job runs. The harness should summarize a concise title at creation time. ACPA uses that title immediately at run time before the scheduled prompt is sent to the model.

List:

```cron
{"action":"list"}
```

Get:

```cron
{"action":"get","id":"cron_xxx"}
```

Pause/resume:

```cron
{"action":"update","id":"cron_xxx","patch":{"enabled":false}}
```

```cron
{"action":"update","id":"cron_xxx","patch":{"enabled":true}}
```

Rename explicitly:

```cron
{"action":"update","id":"cron_xxx","patch":{"name":"daily project summary"}}
```

Run manually:

```cron
{"action":"run","id":"cron_xxx"}
```

Run history:

```cron
{"action":"runs","id":"cron_xxx"}
```

Remove:

```cron
{"action":"remove","id":"cron_xxx"}
```

IM command form:

```text
/cron {"action":"list"}
```

## Execution Rules

- `schedule.kind` supports `at`, `every`, and `cron`.
- `at` schedules use `schedule.at` as an RFC3339 time with an explicit offset.
- `every` schedules use `schedule.everyMs`.
- `cron` schedules use five-field `schedule.expr` and optional IANA `schedule.tz`.
- `payload.kind` supports `agentTurn`; `payload.message` must be self-contained.
- `sessionTarget` supports `isolated` and `main`.
- Use `isolated` for reminders and scheduled assistant work by default; use `main` only when the scheduled task should intentionally continue the current conversation.
- `delivery.mode` supports `announce` with `target: "origin"`, or `none`.
- Cron execution suppresses the cron-management prompt prefix so the scheduled prompt is the only task instruction sent to the harness for that run.
- For Feishu origin delivery, Cron execution sends the stored title first, then streams the model reply into one or more cards.
- Every Feishu Cron reply card includes the stored title and the Cron id; if tool or permission boundaries split the response, each new card repeats that identity.
- Updating a Cron job preserves the existing title unless `patch.name` is explicitly provided.
- Only owner/admin users may execute cron tool calls or `/cron` commands.

Legacy `acpa-cron`, `create`/`delete`, `schedule_type`, `schedule_expr`, top-level `message`, `job_id`, and `/cron add --every ...` are not supported.
