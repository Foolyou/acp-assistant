CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  assistant_id TEXT NOT NULL,
  type TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  data_json TEXT NOT NULL DEFAULT '{}',
  at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_assistant_at ON events(assistant_id, at, id);
CREATE INDEX IF NOT EXISTS idx_events_assistant_type_at ON events(assistant_id, type, at);

CREATE TABLE IF NOT EXISTS connector_status (
  assistant_id TEXT NOT NULL,
  platform TEXT NOT NULL,
  account_id TEXT NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (assistant_id, platform, account_id)
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  assistant_id TEXT NOT NULL,
  platform TEXT NOT NULL,
  account_id TEXT NOT NULL,
  private_channel_id TEXT NOT NULL,
  platform_user_id TEXT NOT NULL,
  conversation_key TEXT NOT NULL DEFAULT '',
  thread_key TEXT NOT NULL DEFAULT '',
  acp_session_id TEXT NOT NULL DEFAULT '',
  external_session_id TEXT NOT NULL DEFAULT '',
  permission_mode TEXT NOT NULL,
  launch_profile_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_binding ON sessions(
  assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key
);

CREATE TABLE IF NOT EXISTS bindings (
  assistant_id TEXT NOT NULL,
  platform TEXT NOT NULL,
  account_id TEXT NOT NULL,
  private_channel_id TEXT NOT NULL,
  platform_user_id TEXT NOT NULL,
  conversation_key TEXT NOT NULL DEFAULT '',
  thread_key TEXT NOT NULL DEFAULT '',
  active_session_id TEXT NOT NULL,
  default_permission_mode TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key)
);

CREATE TABLE IF NOT EXISTS acp_mappings (
  assistant_id TEXT NOT NULL,
  local_session_id TEXT NOT NULL,
  harness_provider TEXT NOT NULL,
  launch_profile_key TEXT NOT NULL,
  acp_session_id TEXT NOT NULL,
  external_session_id TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (assistant_id, local_session_id, harness_provider, launch_profile_key)
);

CREATE TABLE IF NOT EXISTS permissions (
  id TEXT PRIMARY KEY,
  local_session_id TEXT NOT NULL,
  assistant_id TEXT NOT NULL,
  platform TEXT NOT NULL,
  account_id TEXT NOT NULL,
  private_channel_id TEXT NOT NULL,
  platform_user_id TEXT NOT NULL,
  conversation_key TEXT NOT NULL DEFAULT '',
  thread_key TEXT NOT NULL DEFAULT '',
  acp_request_id TEXT NOT NULL,
  options_json TEXT NOT NULL DEFAULT '[]',
  short_approval_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  resolved_option TEXT NOT NULL DEFAULT '',
  timeout_resolution TEXT NOT NULL DEFAULT 'reject',
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  resolved_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_permissions_owner ON permissions(
  assistant_id, platform, account_id, private_channel_id, platform_user_id, status
);

CREATE TABLE IF NOT EXISTS memory_revisions (
  id TEXT PRIMARY KEY,
  assistant_id TEXT NOT NULL,
  target TEXT NOT NULL,
  revision INTEGER NOT NULL,
  origin TEXT NOT NULL,
  actor_id TEXT NOT NULL DEFAULT '',
  content_path TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (assistant_id, target, revision)
);

CREATE INDEX IF NOT EXISTS idx_memory_revisions_target ON memory_revisions(assistant_id, target, revision);

CREATE TABLE IF NOT EXISTS errors (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  assistant_id TEXT NOT NULL,
  source TEXT NOT NULL,
  message TEXT NOT NULL,
  at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  assistant_id TEXT NOT NULL,
  platform TEXT NOT NULL,
  account_id TEXT NOT NULL,
  key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (assistant_id, platform, account_id, key)
);
