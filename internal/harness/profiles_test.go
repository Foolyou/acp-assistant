package harness_test

import (
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
