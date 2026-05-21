package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestServerStatusWritesMetadataAndServesConsole(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewServer(ServerOptions{Home: home, Executable: os.Args[0], Bind: "127.0.0.1:0"})
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx) }()

	client := waitForClient(t, home)
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Reachable || status.Endpoint == "" || status.PID == 0 {
		t.Fatalf("unexpected status: %#v", status)
	}
	res, err := http.Get(status.Endpoint + "/")
	if err != nil {
		t.Fatalf("console get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("console status: %s", res.Status)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Start QR Setup") || !strings.Contains(string(body), "Run Doctor") {
		t.Fatalf("console should expose Feishu QR setup flow")
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("server shutdown: %v", err)
	}
}

func TestServerSupportsForwardedPrefixForConsoleAndAPI(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewServer(ServerOptions{Home: home, Executable: os.Args[0], Bind: "127.0.0.1:0"})
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx) }()

	client := waitForClient(t, home)
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, status.Endpoint+"/acp-assistant/api/status", nil)
	req.Header.Set("X-Forwarded-Prefix", "/acp-assistant")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("prefixed status get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("prefixed status code: %s", res.Status)
	}
	var prefixed Status
	if err := json.NewDecoder(res.Body).Decode(&prefixed); err != nil {
		t.Fatalf("decode prefixed status: %v", err)
	}
	if !prefixed.Reachable || prefixed.Endpoint == "" {
		t.Fatalf("unexpected prefixed status: %#v", prefixed)
	}

	consoleReq, _ := http.NewRequest(http.MethodGet, status.Endpoint+"/acp-assistant/", nil)
	consoleReq.Header.Set("X-Forwarded-Prefix", "/acp-assistant")
	consoleRes, err := http.DefaultClient.Do(consoleReq)
	if err != nil {
		t.Fatalf("prefixed console get: %v", err)
	}
	defer consoleRes.Body.Close()
	if consoleRes.StatusCode != http.StatusOK {
		t.Fatalf("prefixed console status: %s", consoleRes.Status)
	}
	body, _ := io.ReadAll(consoleRes.Body)
	html := string(body)
	if !strings.Contains(html, "api/") || !strings.Contains(html, "window.location.href") {
		t.Fatalf("console should derive API base from current URL")
	}
	if strings.Contains(html, "fetch(\"/api/") || strings.Contains(html, "fetch('/api/") {
		t.Fatalf("console should not use root-relative API paths")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("server shutdown: %v", err)
	}
}

func TestClientEnsureRunningUsesLazyStartup(t *testing.T) {
	home := t.TempDir()
	started := false
	client := Client{
		Home: home,
		StartDaemon: func(ctx context.Context, bind string) error {
			started = true
			return SaveMetadata(home, Metadata{PID: 123, Endpoint: "http://127.0.0.1:1", Started: time.Now()})
		},
		HTTPClient: failingRoundTripperClient(),
	}
	_, err := client.EnsureRunning(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "did not become ready") {
		t.Fatalf("expected readiness failure after lazy start, got %v", err)
	}
	if !started {
		t.Fatalf("lazy startup hook was not called")
	}
}

