package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store/migrations"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("store path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY NOT NULL)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		data, err := migrations.FS.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(name) VALUES (?)`, entry.Name()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecordEvent(ctx context.Context, event model.Event) error {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	data := "{}"
	if event.Data != nil {
		encoded, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		data = string(encoded)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events(assistant_id, type, scope, message, data_json, at) VALUES (?, ?, ?, ?, ?, ?)`,
		event.AssistantID, string(event.Type), event.Scope, event.Message, data, encodeTime(event.At))
	return err
}

func (s *Store) RecentEvents(ctx context.Context, assistantID string, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, assistant_id, type, scope, message, data_json, at FROM events WHERE assistant_id = ? ORDER BY at DESC, id DESC LIMIT ?`, assistantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (s *Store) EventsAfter(ctx context.Context, assistantID string, afterID int64, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, assistant_id, type, scope, message, data_json, at FROM events WHERE assistant_id = ? AND id > ? ORDER BY id ASC LIMIT ?`, assistantID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (s *Store) UpsertConnectorStatus(ctx context.Context, status model.ConnectorStatus) error {
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO connector_status(assistant_id, platform, account_id, state, message, last_error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(assistant_id, platform, account_id) DO UPDATE SET
		  state = excluded.state,
		  message = excluded.message,
		  last_error = excluded.last_error,
		  updated_at = excluded.updated_at`,
		status.AssistantID, string(status.Platform), status.AccountID, string(status.State), status.Message, status.LastError, encodeTime(status.UpdatedAt))
	if err != nil {
		return err
	}
	return s.RecordEvent(ctx, model.Event{
		AssistantID: status.AssistantID,
		Type:        model.EventConnector,
		Scope:       string(status.Platform) + "/" + status.AccountID,
		Message:     string(status.State),
		At:          status.UpdatedAt,
		Data: map[string]any{
			"platform":   status.Platform,
			"account_id": status.AccountID,
			"last_error": status.LastError,
		},
	})
}

func (s *Store) ConnectorStatuses(ctx context.Context, assistantID string) ([]model.ConnectorStatus, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT assistant_id, platform, account_id, state, message, last_error, updated_at FROM connector_status WHERE assistant_id = ? ORDER BY platform, account_id`, assistantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ConnectorStatus
	for rows.Next() {
		var status model.ConnectorStatus
		var platform, state, updated string
		if err := rows.Scan(&status.AssistantID, &platform, &status.AccountID, &state, &status.Message, &status.LastError, &updated); err != nil {
			return nil, err
		}
		status.Platform = model.Platform(platform)
		status.State = model.ConnectorState(state)
		status.UpdatedAt = decodeTime(updated)
		out = append(out, status)
	}
	return out, rows.Err()
}

func (s *Store) RememberIdempotency(ctx context.Context, assistantID string, platform model.Platform, accountID, key string) (bool, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO idempotency_keys(assistant_id, platform, account_id, key, created_at) VALUES (?, ?, ?, ?, ?)`,
		assistantID, string(platform), accountID, key, encodeTime(time.Now().UTC()))
	if err != nil {
		if strings.Contains(err.Error(), "constraint") {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (s *Store) CreateSession(ctx context.Context, key model.SessionBindingKey, mode model.PermissionMode, profileKey string) (model.LocalSession, error) {
	now := time.Now().UTC()
	session := model.LocalSession{
		ID:               randomID("ses"),
		Binding:          key,
		PermissionMode:   mode,
		LaunchProfileKey: profileKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LocalSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, permission_mode, launch_profile_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey, string(mode), profileKey, encodeTime(now), encodeTime(now)); err != nil {
		_ = tx.Rollback()
		return model.LocalSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bindings(assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, active_session_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key) DO UPDATE SET
		  active_session_id = excluded.active_session_id,
		  updated_at = excluded.updated_at`,
		key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey, session.ID, encodeTime(now)); err != nil {
		_ = tx.Rollback()
		return model.LocalSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.LocalSession{}, err
	}
	_ = s.RecordEvent(ctx, model.Event{AssistantID: key.AssistantID, Type: model.EventSession, Scope: session.ID, Message: "created", At: now})
	return session, nil
}

func (s *Store) ActiveSessionForBinding(ctx context.Context, key model.SessionBindingKey) (model.LocalSession, error) {
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.assistant_id, s.platform, s.account_id, s.private_channel_id, s.platform_user_id, s.conversation_key, s.thread_key, s.acp_session_id, s.external_session_id, s.permission_mode, s.launch_profile_key, s.created_at, s.updated_at
		FROM bindings b JOIN sessions s ON s.id = b.active_session_id
		WHERE b.assistant_id = ? AND b.platform = ? AND b.account_id = ? AND b.private_channel_id = ? AND b.platform_user_id = ? AND b.conversation_key = ? AND b.thread_key = ?`,
		key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey)
	return scanSession(row)
}

func (s *Store) SessionByID(ctx context.Context, id string) (model.LocalSession, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, acp_session_id, external_session_id, permission_mode, launch_profile_key, created_at, updated_at FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *Store) SetActiveSession(ctx context.Context, key model.SessionBindingKey, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bindings SET active_session_id = ?, updated_at = ? WHERE assistant_id = ? AND platform = ? AND account_id = ? AND private_channel_id = ? AND platform_user_id = ? AND conversation_key = ? AND thread_key = ?`,
		sessionID, encodeTime(time.Now().UTC()), key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey)
	return err
}

