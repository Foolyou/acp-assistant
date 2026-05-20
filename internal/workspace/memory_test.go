package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
	"github.com/Foolyou/acp-assistant/internal/workspace"
)

func TestMemoryManagerUpdatesOnlyConfiguredTargetsAndRollsBack(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(filepath.Join(root, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	manager := workspace.NewMemoryManager("alpha", filepath.Join(root, "workspace"), model.DefaultMemoryConfig(), db)
	if err := manager.InitSkeletons(); err != nil {
		t.Fatalf("init skeletons: %v", err)
	}
	first, err := manager.Update(ctx, model.MemoryUpdate{
		Target:  "memory/facts.md",
		Content: "first\n",
		Origin:  model.MemoryOriginUser,
		ActorID: "user-a",
	})
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	second, err := manager.Update(ctx, model.MemoryUpdate{
		Target:  "memory/facts.md",
		Content: "second\n",
		Origin:  model.MemoryOriginHarness,
		ActorID: "session-a",
	})
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if second.Revision <= first.Revision {
		t.Fatalf("revision did not increase: first=%d second=%d", first.Revision, second.Revision)
	}
	if _, err := manager.Update(ctx, model.MemoryUpdate{Target: "../outside.md", Content: "bad"}); err == nil {
		t.Fatal("expected invalid target to be rejected")
	}
	if err := manager.Rollback(ctx, "memory/facts.md", first.Revision, "user-a"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "workspace", "memory", "facts.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "first\n" {
		t.Fatalf("unexpected rollback content: %q", string(content))
	}
}
