package harness_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/harness"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestLaunchProfilesGateModesByHarnessProvider(t *testing.T) {
	codex, err := harness.ResolveLaunchProfile(model.ProviderCodex, model.PermissionFullAuto, harness.ProfileOptions{ReasoningEffort: "high", ResponseMode: "concise"})
	if err != nil {
		t.Fatalf("codex full_auto should be supported: %v", err)
	}
	if codex.Key != "full_auto" || codex.Command != "codex-acp" {
		t.Fatalf("unexpected codex profile: %#v", codex)
	}
	for _, disallowed := range []string{"--permission-mode", "--model-reasoning-effort", "--response-mode", "--dangerously-bypass-approvals-and-sandbox"} {
		if slices.Contains(codex.Args, disallowed) {
			t.Fatalf("codex-acp profile should use -c config overrides, found %s in %#v", disallowed, codex.Args)
		}
	}
	if !slices.Contains(codex.Args, `approval_policy="never"`) || !slices.Contains(codex.Args, `sandbox_mode="workspace-write"`) || !slices.Contains(codex.Args, `model_reasoning_effort="high"`) {
		t.Fatalf("unexpected codex full_auto args: %#v", codex.Args)
	}

	yolo, err := harness.ResolveLaunchProfile(model.ProviderCodex, model.PermissionYolo, harness.ProfileOptions{})
	if err != nil {
		t.Fatalf("codex yolo should be supported: %v", err)
	}
	if !slices.Contains(yolo.Args, `approval_policy="never"`) || !slices.Contains(yolo.Args, `sandbox_mode="danger-full-access"`) {
		t.Fatalf("unexpected codex yolo args: %#v", yolo.Args)
	}

	if _, err := harness.ResolveLaunchProfile(model.ProviderClaude, model.PermissionFullAuto, harness.ProfileOptions{}); err == nil {
		t.Fatal("claude full_auto should be rejected")
	}
	claude, err := harness.ResolveLaunchProfile(model.ProviderClaude, model.PermissionYolo, harness.ProfileOptions{})
	if err != nil {
		t.Fatalf("claude yolo should be supported: %v", err)
	}
	if claude.Command != "npx" || claude.Key != "yolo" {
		t.Fatalf("unexpected claude profile: %#v", claude)
	}
}

func TestLaunchProfilesApplyManagedInstructionMechanisms(t *testing.T) {
	codex, err := harness.ResolveLaunchProfile(model.ProviderCodex, model.PermissionManual, harness.ProfileOptions{
		ManagedInstructions: "managed instructions",
	})
	if err != nil {
		t.Fatalf("resolve codex profile: %v", err)
	}
	if len(codex.Env) != 0 {
		t.Fatalf("codex profile should not override CODEX_HOME, got %#v", codex.Env)
	}
	if !slices.Contains(codex.Args, `developer_instructions="managed instructions"`) {
		t.Fatalf("expected codex developer_instructions override, got %#v", codex.Args)
	}

	claude, err := harness.ResolveLaunchProfile(model.ProviderClaude, model.PermissionManual, harness.ProfileOptions{
		ManagedInstructions: "managed instructions",
	})
	if err != nil {
		t.Fatalf("resolve claude profile: %v", err)
	}
	if slices.Contains(claude.Args, "--plugin-dir") {
		t.Fatalf("claude profile should not use generated plugin args, got %#v", claude.Args)
	}
	if claude.ManagedInstructions != "managed instructions" {
		t.Fatalf("expected claude managed instructions on profile, got %#v", claude)
	}
}

