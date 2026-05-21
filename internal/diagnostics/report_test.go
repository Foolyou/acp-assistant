package diagnostics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReportAggregatesSeverityAndRecommendations(t *testing.T) {
	report := NewReport(time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC))
	report.AddCheck(Check{ID: "pass", Title: "Pass", Severity: SeverityPass, Message: "ok"})
	report.AddCheck(Check{ID: "warn", Title: "Warn", Severity: SeverityWarn, Message: "careful", Recommendation: "review warning"})
	if report.Severity != SeverityWarn {
		t.Fatalf("expected warn severity, got %s", report.Severity)
	}
	report.AddCheck(Check{ID: "fail", Title: "Fail", Severity: SeverityFail, Message: "broken", Recommendation: "fix failure"})
	report.Finalize()
	if report.Severity != SeverityFail {
		t.Fatalf("expected fail severity, got %s", report.Severity)
	}
	if len(report.Recommendations) != 2 {
		t.Fatalf("expected recommendations for warning and failure, got %#v", report.Recommendations)
	}
}

func TestRenderTextAndJSONUseStructuredReport(t *testing.T) {
	report := NewReport(time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC))
	report.AssistantID = "alpha"
	report.AddCheck(Check{ID: "config", Title: "Config", Severity: SeverityPass, Message: "loaded", Details: map[string]any{"path": "/tmp/config"}})
	report.AddCheck(Check{ID: "command", Title: "Command", Severity: SeverityFail, Message: "not found", Recommendation: "install command"})

	var text bytes.Buffer
	if err := RenderText(&text, report, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "doctor: FAIL") || !strings.Contains(text.String(), "path: /tmp/config") || !strings.Contains(text.String(), "install command") {
		t.Fatalf("unexpected verbose text:\n%s", text.String())
	}

	var raw bytes.Buffer
	if err := RenderJSON(&raw, report); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(raw.Bytes(), &decoded); err != nil {
		t.Fatalf("json output did not decode: %v\n%s", err, raw.String())
	}
	if decoded.Severity != SeverityFail || len(decoded.Checks) != 2 || decoded.AssistantID != "alpha" {
		t.Fatalf("unexpected json report: %#v", decoded)
	}
}
