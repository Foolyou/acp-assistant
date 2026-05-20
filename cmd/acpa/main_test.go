package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestFeishuChannelAddCanUseQRRegistrationWithoutManualWebsocketURL(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("ACPA_HOME", filepath.Join(root, "home"))

	configDir := filepath.Join(root, "config")
	if err := run(ctx, []string{"assistant", "create", "--name", "Live", "--workspace", filepath.Join(root, "workspace"), "--configspace", configDir, "--harness", "codex"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("assistant create: %v", err)
	}

	server := newFakeFeishuRegistrationServer(t)
	defer server.Close()

	var out bytes.Buffer
	if err := run(ctx, []string{
		"channel", "add", "feishu",
		"--configspace", configDir,
		"--id", "feishu-main",
		"--registration-base-url", server.URL,
		"--open-base-url", server.URL,
		"--onboarding-timeout", "5",
	}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatalf("channel add feishu qr: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Scan the QR code") || !strings.Contains(out.String(), "ABCD-EFGH") {
		t.Fatalf("expected QR onboarding output, got:\n%s", out.String())
	}
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		t.Fatalf("load channels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected one channel, got %#v", channels)
	}
	channel := channels[0]
	if _, ok := channel.Credentials["websocket_url"]; ok {
		t.Fatalf("websocket_url should not be required in QR onboarding config: %#v", channel.Credentials)
	}
	if channel.Credentials["app_id"].Type != model.SecretFile || channel.Credentials["app_secret"].Type != model.SecretFile {
		t.Fatalf("expected file-backed app credentials: %#v", channel.Credentials)
	}
	if channel.Options["bot_open_id"] != "ou_bot" || channel.Options["bot_name"] != "Live Bot" {
		t.Fatalf("expected bot metadata in channel options: %#v", channel.Options)
	}
	appIDBytes, err := os.ReadFile(channel.Credentials["app_id"].Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(appIDBytes)) != "cli_test" {
		t.Fatalf("unexpected app id secret file: %q", string(appIDBytes))
	}
}

func newFakeFeishuRegistrationServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/app/registration", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch r.Form.Get("action") {
		case "init":
			_ = json.NewEncoder(w).Encode(map[string]any{"supported_auth_methods": []string{"client_secret"}})
		case "begin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "device-1",
				"user_code":                 "ABCD-EFGH",
				"verification_uri_complete": "https://open.feishu.cn/page/cli?user_code=ABCD-EFGH",
				"interval":                  1,
				"expire_in":                 30,
			})
		case "poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id":     "cli_test",
				"client_secret": "secret_test",
				"user_info":     map[string]string{"open_id": "ou_user", "tenant_brand": "feishu"},
			})
		default:
			t.Fatalf("unexpected registration action %q", r.Form.Get("action"))
		}
	})
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token"})
	})
	mux.HandleFunc("/open-apis/bot/v3/info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "bot": map[string]string{"app_name": "Live Bot", "open_id": "ou_bot"}})
	})
	return httptest.NewServer(mux)
}
