package diagnostics

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	harnesspkg "github.com/Foolyou/acp-assistant/internal/harness"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
	"gopkg.in/yaml.v3"
)

type Options struct {
	ConfigspacePath string
	HomePath        string
	Now             time.Time
	LogLines        int
}

func Collect(ctx context.Context, opts Options) Report {
	report := NewReport(opts.Now)
	if opts.LogLines <= 0 {
		opts.LogLines = 20
	}
	configDir := strings.TrimSpace(opts.ConfigspacePath)
	if configDir == "" {
		report.AddCheck(Check{ID: "configspace.resolve", Title: "Configspace resolution", Severity: SeverityFail, Message: "configspace path was not provided", Recommendation: "Pass an assistant id, --root, or --configspace."})
		return report
	}
	configDir = absPath(configDir)
	report.ConfigspacePath = configDir
	report.AddCheck(statPathCheck("configspace.exists", "Configspace", configDir, true))

	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		report.AddCheck(Check{ID: "assistant.config", Title: "Assistant config", Severity: SeverityFail, Message: err.Error(), Details: map[string]any{"path": filepath.Join(configDir, configspace.AssistantFile)}, Recommendation: "Repair assistant.yaml or recreate the assistant configspace."})
		return report
	}
	report.AssistantID = cfg.ID
	report.AssistantName = cfg.Name
	report.WorkspacePath = cfg.WorkspacePath
	report.EventDBPath = cfg.EventDBPath
	report.AddCheck(Check{ID: "assistant.config", Title: "Assistant config", Severity: SeverityPass, Message: "loaded assistant config", Details: map[string]any{"path": filepath.Join(configDir, configspace.AssistantFile)}})
	report.AddCheck(registryCheck(opts.HomePath, cfg))
	report.AddCheck(statPathCheck("workspace.exists", "Workspace", cfg.WorkspacePath, true))
	report.AddCheck(statPathCheck("eventdb.parent", "Event DB directory", filepath.Dir(cfg.EventDBPath), true))
	report.Process = processCheck(configDir, &report)

	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		report.AddCheck(Check{ID: "channels.load", Title: "Channel configs", Severity: SeverityFail, Message: err.Error(), Recommendation: "Fix channel YAML files under the configspace channels directory."})
	} else {
		report.AddCheck(Check{ID: "channels.load", Title: "Channel configs", Severity: SeverityPass, Message: fmt.Sprintf("loaded %d channel config(s)", len(channels)), Details: map[string]any{"count": len(channels)}})
	}

	var db *store.Store
	if cfg.EventDBPath != "" {
		db, err = store.Open(cfg.EventDBPath)
		if err != nil {
			report.AddCheck(Check{ID: "eventdb.open", Title: "Event DB", Severity: SeverityFail, Message: err.Error(), Details: map[string]any{"path": cfg.EventDBPath}, Recommendation: "Check event DB path permissions and disk availability."})
		} else {
			defer db.Close()
			if err := db.Migrate(ctx); err != nil {
				report.AddCheck(Check{ID: "eventdb.migrate", Title: "Event DB schema", Severity: SeverityFail, Message: err.Error(), Details: map[string]any{"path": cfg.EventDBPath}, Recommendation: "Inspect the event DB and migration state."})
			} else {
				report.AddCheck(Check{ID: "eventdb.migrate", Title: "Event DB schema", Severity: SeverityPass, Message: "event DB opened and migrations are current", Details: map[string]any{"path": cfg.EventDBPath}})
				status, err := db.StatusSnapshot(ctx, cfg.ID)
				if err != nil {
					report.AddCheck(Check{ID: "status.snapshot", Title: "Status snapshot", Severity: SeverityWarn, Message: err.Error(), Recommendation: "Inspect event DB contents for incomplete tables or corrupt rows."})
				} else {
					report.Status = &StatusSummary{ActiveSessions: status.ActiveSessions, PendingPermissions: status.PendingPermissions, RecentErrorCount: len(status.RecentErrors)}
					report.RecentErrors = status.RecentErrors
					addStatusChecks(&report, status)
					report.Connectors = connectorSnapshots(channels, status.Connectors)
				}
			}
		}
	}
	if db == nil {
		report.Connectors = connectorSnapshots(channels, nil)
	}
	addConnectorChecks(&report, channels)
	addHarnessChecks(&report, cfg, opts.HomePath)
	report.LogSnippets = collectLogSnippets(configDir, opts.LogLines)
	addLogChecks(&report)
	report.Finalize()
	return report
}

