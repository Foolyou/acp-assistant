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
	sections := []string{managedInstructionPreamble(provider)}
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
