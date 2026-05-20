package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestAssistantCreateInspectAndChannelOnboarding(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("ACPA_HOME", filepath.Join(root, "home"))
	t.Setenv("FEISHU_APP_ID", "app-id")
	t.Setenv("FEISHU_APP_SECRET", "app-secret")

	workspace := filepath.Join(root, "workspace")
	configDir := filepath.Join(root, "config")
	var out bytes.Buffer
	if err := run(ctx, []string{"assistant", "create", "--name", "Demo Assistant", "--workspace", workspace, "--configspace", configDir, "--harness", "codex"}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("assistant create: %v", err)
	}
	if !strings.Contains(out.String(), "created assistant demo-assistant") {
		t.Fatalf("unexpected create output: %s", out.String())
	}

	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		t.Fatalf("load created assistant: %v", err)
	}
	if cfg.ID != "demo-assistant" || cfg.Harness.Provider != model.ProviderCodex {
		t.Fatalf("unexpected assistant config: %#v", cfg)
	}
	if _, err := os.Stat(filepath.Join(workspace, "memory", "identity.md")); err != nil {
		t.Fatalf("memory skeleton missing: %v", err)
	}

	out.Reset()
	if err := run(ctx, []string{"channel", "add", "feishu", "--configspace", configDir, "--id", "feishu-main", "--account-id", "main", "--app-id-env", "FEISHU_APP_ID", "--app-secret-env", "FEISHU_APP_SECRET", "--setup-url", "https://example.com/setup"}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("channel add: %v", err)
	}
	if !strings.Contains(out.String(), "wrote feishu channel feishu-main") || !strings.Contains(out.String(), "https://example.com/setup") {
		t.Fatalf("unexpected channel output: %s", out.String())
	}
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		t.Fatalf("load channels: %v", err)
	}
	if len(channels) != 1 || channels[0].Platform != model.PlatformFeishu || channels[0].Credentials["app_id"].Name != "FEISHU_APP_ID" {
		t.Fatalf("unexpected channel config: %#v", channels)
	}

	out.Reset()
	if err := run(ctx, []string{"assistant", "inspect", "demo-assistant"}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("inspect by registry id: %v", err)
	}
	if !strings.Contains(out.String(), "channels: 1") {
		t.Fatalf("unexpected inspect output: %s", out.String())
	}
}
