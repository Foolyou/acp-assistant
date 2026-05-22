# ACPA Cron

ACPA cron is an assistant-owned scheduler for reminders and recurring assistant work. Jobs are persisted in the assistant SQLite store, executed by `assistant serve`, and delivered back through the creator's IM route unless delivery is disabled.

## Surfaces

ACPA exposes cron through two surfaces:

- Owner/admin IM commands for deterministic operations: `/cron add`, `/cron list`, `/cron pause`, `/cron resume`, `/cron remove`, `/cron run`, and `/cron runs`.
- A built-in harness skill named `acpa-cron`, injected into Codex and Claude overlays. The same cron protocol is also included in the first prompt sent to each ACP session so active harnesses can see it without repeating it through the whole session history. The protocol tells the harness to return a fenced `acpa-cron` JSON block for `create`, `delete`, and `list` operations. The assistant runtime executes the block and sends the confirmation or error to the user.

The runtime does not parse arbitrary natural-language reminders itself. Natural-language understanding belongs to the harness; the host only validates and executes the structured cron tool call.

## Harness Tool Block

Create:

```acpa-cron
{"action":"create","name":"sleep reminder","schedule_type":"at","schedule_expr":"2099-05-23T01:10:00+08:00","timezone":"Asia/Shanghai","message":"该睡觉啦！","target":"direct","delivery":"origin"}
```

Delete:

```acpa-cron
{"action":"delete","job_id":"cron_xxx"}
```

List:

```acpa-cron
{"action":"list"}
```

## Execution Rules

- `schedule_type` supports `at`, `every`, and five-field `cron`.
- `at` schedules should use RFC3339 with an explicit offset.
- `every` schedules use Go durations such as `10m`, `2h`, or `24h`.
- `target` supports `direct`, `isolated`, and `main`.
- Use `direct` for simple reminders; ACPA sends `message` exactly at the scheduled time without invoking the harness again.
- Use `isolated` or `main` for scheduled assistant work that needs model reasoning; scheduled prompts should be self-contained.
- `delivery` defaults to `origin`.
- Only owner/admin users may execute cron tool calls or `/cron` commands.
