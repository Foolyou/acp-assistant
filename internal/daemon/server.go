package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/harness"
	"github.com/Foolyou/acp-assistant/internal/im"
	"github.com/Foolyou/acp-assistant/internal/model"
)

type ServerOptions struct {
	Home       string
	Executable string
	Bind       string
}

type Server struct {
	opts       ServerOptions
	supervisor *Supervisor
	httpServer *http.Server
}

func NewServer(opts ServerOptions) *Server {
	if opts.Bind == "" {
		opts.Bind = DefaultBindAddress
	}
	return &Server{opts: opts, supervisor: NewSupervisor(opts.Home, opts.Executable)}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := ValidateExecutable(s.opts.Executable); err != nil {
		return err
	}
	mux := http.NewServeMux()
	s.routes(mux)
	ln, err := net.Listen("tcp", s.opts.Bind)
	if err != nil {
		return err
	}
	endpoint := "http://" + ln.Addr().String()
	if err := SaveMetadata(s.opts.Home, Metadata{PID: os.Getpid(), Endpoint: endpoint, Started: time.Now().UTC()}); err != nil {
		_ = ln.Close()
		return err
	}
	s.httpServer = &http.Server{Handler: mux}
	go s.supervisor.StartAutostart(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()
	err = s.httpServer.Serve(ln)
	s.supervisor.Shutdown(context.Background())
	_ = RemoveMetadata(s.opts.Home)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleConsole)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/daemon/stop", s.handleDaemonStop)
	mux.HandleFunc("/api/assistants", s.handleAssistants)
	mux.HandleFunc("/api/assistants/", s.handleAssistantAction)
	mux.HandleFunc("/api/setup/feishu/manual", s.handleFeishuManual)
	mux.HandleFunc("/api/setup/feishu/qr/begin", s.handleFeishuQRBegin)
	mux.HandleFunc("/api/setup/feishu/qr/complete", s.handleFeishuQRComplete)
}

func (s *Server) status(ctx context.Context) (Status, error) {
	states, err := s.supervisor.List(ctx)
	if err != nil {
		return Status{}, err
	}
	meta, _ := LoadMetadata(s.opts.Home)
	return Status{
		Reachable:      true,
		Endpoint:       meta.Endpoint,
		PID:            meta.PID,
		AssistantCount: len(states),
		RunningCount:   RunningCount(states),
		Assistants:     states,
		ShutdownPolicy: "daemon stop sends SIGTERM to supervised assistant serve processes",
	}, nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	status, err := s.status(r.Context())
	writeJSON(w, status, err)
}

func (s *Server) handleDaemonStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, map[string]string{"status": "stopping"}, nil)
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = s.httpServer.Shutdown(context.Background())
	}()
}

