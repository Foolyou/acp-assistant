package harness

import (
	"fmt"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/model"
)

type ProfileOptions struct {
	ReasoningEffort     string
	ResponseMode        string
	Command             string
	Args                []string
	Env                 map[string]string
	ClaudePluginDir     string
	PromptPrefix        string
	ProcessDir          string
	ManagedInstructions string
}

type LaunchProfile struct {
	Provider            model.HarnessProvider `json:"provider"`
	Key                 string                `json:"key"`
	PermissionMode      model.PermissionMode  `json:"permission_mode"`
	Command             string                `json:"command"`
	Args                []string              `json:"args"`
	Env                 map[string]string     `json:"env,omitempty"`
	PromptPrefix        string                `json:"prompt_prefix,omitempty"`
	ProcessDir          string                `json:"process_dir,omitempty"`
	EffortLevel         string                `json:"effort_level,omitempty"`
	ManagedInstructions string                `json:"managed_instructions,omitempty"`
}

func ResolveLaunchProfile(provider model.HarnessProvider, mode model.PermissionMode, options ProfileOptions) (LaunchProfile, error) {
	if mode == "" {
		mode = model.PermissionManual
	}
	switch provider {
	case model.ProviderCodex:
		return codexProfile(mode, options)
	case model.ProviderClaude:
		return claudeProfile(mode, options)
	default:
		return LaunchProfile{}, fmt.Errorf("unsupported harness provider %q", provider)
	}
}

func SupportsMode(provider model.HarnessProvider, mode model.PermissionMode) bool {
	_, err := ResolveLaunchProfile(provider, mode, ProfileOptions{})
	return err == nil
}

func DefaultCommand(provider model.HarnessProvider) (string, []string, error) {
	switch provider {
	case model.ProviderCodex:
		return "codex-acp", nil, nil
	case model.ProviderClaude:
		return "npx", []string{"--yes", "@agentclientprotocol/claude-agent-acp"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported harness provider %q", provider)
	}
}

func codexProfile(mode model.PermissionMode, options ProfileOptions) (LaunchProfile, error) {
	if mode != model.PermissionManual && mode != model.PermissionFullAuto && mode != model.PermissionYolo {
		return LaunchProfile{}, fmt.Errorf("codex does not support permission mode %q", mode)
	}
	command := options.Command
	if command == "" {
		command = "codex-acp"
	}
	args := append([]string{}, options.Args...)
	switch mode {
	case model.PermissionFullAuto:
		args = append(args, "-c", `approval_policy="never"`, "-c", `sandbox_mode="workspace-write"`)
	case model.PermissionYolo:
		args = append(args, "-c", `approval_policy="never"`, "-c", `sandbox_mode="danger-full-access"`)
	default:
		args = append(args, "-c", `approval_policy="on-request"`, "-c", `sandbox_mode="workspace-write"`)
	}
	if strings.TrimSpace(options.ReasoningEffort) != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", options.ReasoningEffort))
	}
	if strings.TrimSpace(options.ResponseMode) != "" {
		args = append(args, "-c", fmt.Sprintf("response_mode=%q", options.ResponseMode))
	}
	if strings.TrimSpace(options.ManagedInstructions) != "" {
		args = append(args, "-c", fmt.Sprintf("developer_instructions=%q", options.ManagedInstructions))
	}
	return LaunchProfile{Provider: model.ProviderCodex, Key: string(mode), PermissionMode: mode, Command: command, Args: args, Env: cloneEnv(options.Env), PromptPrefix: options.PromptPrefix, ProcessDir: options.ProcessDir}, nil
}

func claudeProfile(mode model.PermissionMode, options ProfileOptions) (LaunchProfile, error) {
	if mode == model.PermissionFullAuto {
		return LaunchProfile{}, fmt.Errorf("claude code does not support full_auto")
	}
	if mode != model.PermissionManual && mode != model.PermissionYolo {
		return LaunchProfile{}, fmt.Errorf("claude code does not support permission mode %q", mode)
	}
	command := options.Command
	if command == "" {
		command = "npx"
	}
	args := append([]string{}, options.Args...)
	if len(args) == 0 {
		args = []string{"--yes", "@agentclientprotocol/claude-agent-acp"}
	}
	if mode == model.PermissionYolo {
		args = append(args, "--dangerously-skip-permissions")
	}
	effort := strings.TrimSpace(options.ReasoningEffort)
	if effort == "" {
		effort = "high"
	}
	return LaunchProfile{Provider: model.ProviderClaude, Key: string(mode), PermissionMode: mode, Command: command, Args: args, Env: cloneEnv(options.Env), PromptPrefix: options.PromptPrefix, ProcessDir: options.ProcessDir, EffortLevel: effort, ManagedInstructions: options.ManagedInstructions}, nil
}

func cloneEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(env))
	for key, value := range env {
		cloned[key] = value
	}
	return cloned
}
