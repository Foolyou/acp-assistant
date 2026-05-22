package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

func TestStoreCreatesClaimsAndCompletesCronRuns(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	next := due.Add(time.Hour)
	owner := model.SessionBindingKey{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "owner",
	}

	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:    "alpha",
		Name:           "hourly report",
		Enabled:        true,
		ScheduleType:   model.CronScheduleTypeEvery,
		ScheduleExpr:   "1h",
		Timezone:       "UTC",
		Prompt:         "summarize workspace",
		Target:         model.CronTargetIsolated,
		DeliveryMode:   model.CronDeliveryOrigin,
		Creator:        owner,
		PermissionMode: model.PermissionManual,
		NextRunAt:      due,
	})
	if err != nil {
		t.Fatalf("create cron job: %v", err)
	}
	if job.ID == "" || !job.Enabled {
		t.Fatalf("unexpected created job: %#v", job)
	}

	claims, err := db.ClaimDueCronRuns(ctx, "alpha", due, 10)
	if err != nil {
		t.Fatalf("claim due runs: %v", err)
	}
	if len(claims) != 1 || claims[0].Job.ID != job.ID || !claims[0].StartedAt.Equal(due) {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	again, err := db.ClaimDueCronRuns(ctx, "alpha", due, 10)
	if err != nil {
		t.Fatalf("claim again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("running job should not be claimed again: %#v", again)
	}

	completed, err := db.CompleteCronRun(ctx, claims[0].ID, model.CronRunStatusSucceeded, "ok", "", "ses_1", "acp_1", "external_1", &next)
	if err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if completed.Status != model.CronRunStatusSucceeded || completed.FinalText != "ok" {
		t.Fatalf("unexpected completed run: %#v", completed)
	}
	loaded, err := db.CronJob(ctx, "alpha", job.ID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if !loaded.NextRunAt.Equal(next) || loaded.Running {
		t.Fatalf("job was not advanced and unlocked: %#v", loaded)
	}
	runs, err := db.RecentCronRuns(ctx, "alpha", job.ID, 5)
	if err != nil {
		t.Fatalf("recent runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != completed.ID {
		t.Fatalf("unexpected recent runs: %#v", runs)
	}
}

func TestStoreDisablesOneTimeCronJobWhenNextRunCleared(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "one shot",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeAt,
		ScheduleExpr: "2026-05-23T08:00:00Z",
		Timezone:     "UTC",
		Prompt:       "ping",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryNone,
		NextRunAt:    due,
	})
	if err != nil {
		t.Fatalf("create cron job: %v", err)
	}
	claims, err := db.ClaimDueCronRuns(ctx, "alpha", due, 10)
	if err != nil {
		t.Fatalf("claim due runs: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected one claim, got %#v", claims)
	}
	if _, err := db.CompleteCronRun(ctx, claims[0].ID, model.CronRunStatusSucceeded, "ok", "", "", "", "", nil); err != nil {
		t.Fatalf("complete run: %v", err)
	}
	loaded, err := db.CronJob(ctx, "alpha", job.ID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if loaded.Enabled || !loaded.NextRunAt.IsZero() {
		t.Fatalf("one-time job should be disabled and cleared: %#v", loaded)
	}
}