func (s *Store) SetBindingDefaultMode(ctx context.Context, key model.SessionBindingKey, mode model.PermissionMode) error {
	now := encodeTime(time.Now().UTC())
	_, err := s.db.ExecContext(ctx, `INSERT INTO bindings(assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, active_session_id, default_permission_mode, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)
		ON CONFLICT(assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key) DO UPDATE SET
		  default_permission_mode = excluded.default_permission_mode,
		  updated_at = excluded.updated_at`,
		key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey, string(mode), now)
	return err
}

func (s *Store) BindingDefaultMode(ctx context.Context, key model.SessionBindingKey) (model.PermissionMode, error) {
	var mode string
	err := s.db.QueryRowContext(ctx, `SELECT default_permission_mode FROM bindings WHERE assistant_id = ? AND platform = ? AND account_id = ? AND private_channel_id = ? AND platform_user_id = ? AND conversation_key = ? AND thread_key = ?`,
		key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey).Scan(&mode)
	return model.PermissionMode(mode), err
}

func (s *Store) ListSessionsForBinding(ctx context.Context, key model.SessionBindingKey) ([]model.LocalSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, acp_session_id, external_session_id, permission_mode, launch_profile_key, created_at, updated_at
		FROM sessions WHERE assistant_id = ? AND platform = ? AND account_id = ? AND private_channel_id = ? AND platform_user_id = ? AND conversation_key = ? AND thread_key = ?
		ORDER BY created_at DESC`,
		key.AssistantID, string(key.Platform), key.AccountID, key.PrivateChannelID, key.PlatformUserID, key.ConversationKey, key.ThreadKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.LocalSession
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) UpdateSessionACP(ctx context.Context, sessionID, acpSessionID, externalSessionID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET acp_session_id = ?, external_session_id = ?, updated_at = ? WHERE id = ?`,
		acpSessionID, externalSessionID, encodeTime(time.Now().UTC()), sessionID)
	return err
}

func (s *Store) UpdateSessionMode(ctx context.Context, sessionID string, mode model.PermissionMode, profileKey string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET permission_mode = ?, launch_profile_key = ?, updated_at = ? WHERE id = ?`,
		string(mode), profileKey, encodeTime(time.Now().UTC()), sessionID)
	return err
}

func (s *Store) CreatePermission(ctx context.Context, permission model.PendingPermission) (model.PendingPermission, error) {
	now := time.Now().UTC()
	permission.ID = randomID("perm")
	if permission.ShortApprovalID == "" {
		permission.ShortApprovalID = strings.ToUpper(randomID(""))[:6]
	}
	if permission.CreatedAt.IsZero() {
		permission.CreatedAt = now
	}
	if permission.ExpiresAt.IsZero() {
		permission.ExpiresAt = now.Add(10 * time.Minute)
	}
	if permission.Status == "" {
		permission.Status = "pending"
	}
	if permission.TimeoutResolution == "" {
		permission.TimeoutResolution = "reject"
	}
	options, err := json.Marshal(permission.Options)
	if err != nil {
		return model.PendingPermission{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO permissions(id, local_session_id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, acp_request_id, options_json, short_approval_id, status, timeout_resolution, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		permission.ID, permission.LocalSessionID, permission.Owner.AssistantID, string(permission.Owner.Platform), permission.Owner.AccountID, permission.Owner.PrivateChannelID, permission.Owner.PlatformUserID, permission.Owner.ConversationKey, permission.Owner.ThreadKey, permission.ACPRequestID, string(options), permission.ShortApprovalID, permission.Status, permission.TimeoutResolution, encodeTime(permission.CreatedAt), encodeTime(permission.ExpiresAt))
	if err != nil {
		return model.PendingPermission{}, err
	}
	_ = s.RecordEvent(ctx, model.Event{AssistantID: permission.Owner.AssistantID, Type: model.EventPermission, Scope: permission.LocalSessionID, Message: "pending", At: now, Data: map[string]any{"short_id": permission.ShortApprovalID}})
	return permission, nil
}

func (s *Store) PermissionByShortID(ctx context.Context, shortID string) (model.PendingPermission, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, local_session_id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, acp_request_id, options_json, short_approval_id, status, resolved_option, timeout_resolution, created_at, expires_at, resolved_at FROM permissions WHERE short_approval_id = ?`, shortID)
	return scanPermission(row)
}

