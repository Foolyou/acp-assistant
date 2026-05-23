package harness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/model"
)

type Overlay struct {
	Env                 map[string]string
	ClaudePluginDir     string
	PromptPrefix        string
	ProcessDir          string
	ManagedInstructions string
}

func PrepareOverlay(cfg model.AssistantConfig, acpaHome string) (Overlay, error) {
	_ = acpaHome
	cfg = configspace.ApplyAssistantHome(cfg)
	if strings.TrimSpace(cfg.ConfigspacePath) == "" {
		return Overlay{}, fmt.Errorf("configspace path is required")
	}
	if strings.TrimSpace(cfg.WorkspacePath) == "" {
		return Overlay{}, fmt.Errorf("workspace path is required")
	}
	if err := configspace.EnsureAssistantSources(cfg); err != nil {
		return Overlay{}, err
	}
	if err := MaterializeBuiltInSkills(cfg.WorkspacePath, cfg.Harness.Provider); err != nil {
		return Overlay{}, err
	}
	instructions, err := RenderManagedInstructions(cfg.ConfigspacePath, cfg.Harness.Provider)
	if err != nil {
		return Overlay{}, err
	}
	overlay := Overlay{
		ProcessDir:          cfg.WorkspacePath,
		ManagedInstructions: instructions,
	}
	if cfg.Harness.Provider == model.ProviderClaude {
		overlay.Env = claudeOverlayEnv()
	}
	return overlay, nil
}

func RenderManagedInstructions(configspacePath string, provider model.HarnessProvider) (string, error) {
	providerFile, err := providerInstructionsFile(provider)
	if err != nil {
		return "", err
	}
	sections := []string{managedInstructionPreamble(provider), hostCronProtocolInstructions()}
	for _, path := range []string{
		filepath.Join(configspacePath, configspace.InstructionsDir, configspace.CommonInstructionsFile),
		filepath.Join(configspacePath, configspace.InstructionsDir, providerFile),
	} {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if text := strings.TrimSpace(string(data)); text != "" {
			sections = append(sections, text)
		}
	}
	if len(sections) == 0 {
		return "", nil
	}
	return strings.Join(sections, "\n\n"), nil
}

func managedInstructionPreamble(provider model.HarnessProvider) string {
	text := "Shared project guidance belongs in the workspace `AGENTS.md` file. When durable shared guidance should be added or changed, update `AGENTS.md`."
	if provider == model.ProviderClaude {
		text += " Do not write shared project guidance directly into `CLAUDE.md`; it is only a bridge to `AGENTS.md`."
	}
	return text
}

func hostCronProtocolInstructions() string {
	return strings.Join([]string{
		"ACPA host cron protocol:",
		"",
		"When the user asks to create a reminder, schedule one-time work, schedule recurring work, remove scheduled work, or list scheduled jobs, use the host cron protocol instead of merely saying it is done.",
		"",
		"Return exactly one fenced JSON block using ```cron and no user-facing prose. ACPA will execute the block, then ACPA will send the confirmation or error.",
		"",
		"Strict JSON rules:",
		"- Output valid JSON only inside the ```cron fence.",
		"- Use double quotes for all JSON strings.",
		"- Do not include comments, markdown outside the fence, trailing commas, null fields, or extra fields.",
		"- Top-level allowed fields are only: action, id, job, patch.",
		"- job allowed fields are only: name, schedule, sessionTarget, payload, delivery.",
		"- schedule allowed fields are only: kind, at, everyMs, expr, tz.",
		"- payload allowed fields are only: kind, message.",
		"- delivery allowed fields are only: mode, target.",
		"- patch allowed fields are only: enabled, name.",
		"- Never output legacy or invented fields such as recurring, schedule_type, schedule_expr, message at the top level, job_id, create, delete, timezone, prompt, or targetSession.",
		"",
		"Create one-time reminder or one-time scheduled work with schedule.kind at:",
		"```cron",
		`{"action":"add","job":{"name":"short name","schedule":{"kind":"at","at":"2099-01-02T15:04:05+08:00"},"sessionTarget":"isolated","payload":{"kind":"agentTurn","message":"self-contained reminder or task prompt"},"delivery":{"mode":"announce","target":"origin"}}}`,
		"```",
		"",
		"Create a fixed-interval recurring job with schedule.kind every and everyMs as an integer number of milliseconds:",
		"```cron",
		`{"action":"add","job":{"name":"hourly status check","schedule":{"kind":"every","everyMs":3600000},"sessionTarget":"isolated","payload":{"kind":"agentTurn","message":"check current progress and summarize blockers"},"delivery":{"mode":"announce","target":"origin"}}}`,
		"```",
		"",
		"Create a calendar recurring job with schedule.kind cron, expr as five cron fields, and optional tz:",
		"```cron",
		`{"action":"add","job":{"name":"weekday morning summary","schedule":{"kind":"cron","expr":"0 9 * * 1-5","tz":"Asia/Shanghai"},"sessionTarget":"isolated","payload":{"kind":"agentTurn","message":"summarize current work progress"},"delivery":{"mode":"announce","target":"origin"}}}`,
		"```",
		"",
		"Choose job.name as a concise stable title for the scheduled work. ACPA shows this title immediately when the cron runs, before the model completes the scheduled prompt.",
		"For relative user requests like \"in two minutes\", compute an RFC3339 timestamp with an explicit offset and use schedule.kind at. Do not use recurring for one-time reminders.",
		"",
		"Rename only when the user explicitly asks to rename or the scheduled work meaning changes:",
		"```cron",
		`{"action":"update","id":"cron_xxx","patch":{"name":"new short title"}}`,
		"```",
		"",
		"List:",
		"```cron",
		`{"action":"list"}`,
		"```",
		"",
		"Remove:",
		"```cron",
		`{"action":"remove","id":"cron_xxx"}`,
		"```",
		"",
		"Use schedule.kind at with RFC3339 times, every with everyMs, or cron with expr and optional tz. Use payload.kind agentTurn with a self-contained message. Use sessionTarget isolated unless the user explicitly asks scheduled work to continue the main conversation. Use delivery mode announce with target origin by default, or none when the user asks for no delivery.",
	}, "\n")
}

func ManagedInstructionPaths(configspacePath string, provider model.HarnessProvider) ([]string, error) {
	providerFile, err := providerInstructionsFile(provider)
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(configspacePath, configspace.InstructionsDir, configspace.CommonInstructionsFile),
		filepath.Join(configspacePath, configspace.InstructionsDir, providerFile),
	}, nil
}

func providerInstructionsFile(provider model.HarnessProvider) (string, error) {
	switch provider {
	case model.ProviderCodex:
		return configspace.CodexInstructionsFile, nil
	case model.ProviderClaude:
		return configspace.ClaudeInstructionsFile, nil
	default:
		return "", fmt.Errorf("unsupported harness provider %q", provider)
	}
}

func claudeOverlayEnv() map[string]string {
	path, err := exec.LookPath("claude")
	if err != nil || strings.TrimSpace(path) == "" {
		return nil
	}
	return map[string]string{"CLAUDE_CODE_EXECUTABLE": path}
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
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "assistant"
	}
	return name
}
