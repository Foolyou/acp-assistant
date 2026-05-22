CREATE TABLE IF NOT EXISTS cron_jobs (
  id TEXT PRIMARY KEY,
  assistant_id TEXT NOT NULL,
  name TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  schedule_type TEXT NOT NULL,
  schedule_expr TEXT NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  prompt TEXT NOT NULL,
  target TEXT NOT NULL,
  delivery_mode TEXT NOT NULL,
  creator_platform TEXT NOT NULL DEFAULT '',
  creator_account_id TEXT NOT NULL DEFAULT '',
  creator_private_channel_id TEXT NOT NULL DEFAULT '',
  creator_platform_user_id TEXT NOT NULL DEFAULT '',
  creator_conversation_key TEXT NOT NULL DEFAULT '',
  creator_thread_key TEXT NOT NULL DEFAULT '',
  permission_mode TEXT NOT NULL DEFAULT 'manual',
  max_concurrency INTEGER NOT NULL DEFAULT 1,
  next_run_at TEXT NOT NULL DEFAULT '',
  last_run_at TEXT NOT NULL DEFAULT '',
  running INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_due ON cron_jobs(assistant_id, enabled, running, next_run_at);

CREATE TABLE IF NOT EXISTS cron_runs (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL,
  assistant_id TEXT NOT NULL,
  status TEXT NOT NULL,
  manual INTEGER NOT NULL DEFAULT 0,
  due_at TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT '',
  local_session_id TEXT NOT NULL DEFAULT '',
  acp_session_id TEXT NOT NULL DEFAULT '',
  external_session_id TEXT NOT NULL DEFAULT '',
  final_text TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_cron_runs_job_started ON cron_runs(assistant_id, job_id, started_at, id);
CREATE INDEX IF NOT EXISTS idx_cron_runs_status ON cron_runs(assistant_id, status, started_at);