func (s *Store) ResolvePermission(ctx context.Context, shortID string, owner model.SessionBindingKey, option string) (model.PendingPermission, error) {
	permission, err := s.PermissionByShortID(ctx, shortID)
	if err != nil {
		return model.PendingPermission{}, err
	}
	if permission.Owner != owner {
		return model.PendingPermission{}, fmt.Errorf("permission %s belongs to a different owner", shortID)
	}
	if permission.Status != "pending" {
		return model.PendingPermission{}, fmt.Errorf("permission %s is not pending", shortID)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE permissions SET status = 'resolved', resolved_option = ?, resolved_at = ? WHERE short_approval_id = ?`,
		option, encodeTime(now), shortID)
	if err != nil {
		return model.PendingPermission{}, err
	}
	permission.Status = "resolved"
	permission.ResolvedOption = option
	permission.ResolvedAt = &now
	_ = s.RecordEvent(ctx, model.Event{AssistantID: owner.AssistantID, Type: model.EventPermission, Scope: permission.LocalSessionID, Message: "resolved", At: now, Data: map[string]any{"short_id": shortID, "option": option}})
	return permission, nil
}

func (s *Store) ExpirePermissions(ctx context.Context, now time.Time) ([]model.PendingPermission, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, local_session_id, assistant_id, platform, account_id, private_channel_id, platform_user_id, conversation_key, thread_key, acp_request_id, options_json, short_approval_id, status, resolved_option, timeout_resolution, created_at, expires_at, resolved_at
		FROM permissions WHERE status = 'pending' AND expires_at <= ?`, encodeTime(now.UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var expired []model.PendingPermission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		expired = append(expired, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, permission := range expired {
		_, err := s.db.ExecContext(ctx, `UPDATE permissions SET status = 'expired', resolved_option = ?, resolved_at = ? WHERE id = ?`,
			permission.TimeoutResolution, encodeTime(now.UTC()), permission.ID)
		if err != nil {
			return nil, err
		}
	}
	return expired, nil
}

func (s *Store) RecordMemoryRevision(ctx context.Context, revision model.MemoryRevision) (model.MemoryRevision, error) {
	if revision.ID == "" {
		revision.ID = randomID("mem")
	}
	if revision.CreatedAt.IsZero() {
		revision.CreatedAt = time.Now().UTC()
	}
	if revision.Revision == 0 {
		next, err := s.NextMemoryRevision(ctx, revision.AssistantID, revision.Target)
		if err != nil {
			return model.MemoryRevision{}, err
		}
		revision.Revision = next
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_revisions(id, assistant_id, target, revision, origin, actor_id, content_path, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.ID, revision.AssistantID, revision.Target, revision.Revision, string(revision.Origin), revision.ActorID, revision.ContentPath, encodeTime(revision.CreatedAt))
	if err != nil {
		return model.MemoryRevision{}, err
	}
	_ = s.RecordEvent(ctx, model.Event{AssistantID: revision.AssistantID, Type: model.EventMemory, Scope: revision.Target, Message: "revision", At: revision.CreatedAt, Data: map[string]any{"revision": revision.Revision, "origin": revision.Origin}})
	return revision, nil
}

func (s *Store) NextMemoryRevision(ctx context.Context, assistantID, target string) (int64, error) {
	var current sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(revision) FROM memory_revisions WHERE assistant_id = ? AND target = ?`, assistantID, target).Scan(&current); err != nil {
		return 0, err
	}
	if !current.Valid {
		return 1, nil
	}
	return current.Int64 + 1, nil
}

func (s *Store) MemoryRevision(ctx context.Context, assistantID, target string, revision int64) (model.MemoryRevision, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, assistant_id, target, revision, origin, actor_id, content_path, created_at FROM memory_revisions WHERE assistant_id = ? AND target = ? AND revision = ?`, assistantID, target, revision)
	return scanMemoryRevision(row)
}

func (s *Store) RecentMemoryRevisions(ctx context.Context, assistantID string, limit int) ([]model.MemoryRevision, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, assistant_id, target, revision, origin, actor_id, content_path, created_at FROM memory_revisions WHERE assistant_id = ? ORDER BY created_at DESC, revision DESC LIMIT ?`, assistantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MemoryRevision
	for rows.Next() {
		revision, err := scanMemoryRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, revision)
	}
	return out, rows.Err()
}

func (s *Store) StatusSnapshot(ctx context.Context, assistantID string) (model.StatusSnapshot, error) {
	connectors, err := s.ConnectorStatuses(ctx, assistantID)
	if err != nil {
		return model.StatusSnapshot{}, err
	}
	var activeSessions int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bindings WHERE assistant_id = ? AND active_session_id != ''`, assistantID).Scan(&activeSessions); err != nil {
		return model.StatusSnapshot{}, err
	}
	var pending int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM permissions WHERE assistant_id = ? AND status = 'pending'`, assistantID).Scan(&pending); err != nil {
		return model.StatusSnapshot{}, err
	}
	errors, err := s.eventsByType(ctx, assistantID, model.EventError, 10)
	if err != nil {
		return model.StatusSnapshot{}, err
	}
	revisions, err := s.RecentMemoryRevisions(ctx, assistantID, 10)
	if err != nil {
		return model.StatusSnapshot{}, err
	}
	events, err := s.RecentEvents(ctx, assistantID, 1)
	if err != nil {
		return model.StatusSnapshot{}, err
	}
	var last *model.Event
	if len(events) > 0 {
		last = &events[0]
	}
	return model.StatusSnapshot{AssistantID: assistantID, LastEvent: last, Connectors: connectors, ActiveSessions: activeSessions, PendingPermissions: pending, RecentErrors: errors, MemoryRevisions: revisions}, nil
}

func (s *Store) eventsByType(ctx context.Context, assistantID string, eventType model.EventType, limit int) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, assistant_id, type, scope, message, data_json, at FROM events WHERE assistant_id = ? AND type = ? ORDER BY at DESC, id DESC LIMIT ?`, assistantID, string(eventType), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEvents(rows *sql.Rows) ([]model.Event, error) {
	var out []model.Event
	for rows.Next() {
		var event model.Event
		var kind, data, at string
		if err := rows.Scan(&event.ID, &event.AssistantID, &kind, &event.Scope, &event.Message, &data, &at); err != nil {
			return nil, err
		}
		event.Type = model.EventType(kind)
		event.At = decodeTime(at)
		if data != "" {
			_ = json.Unmarshal([]byte(data), &event.Data)
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func scanSession(row scanner) (model.LocalSession, error) {
	var session model.LocalSession
	var platform, mode, created, updated string
	if err := row.Scan(&session.ID, &session.Binding.AssistantID, &platform, &session.Binding.AccountID, &session.Binding.PrivateChannelID, &session.Binding.PlatformUserID, &session.Binding.ConversationKey, &session.Binding.ThreadKey, &session.ACPSessionID, &session.ExternalSessionID, &mode, &session.LaunchProfileKey, &created, &updated); err != nil {
		return model.LocalSession{}, err
	}
	session.Binding.Platform = model.Platform(platform)
	session.PermissionMode = model.PermissionMode(mode)
	session.CreatedAt = decodeTime(created)
	session.UpdatedAt = decodeTime(updated)
	return session, nil
}

func scanPermission(row scanner) (model.PendingPermission, error) {
	var permission model.PendingPermission
	var platform, options, created, expires, resolved string
	if err := row.Scan(&permission.ID, &permission.LocalSessionID, &permission.Owner.AssistantID, &platform, &permission.Owner.AccountID, &permission.Owner.PrivateChannelID, &permission.Owner.PlatformUserID, &permission.Owner.ConversationKey, &permission.Owner.ThreadKey, &permission.ACPRequestID, &options, &permission.ShortApprovalID, &permission.Status, &permission.ResolvedOption, &permission.TimeoutResolution, &created, &expires, &resolved); err != nil {
		return model.PendingPermission{}, err
	}
	permission.Owner.Platform = model.Platform(platform)
	_ = json.Unmarshal([]byte(options), &permission.Options)
	permission.CreatedAt = decodeTime(created)
	permission.ExpiresAt = decodeTime(expires)
	if resolved != "" {
		t := decodeTime(resolved)
		permission.ResolvedAt = &t
	}
	return permission, nil
}

func scanMemoryRevision(row scanner) (model.MemoryRevision, error) {
	var revision model.MemoryRevision
	var origin, created string
	if err := row.Scan(&revision.ID, &revision.AssistantID, &revision.Target, &revision.Revision, &origin, &revision.ActorID, &revision.ContentPath, &created); err != nil {
		return model.MemoryRevision{}, err
	}
	revision.Origin = model.MemoryOrigin(origin)
	revision.CreatedAt = decodeTime(created)
	return revision, nil
}

func encodeTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func decodeTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func randomID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s%x", prefix, time.Now().UnixNano())
	}
	if prefix == "" {
		return hex.EncodeToString(bytes[:])
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
