package harness_test

import (
	"slices"
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