func (s *Server) handleAssistants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		states, err := s.supervisor.List(r.Context())
		writeJSON(w, states, err)
	case http.MethodPost:
		var req CreateAssistantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cfg, err := s.createAssistant(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, stateFromConfig(cfg), nil)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAssistantAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/assistants/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, fmt.Errorf("assistant action is required"))
		return
	}
	configDir, err := ResolveConfigspace(s.opts.Home, parts[0])
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	action := parts[1]
	switch {
	case r.Method == http.MethodGet && action == "status":
		state, err := s.supervisor.Status(r.Context(), configDir)
		writeJSON(w, state, err)
	case r.Method == http.MethodPost && action == "start":
		state, err := s.supervisor.Start(r.Context(), configDir)
		writeJSON(w, state, err)
	case r.Method == http.MethodPost && action == "stop":
		state, err := s.supervisor.Stop(r.Context(), configDir)
		writeJSON(w, state, err)
	case r.Method == http.MethodPost && action == "restart":
		state, err := s.supervisor.Restart(r.Context(), configDir)
		writeJSON(w, state, err)
	case r.Method == http.MethodPost && action == "autostart":
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		state, err := s.supervisor.SetAutostart(configDir, req.Enabled)
		writeJSON(w, state, err)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) createAssistant(ctx context.Context, req CreateAssistantRequest) (model.AssistantConfig, error) {
	id := slug(req.ID)
	if id == "" {
		id = slug(req.Name)
	}
	if id == "" {
		return model.AssistantConfig{}, fmt.Errorf("assistant id or name is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}
	root := strings.TrimSpace(req.RootPath)
	if root == "" {
		root = filepath.Join(s.opts.Home, "assistants", id)
	}
	workspacePath := req.WorkspacePath
	if workspacePath == "" {
		workspacePath = filepath.Join(root, "workspace")
	}
	configDir := req.ConfigspacePath
	if configDir == "" {
		configDir = filepath.Join(root, "config")
	}
	provider := req.Harness
	if provider == "" {
		provider = model.ProviderCodex
	}
	defaultCommand, defaultArgs, err := harness.DefaultCommand(provider)
	if err != nil {
		return model.AssistantConfig{}, err
	}
	command := req.Command
	if command == "" {
		command = defaultCommand
	}
	args := req.Args
	if len(args) == 0 {
		args = defaultArgs
	}
	autostart := true
	if req.Autostart != nil {
		autostart = *req.Autostart
	}
	cfg := model.AssistantConfig{
		ID:              id,
		Name:            name,
		WorkspacePath:   absPath(workspacePath),
		ConfigspacePath: absPath(configDir),
		Harness:         model.HarnessBinding{Provider: provider, Command: command, Args: args},
		Memory:          req.Memory,
		EventDBPath:     filepath.Join(absPath(configDir), configspace.EventsDBFile),
		Autostart:       autostart,
	}
	if cfg.Memory.Files == nil {
		cfg.Memory = model.DefaultMemoryConfig()
	}
	if err := configspace.InitializeGlobal(s.opts.Home); err != nil {
		return model.AssistantConfig{}, err
	}
	if err := configspace.Initialize(ctx, cfg); err != nil {
		return model.AssistantConfig{}, err
	}
	if err := RegisterAssistant(s.opts.Home, cfg); err != nil {
		return model.AssistantConfig{}, err
	}
	return cfg, nil
}

func (s *Server) handleFeishuManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req FeishuManualRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	configDir, err := setupConfigspace(s.opts.Home, req.AssistantID, req.ConfigspacePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	channelID := defaultString(req.ChannelID, "feishu-"+defaultString(req.AccountID, "main"))
	appIDPath := filepath.Join(configDir, "secrets", channelID+".app_id")
	appSecretPath := filepath.Join(configDir, "secrets", channelID+".app_secret")
	if err := writeSecretFile(appIDPath, req.AppID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := writeSecretFile(appSecretPath, req.AppSecret); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	channel := feishuChannel(configDir, channelID, req.AccountID, req.DisplayName, req.Domain, req.OpenBaseURL, enabled)
	channel.Credentials["app_id"] = model.SecretRef{Type: model.SecretFile, Path: appIDPath}
	channel.Credentials["app_secret"] = model.SecretRef{Type: model.SecretFile, Path: appSecretPath}
	writeJSON(w, channel, configspace.SaveChannel(configDir, channel))
}

func (s *Server) handleFeishuQRBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req FeishuQRBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := im.FeishuRegistrationClient{AccountsBaseURL: req.RegistrationBaseURL, OpenBaseURL: req.OpenBaseURL}
	timeout := req.OnboardingTimeoutSec
	if timeout == 0 {
		timeout = 600
	}
	begin, err := client.Begin(r.Context(), im.FeishuRegistrationOptions{Domain: defaultString(req.Domain, "feishu"), TimeoutSeconds: timeout})
	writeJSON(w, begin, err)
}

func (s *Server) handleFeishuQRComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		FeishuQRBeginRequest
		Begin im.FeishuRegistrationResult `json:"begin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	configDir, err := setupConfigspace(s.opts.Home, req.AssistantID, req.ConfigspacePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := im.FeishuRegistrationClient{AccountsBaseURL: req.RegistrationBaseURL, OpenBaseURL: req.OpenBaseURL}
	result, err := client.Poll(r.Context(), req.Begin)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	channelID := defaultString(req.ChannelID, "feishu-"+defaultString(req.AccountID, "main"))
	appIDPath := filepath.Join(configDir, "secrets", channelID+".app_id")
	appSecretPath := filepath.Join(configDir, "secrets", channelID+".app_secret")
	if err := writeSecretFile(appIDPath, result.AppID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := writeSecretFile(appSecretPath, result.AppSecret); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	channel := feishuChannel(configDir, channelID, req.AccountID, req.DisplayName, result.Domain, req.OpenBaseURL, true)
	channel.Credentials["app_id"] = model.SecretRef{Type: model.SecretFile, Path: appIDPath}
	channel.Credentials["app_secret"] = model.SecretRef{Type: model.SecretFile, Path: appSecretPath}
	channel.Options["owner_open_id"] = result.OpenID
	channel.Options["bot_open_id"] = result.BotOpenID
	channel.Options["bot_name"] = result.BotName
	writeJSON(w, channel, configspace.SaveChannel(configDir, channel))
}

func setupConfigspace(home, assistantID, configDir string) (string, error) {
	if configDir != "" {
		return absPath(configDir), nil
	}
	return ResolveConfigspace(home, assistantID)
}

func feishuChannel(configDir, id, accountID, displayName, domain, openBaseURL string, enabled bool) model.ChannelConfig {
	if accountID == "" {
		accountID = "main"
	}
	if displayName == "" {
		displayName = id
	}
	options := map[string]string{"connection_mode": "websocket", "domain": defaultString(domain, "feishu")}
	if strings.TrimSpace(openBaseURL) != "" {
		options["open_base_url"] = strings.TrimRight(openBaseURL, "/")
	}
	return model.ChannelConfig{
		ID:               id,
		Platform:         model.PlatformFeishu,
		AccountID:        accountID,
		DisplayName:      displayName,
		Enabled:          enabled,
		Credentials:      map[string]model.SecretRef{},
		Options:          options,
		RejectNonPrivate: true,
		TextChunkLimit:   3500,
		TokenCachePath:   filepath.Join(configDir, "secrets", id+".token"),
	}
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(consoleHTML))
}

func writeSecretFile(path, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("secret value is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o600)
}

func writeJSON(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
