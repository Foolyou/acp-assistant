package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
)

func NextRun(kind model.CronScheduleType, expr, timezone string, now, last time.Time) (time.Time, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	now = now.UTC()
	switch kind {
	case model.CronScheduleTypeAt:
		return nextAt(expr, loc, now)
	case model.CronScheduleTypeEvery:
		return nextEvery(expr, now, last)
	case model.CronScheduleTypeCron:
		return nextCron(expr, loc, now)
	default:
		return time.Time{}, fmt.Errorf("unsupported schedule type %q", kind)
	}
}

func nextAt(expr string, loc *time.Location, now time.Time) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return time.Time{}, fmt.Errorf("at schedule requires a time")
	}
	if d, ok := RelativeDuration(expr); ok {
		t := now.Add(d).UTC()
		if !t.After(now) {
			return time.Time{}, fmt.Errorf("at schedule must be in the future")
		}
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, expr); err == nil {
		t = t.UTC()
		if !t.After(now) {
			return time.Time{}, fmt.Errorf("at schedule must be in the future")
		}
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, expr, loc); err == nil {
			t = t.UTC()
			if !t.After(now) {
				return time.Time{}, fmt.Errorf("at schedule must be in the future")
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid at schedule %q", expr)
}

func RelativeDuration(expr string) (time.Duration, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, false
	}
	d, err := time.ParseDuration(expr)
	if err != nil {
		return 0, false
	}
	return d, true
}

func nextEvery(expr string, now, last time.Time) (time.Time, error) {
	d, err := time.ParseDuration(strings.TrimSpace(expr))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid every schedule: %w", err)
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("every schedule must be positive")
	}
	base := now
	if !last.IsZero() {
		base = last.UTC()
	}
	next := base.Add(d)
	for !next.After(now) {
		next = next.Add(d)
	}
	return next.UTC(), nil
}

func nextCron(expr string, loc *time.Location, now time.Time) (time.Time, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("cron schedule requires five fields")
	}
	minutes, err := parseCronField(parts[0], 0, 59, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, err := parseCronField(parts[1], 0, 23, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour field: %w", err)
	}
	days, err := parseCronField(parts[2], 1, 31, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	months, err := parseCronField(parts[3], 1, 12, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month field: %w", err)
	}
	weekdays, err := parseCronField(parts[4], 0, 7, true)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	cursor := now.In(loc).Truncate(time.Minute).Add(time.Minute)
	deadline := cursor.AddDate(5, 0, 0)
	for cursor.Before(deadline) {
		if minutes[cursor.Minute()] &&
			hours[cursor.Hour()] &&
			days[cursor.Day()] &&
			months[int(cursor.Month())] &&
			weekdays[int(cursor.Weekday())] {
			return cursor.UTC(), nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("cron schedule has no matching time within five years")
}

func parseCronField(raw string, min, max int, sundayAlias bool) (map[int]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty field")
	}
	out := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty list item")
		}
		step := 1
		base := part
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			base = pieces[0]
			n, err := strconv.Atoi(pieces[1])
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid step %q", pieces[1])
			}
			step = n
		}
		start, end, err := cronRange(base, min, max)
		if err != nil {
			return nil, err
		}
		for i := start; i <= end; i += step {
			if sundayAlias && i == 7 {
				out[0] = true
				continue
			}
			out[i] = true
		}
	}
	return out, nil
}

func cronRange(raw string, min, max int) (int, int, error) {
	if raw == "*" {
		return min, max, nil
	}
	if strings.Contains(raw, "-") {
		pieces := strings.Split(raw, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", raw)
		}
		start, err := strconv.Atoi(pieces[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range start %q", pieces[0])
		}
		end, err := strconv.Atoi(pieces[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range end %q", pieces[1])
		}
		if start > end || start < min || end > max {
			return 0, 0, fmt.Errorf("range %q out of bounds", raw)
		}
		return start, end, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid value %q", raw)
	}
	if value < min || value > max {
		return 0, 0, fmt.Errorf("value %q out of bounds", raw)
	}
	return value, value, nil
}