func statPathCheck(id, title, path string, wantDir bool) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{ID: id, Title: title, Severity: SeverityFail, Message: err.Error(), Details: map[string]any{"path": path}, Recommendation: "Create the missing path or update assistant.yaml to point at the correct location."}
	}
	if wantDir && !info.IsDir() {
		return Check{ID: id, Title: title, Severity: SeverityFail, Message: "path exists but is not a directory", Details: map[string]any{"path": path}, Recommendation: "Replace the path with a directory or update the configuration."}
	}
	return Check{ID: id, Title: title, Severity: SeverityPass, Message: "path exists", Details: map[string]any{"path": path}}
}

func registryCheck(home string, cfg model.AssistantConfig) Check {
	path := filepath.Join(home, "assistants.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: "global registry file does not exist", Details: map[string]any{"path": path}, Recommendation: "Run assistant create or register the assistant before resolving by id."}
	}
	if err != nil {
		return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: err.Error(), Details: map[string]any{"path": path}, Recommendation: "Check ACPA_HOME and registry file permissions."}
	}
	var reg struct {
		Assistants []struct {
			ID              string `yaml:"id"`
			ConfigspacePath string `yaml:"configspace_path"`
		} `yaml:"assistants"`
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: err.Error(), Details: map[string]any{"path": path}, Recommendation: "Repair the global assistant registry YAML."}
	}
	for _, entry := range reg.Assistants {
		if entry.ID == cfg.ID {
			details := map[string]any{"path": path, "registered_configspace": entry.ConfigspacePath}
			if entry.ConfigspacePath != "" && absPath(entry.ConfigspacePath) != absPath(cfg.ConfigspacePath) {
				return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: "assistant registry points to a different configspace", Details: details, Recommendation: "Update the registry entry or use --configspace explicitly."}
			}
			return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityPass, Message: "assistant is present in global registry", Details: details}
		}
	}
	if len(reg.Assistants) == 0 {
		return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: "assistant id was not found in global registry", Details: map[string]any{"path": path, "assistant_id": cfg.ID}, Recommendation: "Use --configspace explicitly or recreate/register the assistant entry."}
	}
	return Check{ID: "registry.entry", Title: "Assistant registry", Severity: SeverityWarn, Message: "assistant id was not found in global registry", Details: map[string]any{"path": path, "assistant_id": cfg.ID}, Recommendation: "Use --configspace explicitly or recreate/register the assistant entry."}
}

func processCheck(configDir string, report *Report) *ProcessSnapshot {
	pidFile := filepath.Join(configDir, "assistant.pid")
	data, err := os.ReadFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		snapshot := &ProcessSnapshot{PIDFile: pidFile, Running: false, Message: "no pid file"}
		report.AddCheck(Check{ID: "process.pid", Title: "Assistant process", Severity: SeverityWarn, Message: "no assistant.pid file found", Details: map[string]any{"pid_file": pidFile}, Recommendation: "Start the assistant if it should be running."})
		return snapshot
	}
	if err != nil {
		report.AddCheck(Check{ID: "process.pid", Title: "Assistant process", Severity: SeverityWarn, Message: err.Error(), Details: map[string]any{"pid_file": pidFile}, Recommendation: "Check pid file permissions."})
		return &ProcessSnapshot{PIDFile: pidFile, Message: err.Error()}
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		report.AddCheck(Check{ID: "process.pid", Title: "Assistant process", Severity: SeverityFail, Message: "pid file does not contain a valid pid", Details: map[string]any{"pid_file": pidFile}, Recommendation: "Remove the stale pid file and restart the assistant."})
		return &ProcessSnapshot{PIDFile: pidFile, Message: "invalid pid"}
	}
	running := syscall.Kill(pid, 0) == nil
	severity := SeverityPass
	message := "process is running"
	recommendation := ""
	if !running {
		severity = SeverityFail
		message = "pid file exists but process is not running"
		recommendation = "Remove the stale pid file and restart the assistant."
	}
	report.AddCheck(Check{ID: "process.pid", Title: "Assistant process", Severity: severity, Message: message, Details: map[string]any{"pid_file": pidFile, "pid": pid}, Recommendation: recommendation})
	return &ProcessSnapshot{PIDFile: pidFile, PID: pid, Running: running, Message: message}
}

