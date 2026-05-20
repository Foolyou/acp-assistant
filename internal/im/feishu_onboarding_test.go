package im_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/im"
)

func TestFeishuRegistrationFlowCreatesCredentialsAndProbesBot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/app/registration", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch r.Form.Get("action") {
		case "init":
			_ = json.NewEncoder(w).Encode(map[string]any{"supported_auth_methods": []string{"client_secret"}})
		case "begin":
			if r.Form.Get("archetype") != "PersonalAgent" || r.Form.Get("auth_method") != "client_secret" {
				t.Fatalf("unexpected begin form: %s", r.Form.Encode())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "device-1",
				"user_code":                 "ABCD-EFGH",
				"verification_uri_complete": "https://open.feishu.cn/page/cli?user_code=ABCD-EFGH",
				"interval":                  1,
				"expire_in":                 30,
			})
		case "poll":
			if r.Form.Get("device_code") != "device-1" {
				t.Fatalf("unexpected poll form: %s", r.Form.Encode())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id":     "cli_test",
				"client_secret": "secret_test",
				"user_info":     map[string]string{"open_id": "ou_user", "tenant_brand": "feishu"},
			})
		default:
			t.Fatalf("unexpected action %q", r.Form.Get("action"))
		}
	})
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token"})
	})
	mux.HandleFunc("/open-apis/bot/v3/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tenant-token" {
			t.Fatalf("missing tenant token: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "bot": map[string]string{"app_name": "Live Bot", "open_id": "ou_bot"}})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := im.FeishuRegistrationClient{
		AccountsBaseURL: server.URL,
		OpenBaseURL:     server.URL,
		HTTPClient:      server.Client(),
	}
	result, err := client.Register(context.Background(), im.FeishuRegistrationOptions{Domain: "feishu", TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.AppID != "cli_test" || result.AppSecret != "secret_test" || result.OpenID != "ou_user" {
		t.Fatalf("unexpected credentials: %#v", result)
	}
	if result.BotName != "Live Bot" || result.BotOpenID != "ou_bot" {
		t.Fatalf("unexpected bot probe: %#v", result)
	}
	if result.QRURL == "" || result.UserCode != "ABCD-EFGH" {
		t.Fatalf("missing QR registration metadata: %#v", result)
	}
}
