package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

func TestStoreMigratesAndRecordsEventsStatusSessionsAndPermissions(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.RecordEvent(ctx, model.Event{
		AssistantID: "alpha",
		Type:        model.EventLifecycle,
		Scope:       "assistant",
		Message:     "started",
		At:          time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record event: %v", err)
	}
	if err := db.UpsertConnectorStatus(ctx, model.ConnectorStatus{
		AssistantID: "alpha",
		Platform:    model.PlatformFeishu,
		AccountID:   "main",
		State:       model.ConnectorStateConnected,
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert status: %v", err)
	}
	if duplicate, err := db.RememberIdempotency(ctx, "alpha", model.PlatformFeishu, "main", "message-1"); err != nil || duplicate {
		t.Fatalf("first idempotency unexpected duplicate=%v err=%v", duplicate, err)
	}
	if duplicate, err := db.RememberIdempotency(ctx, "alpha", model.PlatformFeishu, "main", "message-1"); err != nil || !duplicate {
		t.Fatalf("second idempotency duplicate=%v err=%v", duplicate, err)
	}

	binding := model.SessionBindingKey{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
	}
	session, err := db.CreateSession(ctx, binding, model.PermissionManual, "manual")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	active, err := db.ActiveSessionForBinding(ctx, binding)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active.ID != session.ID || active.PermissionMode != model.PermissionManual {
		t.Fatalf("unexpected active session: %#v", active)
	}

	permission, err := db.CreatePermission(ctx, model.PendingPermission{
		LocalSessionID:    session.ID,
		Owner:             binding,
		ACPRequestID:      "req-1",
		Options:           []string{"approve", "reject"},
		ShortApprovalID:   "A1B2",
		ExpiresAt:         time.Now().UTC().Add(time.Minute),
		TimeoutResolution: "reject",
	})
	if err != nil {
		t.Fatalf("create permission: %v", err)
	}
	resolved, err := db.ResolvePermission(ctx, permission.ShortApprovalID, binding, "approve")
	if err != nil {
		t.Fatalf("resolve permission: %v", err)
	}
	if resolved.ResolvedOption != "approve" {
		t.Fatalf("unexpected resolved permission: %#v", resolved)
	}

	status, err := db.StatusSnapshot(ctx, "alpha")
	if err != nil {
		t.Fatalf("status snapshot: %v", err)
	}
	if len(status.Connectors) != 1 || status.ActiveSessions != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
}