func addStatusChecks(report *Report, status model.StatusSnapshot) {
	if len(status.RecentErrors) == 0 {
		report.AddCheck(Check{ID: "recent.errors", Title: "Recent errors", Severity: SeverityPass, Message: "no recent error events"})
		return
	}
	report.AddCheck(Check{ID: "recent.errors", Title: "Recent errors", Severity: SeverityWarn, Message: fmt.Sprintf("%d recent error event(s)", len(status.RecentErrors)), Recommendation: "Review recent errors in doctor output or acpa logs."})
}

func connectorSnapshots(channels []model.ChannelConfig, statuses []model.ConnectorStatus) []ConnectorSnapshot {
	statusByKey := map[string]model.ConnectorStatus{}
	for _, status := range statuses {
		statusByKey[string(status.Platform)+"/"+status.AccountID] = status
	}
	out := make([]ConnectorSnapshot, 0, len(channels))
	for _, channel := range channels {
		status := statusByKey[string(channel.Platform)+"/"+channel.AccountID]
		keys := make([]string, 0, len(channel.Credentials))
		for key := range channel.Credentials {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out = append(out, ConnectorSnapshot{ID: channel.ID, Platform: channel.Platform, AccountID: channel.AccountID, Enabled: channel.Enabled, State: status.State, Message: status.Message, LastError: status.LastError, UpdatedAt: status.UpdatedAt, Credentials: keys, Options: channel.Options})
	}
	return out
}

func addConnectorChecks(report *Report, channels []model.ChannelConfig) {
	if len(channels) == 0 {
		report.AddCheck(Check{ID: "connectors.config", Title: "Connector configuration", Severity: SeverityWarn, Message: "no channels are configured", Recommendation: "Add a channel if this assistant should receive IM messages."})
		return
	}
	for _, channel := range channels {
		id := "connector." + channel.ID
		details := map[string]any{"platform": channel.Platform, "account_id": channel.AccountID, "enabled": channel.Enabled}
		if !channel.Enabled {
			report.AddCheck(Check{ID: id, Title: "Connector " + channel.ID, Severity: SeverityWarn, Message: "channel is disabled", Details: details, Recommendation: "Enable the channel when it should receive messages."})
			continue
		}
		if _, err := configspace.ResolveSecrets(channel.Credentials); err != nil {
			report.AddCheck(Check{ID: id, Title: "Connector " + channel.ID, Severity: SeverityFail, Message: err.Error(), Details: details, Recommendation: "Fix missing connector credentials or secret references."})
			continue
		}
		snapshot := findConnector(*report, channel)
		if snapshot.State == model.ConnectorStateFailed {
			report.AddCheck(Check{ID: id, Title: "Connector " + channel.ID, Severity: SeverityFail, Message: snapshot.LastError, Details: details, Recommendation: "Restart the assistant after fixing connector configuration or service credentials."})
			continue
		}
		if snapshot.State == "" {
			report.AddCheck(Check{ID: id, Title: "Connector " + channel.ID, Severity: SeverityWarn, Message: "no runtime connector status has been recorded", Details: details, Recommendation: "Start the assistant and check connector startup logs."})
			continue
		}
		report.AddCheck(Check{ID: id, Title: "Connector " + channel.ID, Severity: SeverityPass, Message: "connector config and last status are available", Details: details})
	}
}

func findConnector(report Report, channel model.ChannelConfig) ConnectorSnapshot {
	for _, snapshot := range report.Connectors {
		if snapshot.Platform == channel.Platform && snapshot.AccountID == channel.AccountID {
			return snapshot
		}
	}
	return ConnectorSnapshot{}
}

func addHarnessChecks(report *Report, cfg model.AssistantConfig, home string) {
	options := harnessOptions(cfg, home)
	profile, err := harnesspkg.ResolveLaunchProfile(cfg.Harness.Provider, model.PermissionManual, options)
	if err != nil {
		report.AddCheck(Check{ID: "harness.profile", Title: "Harness launch profile", Severity: SeverityFail, Message: err.Error(), Recommendation: "Fix harness provider, command, or permission mode configuration."})
		return
	}
	envKeys := make([]string, 0, len(profile.Env))
	for key := range profile.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	report.Harness = &HarnessSnapshot{Provider: profile.Provider, PermissionMode: profile.PermissionMode, Command: profile.Command, Args: profile.Args, ProcessDir: profile.ProcessDir, EnvKeys: envKeys}
	report.AddCheck(Check{ID: "harness.profile", Title: "Harness launch profile", Severity: SeverityPass, Message: "manual launch profile resolved", Details: map[string]any{"process_dir": profile.ProcessDir, "env_keys": envKeys}})
	for _, path := range []string{
		filepath.Join(home, "global", configspace.InstructionsFile),
		filepath.Join(cfg.ConfigspacePath, configspace.InstructionsFile),
	} {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			report.AddCheck(Check{ID: "harness.instructions", Title: "Harness instructions", Severity: SeverityWarn, Message: "instruction source is missing", Details: map[string]any{"path": path}, Recommendation: "Run assistant create or ensure assistant instruction sources exist."})
		} else if err != nil {
			report.AddCheck(Check{ID: "harness.instructions", Title: "Harness instructions", Severity: SeverityWarn, Message: err.Error(), Details: map[string]any{"path": path}, Recommendation: "Check instruction source permissions."})
		}
	}
	commandPath, err := exec.LookPath(profile.Command)
	if err != nil {
		report.AddCheck(Check{ID: "harness.command", Title: "Harness command", Severity: SeverityFail, Message: err.Error(), Details: map[string]any{"command": profile.Command}, Recommendation: "Install the harness command or update assistant.yaml harness.command."})
		return
	}
	report.Harness.CommandPath = commandPath
	report.AddCheck(Check{ID: "harness.command", Title: "Harness command", Severity: SeverityPass, Message: "command found", Details: map[string]any{"command": profile.Command, "path": commandPath}})
	if profile.ProcessDir != "" {
		report.AddCheck(cwdCheck(profile.ProcessDir))
	}
}

