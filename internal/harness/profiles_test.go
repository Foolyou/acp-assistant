package harness_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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

func TestLaunchProfilesApplyOverlayEnvAndPluginArgs(t *testing.T) {
	codex, err := harness.ResolveLaunchProfile(model.ProviderCodex, model.PermissionManual, harness.ProfileOptions{
		Env: map[string]string{"CODEX_HOME": "/tmp/acpa-codex-home"},
	})
	if err != nil {
		t.Fatalf("resolve codex profile: %v", err)
	}
	if codex.Env["CODEX_HOME"] != "/tmp/acpa-codex-home" {
		t.Fatalf("expected CODEX_HOME env on codex profile, got %#v", codex.Env)
	}

	claude, err := harness.ResolveLaunchProfile(model.ProviderClaude, model.PermissionManual, harness.ProfileOptions{
		ClaudePluginDir: "/tmp/acpa-claude-plugin",
	})
	if err != nil {
		t.Fatalf("resolve claude profile: %v", err)
	}
	if !slices.Contains(claude.Args, "--plugin-dir") || !slices.Contains(claude.Args, "/tmp/acpa-claude-plugin") {
		t.Fatalf("expected claude plugin dir args, got %#v", claude.Args)
	}
}

func TestPrepareOverlayGeneratesProviderFiles(t *testing.T) {
	root := t.TempDir()
	globalHome := filepath.Join(root, "home")
	configDir := filepath.Join(root, "config")
	nativeCodexHome := filepath.Join(root, "native-codex")
	t.Setenv("CODEX_HOME", nativeCodexHome)
	if err := os.MkdirAll(nativeCodexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nativeCodexHome, "auth.json"), []byte(`{"token":"redacted"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(globalHome, "global", "skills", "global-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalHome, "global", "instructions.md"), []byte("global instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalHome, "global", "skills", "global-skill", "SKILL.md"), []byte("---\nname: global-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(configDir, "skills", "assistant-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "instructions.md"), []byte("assistant instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "skills", "assistant-skill", "SKILL.md"), []byte("---\nname: assistant-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := model.AssistantConfig{ID: "alpha", Name: "Alpha", ConfigspacePath: configDir, Harness: model.HarnessBinding{Provider: model.ProviderCodex}}
	overlay, err := harness.PrepareOverlay(cfg, globalHome)
	if err != nil {
		t.Fatalf("prepare codex overlay: %v", err)
	}
	codexHome := filepath.Join(configDir, "harness", "codex-home")
	if overlay.Env["CODEX_HOME"] != codexHome {
		t.Fatalf("unexpected codex env: %#v", overlay.Env)
	}
	for _, path := range []string{
		filepath.Join(codexHome, "config.toml"),
		filepath.Join(codexHome, "auth.json"),
		filepath.Join(codexHome, "skills", "acpa-global-global-skill", "SKILL.md"),
		filepath.Join(codexHome, "skills", "acpa-assistant-assistant-skill", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if !strings.Contains(overlay.PromptPrefix, "global instructions") || !strings.Contains(overlay.PromptPrefix, "assistant instructions") {
		t.Fatalf("expected combined instructions in prompt prefix, got %q", overlay.PromptPrefix)
	}
	stateFile := filepath.Join(codexHome, "state_5.sqlite")
	if err := os.WriteFile(stateFile, []byte("keep state"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := harness.PrepareOverlay(cfg, globalHome); err != nil {
		t.Fatalf("prepare codex overlay again: %v", err)
	}
	if content, err := os.ReadFile(stateFile); err != nil || string(content) != "keep state" {
		t.Fatalf("codex overlay generation should preserve existing state, content=%q err=%v", string(content), err)
	}

	cfg.Harness.Provider = model.ProviderClaude
	overlay, err = harness.PrepareOverlay(cfg, globalHome)
	if err != nil {
		t.Fatalf("prepare claude overlay: %v", err)
	}
	claudePlugin := filepath.Join(configDir, "harness", "claude-plugin")
	if overlay.ClaudePluginDir != claudePlugin {
		t.Fatalf("unexpected claude plugin dir: %#v", overlay)
	}
	for _, path := range []string{
		filepath.Join(claudePlugin, ".claude-plugin", "plugin.json"),
		filepath.Join(claudePlugin, "skills", "acpa-global-global-skill", "SKILL.md"),
		filepath.Join(claudePlugin, "skills", "acpa-assistant-assistant-skill", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}