func TestSupervisorAutostartAndLifecycleState(t *testing.T) {
	if os.Getenv("ACPA_DAEMON_TEST_WORKER") == "1" {
		select {}
	}
	home := t.TempDir()
	configDir := filepath.Join(home, "assistants", "alpha", "config")
	cfg := model.AssistantConfig{
		ID:              "alpha",
		Name:            "Alpha",
		WorkspacePath:   filepath.Join(home, "assistants", "alpha", "workspace"),
		ConfigspacePath: configDir,
		Harness:         model.HarnessBinding{Provider: model.ProviderCodex, Command: "codex"},
		Memory:          model.DefaultMemoryConfig(),
		EventDBPath:     filepath.Join(configDir, configspace.EventsDBFile),
		Autostart:       true,
	}
	if err := configspace.Initialize(context.Background(), cfg); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := RegisterAssistant(home, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}
	supervisor := NewSupervisor(home, os.Args[0])
	t.Setenv("ACPA_DAEMON_TEST_WORKER", "1")
	state, err := supervisor.Start(context.Background(), configDir)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !state.Running || state.PID == 0 {
		t.Fatalf("unexpected start state: %#v", state)
	}
	if state.ChannelCount != 0 {
		t.Fatalf("new assistant should start with no channels: %#v", state)
	}
	state, err = supervisor.SetAutostart(configDir, false)
	if err != nil {
		t.Fatalf("set autostart: %v", err)
	}
	if state.Autostart {
		t.Fatalf("autostart should be false: %#v", state)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	state, err = supervisor.Stop(stopCtx, configDir)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if state.Running {
		t.Fatalf("expected stopped state: %#v", state)
	}
}

func TestValidateBindRequiresInsecureConfirmation(t *testing.T) {
	if err := ValidateBind(DefaultBindAddress, false, nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("loopback bind should be accepted: %v", err)
	}
	if err := ValidateBind("0.0.0.0:43791", false, nil, &bytes.Buffer{}); err == nil {
		t.Fatalf("non-loopback bind without --insecure should fail")
	}
	if err := ValidateBind("0.0.0.0:43791", true, strings.NewReader("no\n"), &bytes.Buffer{}); err == nil {
		t.Fatalf("non-loopback bind without exact confirmation should fail")
	}
	if err := ValidateBind("0.0.0.0:43791", true, strings.NewReader("0.0.0.0:43791\n"), &bytes.Buffer{}); err != nil {
		t.Fatalf("confirmed insecure bind should pass: %v", err)
	}
}

func TestCreateAssistantAndManualFeishuSetupPersistConfig(t *testing.T) {
	home := t.TempDir()
	server := NewServer(ServerOptions{Home: home, Executable: os.Args[0], Bind: "127.0.0.1:0"})
	req := CreateAssistantRequest{Name: "Web Alpha", Harness: model.ProviderCodex}
	cfg, err := server.createAssistant(context.Background(), req)
	if err != nil {
		t.Fatalf("create assistant: %v", err)
	}
	if !cfg.Autostart {
		t.Fatalf("new assistant should default autostart=true")
	}
	body, _ := json.Marshal(FeishuManualRequest{
		AssistantID: "web-alpha",
		ChannelID:   "feishu-main",
		AppID:       "cli_test",
		AppSecret:   "secret_test",
	})
	httpReq, _ := http.NewRequest(http.MethodPost, "/api/setup/feishu/manual", bytes.NewReader(body))
	rec := &responseRecorder{header: http.Header{}}
	server.handleFeishuManual(rec, httpReq)
	if rec.status >= 400 {
		t.Fatalf("manual setup failed: status=%d body=%s", rec.status, rec.body.String())
	}
	channels, err := configspace.LoadChannels(cfg.ConfigspacePath)
	if err != nil {
		t.Fatalf("load channels: %v", err)
	}
	if len(channels) != 1 || channels[0].Credentials["app_id"].Type != model.SecretFile {
		t.Fatalf("unexpected channel config: %#v", channels)
	}
	state, err := server.supervisor.Status(context.Background(), cfg.ConfigspacePath)
	if err != nil {
		t.Fatalf("assistant status: %v", err)
	}
	if state.ChannelCount != 1 {
		t.Fatalf("expected one configured channel, got %#v", state)
	}
}

func TestAssistantDoctorEndpointReturnsDiagnosticReport(t *testing.T) {
	home := t.TempDir()
	server := NewServer(ServerOptions{Home: home, Executable: os.Args[0], Bind: "127.0.0.1:0"})
	cfg, err := server.createAssistant(context.Background(), CreateAssistantRequest{Name: "Doctor Alpha", Harness: model.ProviderCodex})
	if err != nil {
		t.Fatalf("create assistant: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/api/assistants/doctor-alpha/doctor", nil)
	rec := &responseRecorder{header: http.Header{}}
	server.handleAssistantAction(rec, req)
	if rec.status >= 400 {
		t.Fatalf("doctor endpoint failed: status=%d body=%s", rec.status, rec.body.String())
	}
	var report struct {
		AssistantID string `json:"assistant_id"`
		Severity    string `json:"severity"`
		Checks      []any  `json:"checks"`
	}
	if err := json.NewDecoder(&rec.body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.AssistantID != cfg.ID || report.Severity == "" || len(report.Checks) == 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func waitForClient(t *testing.T, home string) Client {
	t.Helper()
	client := Client{Home: home}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Status(context.Background()); err == nil {
			return client
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("daemon did not become ready")
	return client
}

func failingRoundTripperClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Millisecond}
}

type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *responseRecorder) Header() http.Header { return r.header }

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(statusCode int) { r.status = statusCode }