func cwdCheck(path string) Check {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{ID: "harness.cwd", Title: "Harness cwd", Severity: SeverityWarn, Message: "runtime cwd has not been prepared yet", Details: map[string]any{"path": path}, Recommendation: "Start the assistant to prepare harness runtime directories."}
	}
	if err != nil {
		return Check{ID: "harness.cwd", Title: "Harness cwd", Severity: SeverityWarn, Message: err.Error(), Details: map[string]any{"path": path}, Recommendation: "Check harness runtime directory permissions."}
	}
	if !info.IsDir() {
		return Check{ID: "harness.cwd", Title: "Harness cwd", Severity: SeverityFail, Message: "path exists but is not a directory", Details: map[string]any{"path": path}, Recommendation: "Replace the path with a directory or update ACPA_HOME."}
	}
	return Check{ID: "harness.cwd", Title: "Harness cwd", Severity: SeverityPass, Message: "path exists", Details: map[string]any{"path": path}}
}

func harnessOptions(cfg model.AssistantConfig, home string) harnesspkg.ProfileOptions {
	options := harnesspkg.ProfileOptions{Command: cfg.Harness.Command, Args: cfg.Harness.Args}
	processDir := filepath.Join(home, "runtime-cwd", safeName(cfg.ID), safeName(string(cfg.Harness.Provider)))
	options.ProcessDir = processDir
	switch cfg.Harness.Provider {
	case model.ProviderCodex:
		options.Env = map[string]string{"CODEX_HOME": filepath.Join(cfg.ConfigspacePath, "harness", "codex-home")}
	case model.ProviderClaude:
		options.ClaudePluginDir = filepath.Join(cfg.ConfigspacePath, "harness", "claude-plugin")
		if claudePath, err := exec.LookPath("claude"); err == nil && strings.TrimSpace(claudePath) != "" {
			options.Env = map[string]string{"CLAUDE_CODE_EXECUTABLE": claudePath}
		}
	}
	return options
}

func collectLogSnippets(configDir string, limit int) []LogSnippet {
	var out []LogSnippet
	for _, name := range []string{"acpa.err.log", "acpa.out.log"} {
		path := filepath.Join(configDir, name)
		lines, err := tailLines(path, limit)
		if err == nil && len(lines) > 0 {
			out = append(out, LogSnippet{Path: path, Lines: lines})
		}
	}
	return out
}

func addLogChecks(report *Report) {
	if len(report.LogSnippets) == 0 {
		report.AddCheck(Check{ID: "logs.snippets", Title: "Log files", Severity: SeverityWarn, Message: "no assistant log snippets found", Recommendation: "Use foreground mode or start the assistant to create log files."})
		return
	}
	report.AddCheck(Check{ID: "logs.snippets", Title: "Log files", Severity: SeverityPass, Message: fmt.Sprintf("collected %d log snippet(s)", len(report.LogSnippets))})
}

func tailLines(path string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[len(lines)-limit:]
		}
	}
	return lines, scanner.Err()
}

func absPath(path string) string {
	if path == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func safeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
