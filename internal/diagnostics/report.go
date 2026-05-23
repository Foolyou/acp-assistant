package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
)

type Severity string

const (
	SeverityPass Severity = "pass"
	SeverityWarn Severity = "warn"
	SeverityFail Severity = "fail"
)

type Report struct {
	AssistantID     string              `json:"assistant_id,omitempty"`
	AssistantName   string              `json:"assistant_name,omitempty"`
	AssistantHome   string              `json:"assistant_home,omitempty"`
	ConfigspacePath string              `json:"configspace_path,omitempty"`
	WorkspacePath   string              `json:"workspace_path,omitempty"`
	EventDBPath     string              `json:"event_db_path,omitempty"`
	GeneratedAt     time.Time           `json:"generated_at"`
	Severity        Severity            `json:"severity"`
	Checks          []Check             `json:"checks"`
	Recommendations []Recommendation    `json:"recommendations,omitempty"`
	Status          *StatusSummary      `json:"status,omitempty"`
	RecentErrors    []model.Event       `json:"recent_errors,omitempty"`
	LogSnippets     []LogSnippet        `json:"log_snippets,omitempty"`
	Connectors      []ConnectorSnapshot `json:"connectors,omitempty"`
	Harness         *HarnessSnapshot    `json:"harness,omitempty"`
	Process         *ProcessSnapshot    `json:"process,omitempty"`
}

type Check struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Severity       Severity       `json:"severity"`
	Message        string         `json:"message"`
	Details        map[string]any `json:"details,omitempty"`
	Recommendation string         `json:"recommendation,omitempty"`
}

type Recommendation struct {
	CheckID string   `json:"check_id,omitempty"`
	Message string   `json:"message"`
	Actions []string `json:"actions,omitempty"`
}

type StatusSummary struct {
	ActiveSessions     int `json:"active_sessions"`
	PendingPermissions int `json:"pending_permissions"`
	RecentErrorCount   int `json:"recent_error_count"`
}

type ConnectorSnapshot struct {
	ID          string               `json:"id"`
	Platform    model.Platform       `json:"platform"`
	AccountID   string               `json:"account_id"`
	Enabled     bool                 `json:"enabled"`
	State       model.ConnectorState `json:"state,omitempty"`
	Message     string               `json:"message,omitempty"`
	LastError   string               `json:"last_error,omitempty"`
	UpdatedAt   time.Time            `json:"updated_at,omitempty"`
	Credentials []string             `json:"credentials,omitempty"`
	Options     map[string]string    `json:"options,omitempty"`
}

type HarnessSnapshot struct {
	Provider       model.HarnessProvider `json:"provider"`
	PermissionMode model.PermissionMode  `json:"permission_mode"`
	Command        string                `json:"command"`
	CommandPath    string                `json:"command_path,omitempty"`
	Args           []string              `json:"args,omitempty"`
	ProcessDir     string                `json:"process_dir,omitempty"`
	EnvKeys        []string              `json:"env_keys,omitempty"`
}

type ProcessSnapshot struct {
	PIDFile string `json:"pid_file,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Running bool   `json:"running"`
	Message string `json:"message,omitempty"`
}

type LogSnippet struct {
	Path  string   `json:"path"`
	Lines []string `json:"lines"`
}

func NewReport(now time.Time) Report {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Report{GeneratedAt: now.UTC(), Severity: SeverityPass}
}

func (r *Report) AddCheck(check Check) {
	if check.Severity == "" {
		check.Severity = SeverityPass
	}
	r.Checks = append(r.Checks, check)
	if check.Recommendation != "" && check.Severity != SeverityPass {
		r.Recommendations = append(r.Recommendations, Recommendation{CheckID: check.ID, Message: check.Recommendation})
	}
	r.Severity = AggregateSeverity(r.Checks)
}

func AggregateSeverity(checks []Check) Severity {
	severity := SeverityPass
	for _, check := range checks {
		switch check.Severity {
		case SeverityFail:
			return SeverityFail
		case SeverityWarn:
			severity = SeverityWarn
		}
	}
	return severity
}

func (r *Report) Finalize() {
	r.Severity = AggregateSeverity(r.Checks)
	seen := map[string]bool{}
	var recs []Recommendation
	for _, rec := range r.Recommendations {
		key := rec.CheckID + "\x00" + rec.Message
		if rec.Message == "" || seen[key] {
			continue
		}
		seen[key] = true
		recs = append(recs, rec)
	}
	r.Recommendations = recs
}

func RenderText(w io.Writer, report Report, verbose bool) error {
	report.Finalize()
	fmt.Fprintf(w, "doctor: %s\n", strings.ToUpper(string(report.Severity)))
	if report.AssistantID != "" {
		fmt.Fprintf(w, "assistant: %s", report.AssistantID)
		if report.AssistantName != "" && report.AssistantName != report.AssistantID {
			fmt.Fprintf(w, " (%s)", report.AssistantName)
		}
		fmt.Fprintln(w)
	}
	if report.Status != nil {
		fmt.Fprintf(w, "sessions: active=%d pending_permissions=%d recent_errors=%d\n", report.Status.ActiveSessions, report.Status.PendingPermissions, report.Status.RecentErrorCount)
	}
	fmt.Fprintln(w)
	for _, check := range report.Checks {
		if !verbose && check.Severity == SeverityPass {
			continue
		}
		fmt.Fprintf(w, "[%s] %s: %s\n", strings.ToUpper(string(check.Severity)), check.Title, check.Message)
		if verbose {
			writeDetails(w, check.Details, "  ")
		}
	}
	if len(report.RecentErrors) > 0 {
		fmt.Fprintln(w, "\nrecent errors:")
		for _, event := range report.RecentErrors {
			fmt.Fprintf(w, "- %s %s %s\n", event.At.Format(time.RFC3339), event.Scope, event.Message)
		}
	}
	if verbose && len(report.LogSnippets) > 0 {
		fmt.Fprintln(w, "\nlog snippets:")
		for _, snippet := range report.LogSnippets {
			fmt.Fprintf(w, "%s:\n", snippet.Path)
			for _, line := range snippet.Lines {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}
	if len(report.Recommendations) > 0 {
		fmt.Fprintln(w, "\nnext actions:")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(w, "- %s\n", rec.Message)
		}
	}
	return nil
}

func RenderJSON(w io.Writer, report Report) error {
	report.Finalize()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeDetails(w io.Writer, details map[string]any, prefix string) {
	if len(details) == 0 {
		return
	}
	keys := make([]string, 0, len(details))
	for key := range details {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(w, "%s%s: %v\n", prefix, key, details[key])
	}
}
