package configspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestInitializeCreatesConfigspaceAndWorkspaceWithoutOverwritingMemory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	configDir := filepath.Join(root, "config")

	existingMemory := filepath.Join(workspace, "memory", "identity.md")
	if err := os.MkdirAll(filepath.Dir(existingMemory), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingMemory, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := model.AssistantConfig{
		ID:              "alpha",
		Name:            "Alpha",
		WorkspacePath:   workspace,
		ConfigspacePath: configDir,
		Harness: model.HarnessBinding{
			Provider: model.ProviderCodex,
			Command:  "codex-acp",
		},
		Memory: model.DefaultMemoryConfig(),
	}
	if err := configspace.Initialize(ctx, cfg); err != nil {
		t.Fatalf("initialize configspace: %v", err)
	}

	for _, path := range []string{
		filepath.Join(configDir, "assistant.yaml"),
		filepath.Join(configDir, "policies.yaml"),
		filepath.Join(configDir, "events.db"),
		filepath.Join(configDir, "channels"),
		filepath.Join(configDir, "secrets"),
		filepath.Join(workspace, "memory", "preferences.md"),
		filepath.Join(workspace, "artifacts"),
		filepath.Join(workspace, "inbox"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	content, err := os.ReadFile(existingMemory)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep me\n" {
		t.Fatalf("existing memory was overwritten: %q", string(content))
	}

	loaded, err := configspace.LoadAssistant(configDir)
	if err != nil {
		t.Fatalf("load assistant: %v", err)
	}
	if loaded.ID != "alpha" || loaded.WorkspacePath != workspace || loaded.Harness.Provider != model.ProviderCodex {
		t.Fatalf("loaded assistant mismatch: %#v", loaded)
	}
}

func TestChannelConfigRoundTripAndSecretResolution(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	cfg := model.AssistantConfig{
		ID:              "alpha",
		Name:            "Alpha",
		WorkspacePath:   filepath.Join(root, "workspace"),
		ConfigspacePath: configDir,
		Harness:         model.HarnessBinding{Provider: model.ProviderCodex, Command: "codex-acp"},
		Memory:          model.DefaultMemoryConfig(),
	}
	if err := configspace.Initialize(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	secretFile := filepath.Join(configDir, "secrets", "app_secret")
	if err := os.WriteFile(secretFile, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ACPA_TEST_APP_ID", "env-id")

	channel := model.ChannelConfig{
		ID:        "feishu-main",
		Platform:  model.PlatformFeishu,
		AccountID: "main",
		Enabled:   true,
		Credentials: map[string]model.SecretRef{
			"app_id":     {Type: model.SecretEnv, Name: "ACPA_TEST_APP_ID"},
			"app_secret": {Type: model.SecretFile, Path: secretFile},
		},
	}
	if err := configspace.SaveChannel(configDir, channel); err != nil {
		t.Fatalf("save channel: %v", err)
	}

	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		t.Fatalf("load channels: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != "feishu-main" {
		t.Fatalf("unexpected channels: %#v", channels)
	}

	resolved, err := configspace.ResolveSecrets(channels[0].Credentials)
	if err != nil {
		t.Fatalf("resolve secrets: %v", err)
	}
	if resolved["app_id"] != "env-id" || resolved["app_secret"] != "file-secret" {
		t.Fatalf("unexpected resolved secrets: %#v", resolved)
	}
}
