package cron_test

import (
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/cron"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestNextRunForAtEveryAndCronSchedules(t *testing.T) {
	now := time.Date(2026, 5, 23, 8, 15, 0, 0, time.UTC)

	atNext, err := cron.NextRun(model.CronScheduleTypeAt, "2026-05-23T09:30:00Z", "UTC", now, time.Time{})
	if err != nil {
		t.Fatalf("at next run: %v", err)
	}
	if !atNext.Equal(time.Date(2026, 5, 23, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected at next run: %s", atNext)
	}

	everyNext, err := cron.NextRun(model.CronScheduleTypeEvery, "15m", "UTC", now, now)
	if err != nil {
		t.Fatalf("every next run: %v", err)
	}
	if !everyNext.Equal(time.Date(2026, 5, 23, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected every next run: %s", everyNext)
	}

	cronNext, err := cron.NextRun(model.CronScheduleTypeCron, "*/20 9-10 * * 1-5", "Asia/Shanghai", now, time.Time{})
	if err != nil {
		t.Fatalf("cron next run: %v", err)
	}
	if !cronNext.Equal(time.Date(2026, 5, 25, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected cron next run: %s", cronNext)
	}
}

func TestNextRunRejectsUnsupportedSchedules(t *testing.T) {
	if _, err := cron.NextRun(model.CronScheduleTypeEvery, "0s", "UTC", time.Now(), time.Time{}); err == nil {
		t.Fatal("expected zero interval to be rejected")
	}
	if _, err := cron.NextRun(model.CronScheduleTypeCron, "@daily", "UTC", time.Now(), time.Time{}); err == nil {
		t.Fatal("expected macro cron expression to be rejected")
	}
	if _, err := cron.NextRun(model.CronScheduleType("later"), "1h", "UTC", time.Now(), time.Time{}); err == nil {
		t.Fatal("expected unknown schedule type to be rejected")
	}
}