func TestClaudeLaunchProfileAppliesReasoningEffort(t *testing.T) {
	claude, err := harness.ResolveLaunchProfile(model.ProviderClaude, model.PermissionManual, harness.ProfileOptions{
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatalf("resolve claude profile: %v", err)
	}
	if claude.EffortLevel != "high" {
		t.Fatalf("expected claude effort level, got %#v", claude)
	}
}

func TestPrepareOverlayGeneratesProviderFiles(t *testing.T) {
	root := t.TempDir()
	assistantHome := filepath.Join(root, "alpha")
	cfg := configspace.ApplyAssistantHome(model.AssistantConfig{
		ID:       "alpha",
		Name:     "Alpha",
		HomePath: assistantHome,
		Harness:  model.HarnessBinding{Provider: model.ProviderCodex},
	})
	if err := configspace.EnsureAssistantSources(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ConfigspacePath, "instructions", "common.md"), []byte("common managed instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ConfigspacePath, "instructions", "codex.md"), []byte("codex managed instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ConfigspacePath, "instructions", "claude.md"), []byte("claude managed instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	overlay, err := harness.PrepareOverlay(cfg, root)
	if err != nil {
		t.Fatalf("prepare codex overlay: %v", err)
	}
	if len(overlay.Env) != 0 {
		t.Fatalf("unexpected codex env: %#v", overlay.Env)
	}
	if overlay.ProcessDir != cfg.WorkspacePath {
		t.Fatalf("unexpected codex process dir: %#v", overlay)
	}
	for _, path := range []string{
		filepath.Join(cfg.WorkspacePath, ".agents", "skills", "acpa-cron", "SKILL.md"),
		filepath.Join(cfg.WorkspacePath, ".agents", "skills", "acpa-cron", ".acpa-managed.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.ConfigspacePath, "harness", "codex-home")); !os.IsNotExist(err) {
		t.Fatalf("codex overlay home should not be generated, err=%v", err)
	}
	if !strings.Contains(overlay.ManagedInstructions, "common managed instructions") || !strings.Contains(overlay.ManagedInstructions, "codex managed instructions") || strings.Contains(overlay.ManagedInstructions, "claude managed instructions") {
		t.Fatalf("unexpected codex managed instructions: %q", overlay.ManagedInstructions)
	}
	if !strings.Contains(overlay.ManagedInstructions, "AGENTS.md") || strings.TrimSpace(overlay.PromptPrefix) != "" {
		t.Fatalf("managed instructions should mention AGENTS.md and prompt prefix should be empty: %#v", overlay)
	}
	gitignore, err := os.ReadFile(filepath.Join(cfg.WorkspacePath, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gitignore), ".agents/skills/acpa-*/") || !strings.Contains(string(gitignore), ".claude/skills/acpa-*/") {
		t.Fatalf("expected managed skill ignore rules, got %q", string(gitignore))
	}

	cfg.Harness.Provider = model.ProviderClaude
	claudeBinDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(claudeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claudeBin := filepath.Join(claudeBinDir, "claude")
	if err := os.WriteFile(claudeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", claudeBinDir)
	overlay, err = harness.PrepareOverlay(cfg, root)
	if err != nil {
		t.Fatalf("prepare claude overlay: %v", err)
	}
	if overlay.ClaudePluginDir != "" {
		t.Fatalf("claude plugin dir should not be generated: %#v", overlay)
	}
	if overlay.ProcessDir != cfg.WorkspacePath {
		t.Fatalf("unexpected claude process dir: %#v", overlay)
	}
	if overlay.Env["CLAUDE_CODE_EXECUTABLE"] != claudeBin {
		t.Fatalf("expected claude executable env, got %#v", overlay.Env)
	}
	for _, path := range []string{
		filepath.Join(cfg.WorkspacePath, ".claude", "skills", "acpa-cron", "SKILL.md"),
		filepath.Join(cfg.WorkspacePath, ".claude", "skills", "acpa-cron", ".acpa-managed.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if !strings.Contains(overlay.ManagedInstructions, "common managed instructions") || !strings.Contains(overlay.ManagedInstructions, "claude managed instructions") || strings.Contains(overlay.ManagedInstructions, "codex managed instructions") {
		t.Fatalf("unexpected claude managed instructions: %q", overlay.ManagedInstructions)
	}
}

func TestPrepareOverlayFailsOnUnownedManagedSkillCollision(t *testing.T) {
	root := t.TempDir()
	cfg := configspace.ApplyAssistantHome(model.AssistantConfig{
		ID:       "alpha",
		Name:     "Alpha",
		HomePath: filepath.Join(root, "alpha"),
		Harness:  model.HarnessBinding{Provider: model.ProviderCodex},
	})
	if err := configspace.EnsureAssistantSources(cfg); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(cfg.WorkspacePath, ".agents", "skills", "acpa-cron")
	if err := os.MkdirAll(collision, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(collision, "SKILL.md"), []byte("user skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := harness.PrepareOverlay(cfg, root); err == nil || !strings.Contains(err.Error(), "unowned") {
		t.Fatalf("expected unowned collision error, got %v", err)
	}
}
