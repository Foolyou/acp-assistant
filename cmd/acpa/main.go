package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Foolyou/acp-assistant/internal/acp"
	"github.com/Foolyou/acp-assistant/internal/assistant"
	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/daemon"
	"github.com/Foolyou/acp-assistant/internal/diagnostics"
	harnesspkg "github.com/Foolyou/acp-assistant/internal/harness"
	"github.com/Foolyou/acp-assistant/internal/im"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
	"github.com/Foolyou/acp-assistant/internal/workspace"
	qrcode "github.com/skip2/go-qrcode"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "acpa:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	switch args[0] {
	case "assistant":
		return runAssistant(ctx, args[1:], stdin, stdout, stderr)
	case "channel":
		return runChannel(ctx, args[1:], stdin, stdout, stderr)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout)
	case "status":
		return runStatus(ctx, args[1:], stdout)
	case "daemon":
		return runDaemon(ctx, args[1:], stdin, stdout, stderr)
	case "console":
		return runConsole(ctx, args[1:], stdout)
	case "logs":
		return runLogs(ctx, args[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAssistant(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("assistant subcommand is required")
	}
	switch args[0] {
	case "create":
		return assistantCreate(ctx, args[1:], stdout)
	case "list":
		return assistantList(args[1:], stdout)
	case "inspect":
		return assistantInspect(ctx, args[1:], stdout)
	case "start":
		return assistantStart(ctx, args[1:], stdout, stderr)
	case "serve":
		return assistantServe(ctx, args[1:], stdout, stderr)
	case "stop":
		return assistantStop(ctx, args[1:], stdout)
	case "restart":
		return assistantRestart(ctx, args[1:], stdout)
	case "status":
		return assistantLifecycleStatus(ctx, args[1:], stdout)
	case "autostart":
		return assistantAutostart(ctx, args[1:], stdout)
	case "remove":
		return assistantRemove(args[1:], stdout)
	default:
		return fmt.Errorf("unknown assistant subcommand %q", args[0])
	}
}

func runChannel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("channel subcommand is required")
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("channel add requires platform feishu or qqbot")
		}
		return channelAdd(ctx, args[1], args[2:], stdin, stdout)
	case "status":
		return channelStatus(ctx, args[1:], stdout)
	default:
		return fmt.Errorf("unknown channel subcommand %q", args[0])
	}
}

func runDaemon(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("daemon subcommand is required")
	}
	switch args[0] {
	case "start":
		return daemonStart(ctx, args[1:], stdin, stdout, stderr)
	case "run":
		return daemonRun(ctx, args[1:], stdin, stdout)
	case "stop":
		client, err := daemonClient()
		if err != nil {
			return err
		}
		if err := client.StopDaemon(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "stopping daemon; supervised assistant serve processes receive SIGTERM")
		return nil
	case "restart":
		if err := runDaemon(ctx, []string{"stop"}, stdin, stdout, stderr); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		return runDaemon(ctx, []string{"start"}, stdin, stdout, stderr)
	case "status":
		return daemonStatus(ctx, stdout)
	default:
		return fmt.Errorf("unknown daemon subcommand %q", args[0])
	}
}

func daemonStart(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("daemon start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bind := fs.String("bind", daemon.DefaultBindAddress, "daemon bind address")
	insecure := fs.Bool("insecure", false, "allow non-loopback daemon bind after confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if status, err := client.Status(ctx); err == nil {
		fmt.Fprintf(stdout, "daemon already running at %s pid=%d\n", status.Endpoint, status.PID)
		return nil
	}
	if err := daemon.ValidateBind(*bind, *insecure, stdin, stdout); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(defaultHome(), "daemon.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	runArgs := []string{"daemon", "run", "--bind", *bind}
	if *insecure {
		runArgs = append(runArgs, "--insecure", "--confirmed-insecure")
	}
	cmd := exec.Command(exe, runArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = cmd.Process.Release()
	_ = logFile.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := client.Status(ctx)
		if err == nil {
			fmt.Fprintf(stdout, "daemon started at %s pid=%d\n", status.Endpoint, status.PID)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon process started but readiness check failed")
}

func daemonRun(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("daemon run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bind := fs.String("bind", daemon.DefaultBindAddress, "daemon bind address")
	insecure := fs.Bool("insecure", false, "allow non-loopback daemon bind after confirmation")
	confirmedInsecure := fs.Bool("confirmed-insecure", false, "internal: non-loopback bind was confirmed by daemon start")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*confirmedInsecure {
		if err := daemon.ValidateBind(*bind, *insecure, stdin, stdout); err != nil {
			return err
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	server := daemon.NewServer(daemon.ServerOptions{Home: defaultHome(), Executable: exe, Bind: *bind})
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.ListenAndServe(sigCtx)
}

func daemonStatus(ctx context.Context, stdout io.Writer) error {
	client, err := daemonClient()
	if err != nil {
		return err
	}
	status, err := client.Status(ctx)
	if err != nil {
		fmt.Fprintln(stdout, "daemon: stopped")
		return nil
	}
	fmt.Fprintf(stdout, "daemon: running\nendpoint: %s\npid: %d\nassistants: %d\nrunning: %d\nshutdown_policy: %s\n", status.Endpoint, status.PID, status.AssistantCount, status.RunningCount, status.ShutdownPolicy)
	return nil
}

func runConsole(ctx context.Context, args []string, stdout io.Writer) error {
	client, err := daemonClient()
	if err != nil {
		return err
	}
	status, err := client.EnsureRunning(ctx, "")
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "console: %s/\n", strings.TrimRight(status.Endpoint, "/"))
	return nil
}

func assistantCreate(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("assistant create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "assistant name")
	id := fs.String("id", "", "assistant id")
	rootPath := fs.String("root", "", "assistant root path containing workspace and config")
	workspacePath := fs.String("workspace", "", "workspace path")
	configDir := fs.String("configspace", "", "configspace path")
	providerRaw := fs.String("harness", "codex", "harness provider: codex or claude")
	command := fs.String("command", "", "ACP adapter command")
	var adapterArgs multiFlag
	fs.Var(&adapterArgs, "arg", "ACP adapter argument")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" && strings.TrimSpace(*id) == "" {
		return fmt.Errorf("--name or --id is required")
	}
	assistantID := strings.TrimSpace(*id)
	if assistantID == "" {
		assistantID = slug(*name)
	}
	if *name == "" {
		*name = assistantID
	}
	root := strings.TrimSpace(*rootPath)
	if root == "" {
		root = filepath.Join(defaultHome(), "assistants", assistantID)
	}
	if *workspacePath == "" {
		*workspacePath = filepath.Join(root, "workspace")
	}
	if *configDir == "" {
		*configDir = filepath.Join(root, "config")
	}
	provider := model.HarnessProvider(*providerRaw)
	defaultCommand, defaultArgs, err := harnesspkg.DefaultCommand(provider)
	if err != nil {
		return err
	}
	if *command == "" {
		*command = defaultCommand
	}
	argsValue := []string(adapterArgs)
	if len(argsValue) == 0 {
		argsValue = defaultArgs
	}
	cfg := model.AssistantConfig{
		ID:              assistantID,
		Name:            *name,
		WorkspacePath:   absPath(*workspacePath),
		ConfigspacePath: absPath(*configDir),
		Harness:         model.HarnessBinding{Provider: provider, Command: *command, Args: argsValue},
		Memory:          model.DefaultMemoryConfig(),
		EventDBPath:     filepath.Join(absPath(*configDir), configspace.EventsDBFile),
		Autostart:       true,
	}
	if err := configspace.InitializeGlobal(defaultHome()); err != nil {
		return err
	}
	if err := configspace.Initialize(ctx, cfg); err != nil {
		return err
	}
	if err := registerAssistant(cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created assistant %s\nconfigspace: %s\nworkspace: %s\n", cfg.ID, cfg.ConfigspacePath, cfg.WorkspacePath)
	return nil
}

func assistantList(args []string, stdout io.Writer) error {
	registry, err := loadRegistry()
	if err != nil {
		return err
	}
	sort.Slice(registry.Assistants, func(i, j int) bool { return registry.Assistants[i].ID < registry.Assistants[j].ID })
	fmt.Fprintln(stdout, "ID\tNAME\tCONFIGSPACE\tWORKSPACE")
	for _, item := range registry.Assistants {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.ID, item.Name, item.ConfigspacePath, item.WorkspacePath)
	}
	return nil
}

func assistantInspect(ctx context.Context, args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return err
	}
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	status, err := db.StatusSnapshot(ctx, cfg.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "id: %s\nname: %s\nharness: %s\nworkspace: %s\nconfigspace: %s\nchannels: %d\nactive_sessions: %d\npending_permissions: %d\n",
		cfg.ID, cfg.Name, cfg.Harness.Provider, cfg.WorkspacePath, cfg.ConfigspacePath, len(channels), status.ActiveSessions, status.PendingPermissions)
	return nil
}

func assistantStart(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	if hasArg(args, "--foreground") {
		return assistantServe(context.Background(), []string{"--configspace", configDir}, stdout, stderr)
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if _, err := client.EnsureRunning(ctx, ""); err != nil {
		return err
	}
	state, err := client.StartAssistant(ctx, configDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "started assistant %s pid=%d\n", state.ID, state.PID)
	return nil
}

func assistantServe(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return err
	}
	if err := configspace.InitializeGlobal(defaultHome()); err != nil {
		return err
	}
	if err := configspace.EnsureAssistantSources(cfg); err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	policies, err := configspace.LoadPolicies(configDir)
	if err != nil {
		return err
	}
	mem := workspace.NewMemoryManager(cfg.ID, cfg.WorkspacePath, cfg.Memory, db)
	if err := mem.InitSkeletons(); err != nil {
		return err
	}
	h := newRuntimeHarness(cfg, db)
	sender := newConnectorSender()
	h.sender = sender
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		return err
	}
	channelOptions := map[string]map[string]string{}
	for _, channel := range channels {
		key := string(channel.Platform) + "/" + channel.AccountID
		channelOptions[key] = channel.Options
	}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{AssistantID: cfg.ID, Provider: cfg.Harness.Provider, Store: db, Harness: h, Sender: sender, Policy: policies, Memory: mem, ChannelOptions: channelOptions, ACPAHome: defaultHome(), ConfigspacePath: cfg.ConfigspacePath})
	h.onPermission = func(ctx context.Context, localSessionID, acpRequestID string, options []string) (model.PendingPermission, error) {
		return rt.RecordPermissionRequest(ctx, assistant.PermissionRequest{LocalSessionID: localSessionID, ACPRequestID: acpRequestID, Options: options, TimeoutResolution: "reject"})
	}
	var accounts []im.Account
	for _, channel := range channels {
		secrets, err := configspace.ResolveSecrets(channel.Credentials)
		if err != nil {
			_ = db.UpsertConnectorStatus(ctx, model.ConnectorStatus{AssistantID: cfg.ID, Platform: channel.Platform, AccountID: channel.AccountID, State: model.ConnectorStateFailed, LastError: err.Error(), UpdatedAt: time.Now().UTC()})
			continue
		}
		accountCfg := im.AccountConfig{
			AssistantID:          cfg.ID,
			Channel:              channel,
			Secrets:              secrets,
			OnInbound:            rt.HandleInbound,
			OnPermissionDecision: rt.HandlePermissionDecision,
			OnStatus:             db.UpsertConnectorStatus,
		}
		var account im.Account
		switch channel.Platform {
		case model.PlatformFeishu:
			account = im.NewFeishuAccount(accountCfg)
		case model.PlatformQQBot:
			account = im.NewQQBotAccount(accountCfg)
		default:
			continue
		}
		accounts = append(accounts, account)
		if err := account.Start(ctx); err != nil {
			_ = db.UpsertConnectorStatus(ctx, model.ConnectorStatus{AssistantID: cfg.ID, Platform: channel.Platform, AccountID: channel.AccountID, State: model.ConnectorStateFailed, LastError: err.Error(), UpdatedAt: time.Now().UTC()})
		}
		sender.Register(account)
	}
	_ = db.RecordEvent(ctx, model.Event{AssistantID: cfg.ID, Type: model.EventLifecycle, Scope: "assistant", Message: "started", At: time.Now().UTC()})
	fmt.Fprintf(stdout, "assistant %s serving with %d connector account(s)\n", cfg.ID, len(accounts))
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopCron := rt.StartCronScheduler(sigCtx, time.Minute)
	defer stopCron()
	permissionTicker := time.NewTicker(30 * time.Second)
	defer permissionTicker.Stop()
	go func() {
		for {
			select {
			case <-sigCtx.Done():
				return
			case <-permissionTicker.C:
				_ = rt.ExpirePermissions(context.Background(), time.Now().UTC())
			}
		}
	}()
	<-sigCtx.Done()
	for _, account := range accounts {
		_ = account.Stop(context.Background())
	}
	_ = h.Stop()
	_ = db.RecordEvent(context.Background(), model.Event{AssistantID: cfg.ID, Type: model.EventLifecycle, Scope: "assistant", Message: "stopped", At: time.Now().UTC()})
	return nil
}

func assistantStop(ctx context.Context, args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if _, err := client.EnsureRunning(ctx, ""); err != nil {
		return err
	}
	state, err := client.StopAssistant(ctx, configDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "stopped assistant %s\n", state.ID)
	return nil
}

func assistantRestart(ctx context.Context, args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if _, err := client.EnsureRunning(ctx, ""); err != nil {
		return err
	}
	state, err := client.RestartAssistant(ctx, configDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "restarted assistant %s pid=%d\n", state.ID, state.PID)
	return nil
}

func assistantLifecycleStatus(ctx context.Context, args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if _, err := client.EnsureRunning(ctx, ""); err != nil {
		return err
	}
	state, err := client.AssistantStatus(ctx, configDir)
	if err != nil {
		return err
	}
	running := "stopped"
	if state.Running {
		running = fmt.Sprintf("running pid=%d", state.PID)
	}
	fmt.Fprintf(stdout, "id: %s\nstatus: %s\nautostart: %t\nlast_error: %s\n", state.ID, running, state.Autostart, state.LastError)
	return nil
}

func assistantAutostart(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: acpa assistant autostart enable|disable <assistant-id|--configspace PATH>")
	}
	enabled := false
	switch args[0] {
	case "enable":
		enabled = true
	case "disable":
		enabled = false
	default:
		return fmt.Errorf("autostart action must be enable or disable")
	}
	configDir, err := resolveConfigspace(args[1:])
	if err != nil {
		return err
	}
	client, err := daemonClient()
	if err != nil {
		return err
	}
	if _, err := client.EnsureRunning(ctx, ""); err != nil {
		return err
	}
	state, err := client.SetAutostart(ctx, configDir, enabled)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "assistant %s autostart=%t\n", state.ID, state.Autostart)
	return nil
}

func assistantRemove(args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return err
	}
	if err := unregisterAssistant(cfg.ID); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "removed assistant %s from registry; configspace left at %s\n", cfg.ID, cfg.ConfigspacePath)
	return nil
}

func channelAdd(ctx context.Context, platformRaw string, args []string, stdin io.Reader, stdout io.Writer) error {
	platform := model.Platform(platformRaw)
	if platform != model.PlatformFeishu && platform != model.PlatformQQBot {
		return fmt.Errorf("unsupported platform %q", platformRaw)
	}
	fs := flag.NewFlagSet("channel add "+platformRaw, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rootPath := fs.String("root", "", "assistant root path containing workspace and config")
	configDir := fs.String("configspace", "", "configspace path")
	id := fs.String("id", "", "channel id")
	accountID := fs.String("account-id", "main", "account id")
	displayName := fs.String("name", "", "display name")
	setupURL := fs.String("setup-url", "", "setup URL")
	domain := fs.String("domain", "feishu", "Feishu/Lark domain: feishu or lark")
	manual := fs.Bool("manual", false, "skip QR registration and use explicit credential flags")
	appIDEnv := fs.String("app-id-env", "", "app id env var")
	appSecretEnv := fs.String("app-secret-env", "", "app secret env var")
	appIDFile := fs.String("app-id-file", "", "app id file")
	appSecretFile := fs.String("app-secret-file", "", "app secret file")
	websocketURL := fs.String("websocket-url", "", "gateway websocket URL")
	registrationBaseURL := fs.String("registration-base-url", "", "override Feishu accounts base URL")
	openBaseURL := fs.String("open-base-url", "", "override Feishu open platform base URL")
	onboardingTimeout := fs.Int("onboarding-timeout", 600, "QR onboarding timeout seconds")
	enabled := fs.Bool("enabled", true, "enable channel")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configDir == "" && *rootPath != "" {
		*configDir = filepath.Join(*rootPath, "config")
	}
	if *configDir == "" {
		return fmt.Errorf("--root or --configspace is required")
	}
	reader := bufio.NewReader(stdin)
	if *id == "" {
		*id = promptDefault(reader, stdout, "channel id", string(platform)+"-"+*accountID)
	}
	if *displayName == "" {
		*displayName = promptDefault(reader, stdout, "display name", *id)
	}
	credentials := map[string]model.SecretRef{}
	options := map[string]string{"connection_mode": "websocket"}
	if platform == model.PlatformFeishu {
		options["domain"] = *domain
		if strings.TrimSpace(*openBaseURL) != "" {
			options["open_base_url"] = strings.TrimRight(*openBaseURL, "/")
		}
	}
	hasManualCredentials := *appIDEnv != "" || *appIDFile != "" || *appSecretEnv != "" || *appSecretFile != ""
	if platform == model.PlatformFeishu && !*manual && !hasManualCredentials {
		result, err := runFeishuQRRegistration(ctx, *domain, *registrationBaseURL, *openBaseURL, *onboardingTimeout, stdout)
		if err != nil {
			return err
		}
		appIDPath := filepath.Join(*configDir, "secrets", *id+".app_id")
		appSecretPath := filepath.Join(*configDir, "secrets", *id+".app_secret")
		if err := writeSecretFile(appIDPath, result.AppID); err != nil {
			return err
		}
		if err := writeSecretFile(appSecretPath, result.AppSecret); err != nil {
			return err
		}
		credentials["app_id"] = model.SecretRef{Type: model.SecretFile, Path: appIDPath}
		credentials["app_secret"] = model.SecretRef{Type: model.SecretFile, Path: appSecretPath}
		options["domain"] = result.Domain
		options["owner_open_id"] = result.OpenID
		options["bot_open_id"] = result.BotOpenID
		options["bot_name"] = result.BotName
	} else if *appIDEnv != "" {
		credentials["app_id"] = model.SecretRef{Type: model.SecretEnv, Name: *appIDEnv}
	} else if *appIDFile != "" {
		credentials["app_id"] = model.SecretRef{Type: model.SecretFile, Path: *appIDFile}
	}
	if *appSecretEnv != "" {
		credentials["app_secret"] = model.SecretRef{Type: model.SecretEnv, Name: *appSecretEnv}
	} else if *appSecretFile != "" {
		credentials["app_secret"] = model.SecretRef{Type: model.SecretFile, Path: *appSecretFile}
	}
	if *websocketURL != "" {
		secretPath := filepath.Join(*configDir, "secrets", *id+".websocket_url")
		if err := os.MkdirAll(filepath.Dir(secretPath), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(secretPath, []byte(*websocketURL+"\n"), 0o600); err != nil {
			return err
		}
		credentials["websocket_url"] = model.SecretRef{Type: model.SecretFile, Path: secretPath}
	}
	channel := model.ChannelConfig{
		ID:               *id,
		Platform:         platform,
		AccountID:        *accountID,
		DisplayName:      *displayName,
		Enabled:          *enabled,
		Credentials:      credentials,
		Options:          options,
		SetupURL:         *setupURL,
		RejectNonPrivate: true,
		TextChunkLimit:   3500,
		TokenCachePath:   filepath.Join(*configDir, "secrets", *id+".token"),
	}
	if err := configspace.SaveChannel(*configDir, channel); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s channel %s\n", platform, channel.ID)
	if platform != model.PlatformFeishu || *manual || hasManualCredentials {
		printOnboarding(platform, *setupURL, stdout)
	}
	_, _ = ctx, reader
	return nil
}

func runFeishuQRRegistration(ctx context.Context, domain, registrationBaseURL, openBaseURL string, timeout int, stdout io.Writer) (im.FeishuRegistrationResult, error) {
	client := im.FeishuRegistrationClient{AccountsBaseURL: registrationBaseURL, OpenBaseURL: openBaseURL}
	begin, err := client.Begin(ctx, im.FeishuRegistrationOptions{Domain: domain, TimeoutSeconds: timeout})
	if err != nil {
		return im.FeishuRegistrationResult{}, err
	}
	fmt.Fprintln(stdout, "Scan the QR code with Feishu to create and configure the bot app.")
	if begin.QRURL != "" {
		if code, err := qrcode.New(begin.QRURL, qrcode.Medium); err == nil {
			fmt.Fprintln(stdout, code.ToString(false))
		}
		fmt.Fprintf(stdout, "URL: %s\n", begin.QRURL)
	}
	if begin.UserCode != "" {
		fmt.Fprintf(stdout, "User code: %s\n", begin.UserCode)
	}
	result, err := client.Poll(ctx, begin)
	if err != nil {
		return im.FeishuRegistrationResult{}, err
	}
	fmt.Fprintf(stdout, "Feishu app registered: %s\n", result.AppID)
	if result.BotName != "" {
		fmt.Fprintf(stdout, "Bot: %s\n", result.BotName)
	}
	return result, nil
}

func writeSecretFile(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o600)
}

func channelStatus(ctx context.Context, args []string, stdout io.Writer) error {
	configDir, err := resolveConfigspace(args)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return err
	}
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	statuses, err := db.ConnectorStatuses(ctx, cfg.ID)
	if err != nil {
		return err
	}
	statusByKey := map[string]model.ConnectorStatus{}
	for _, status := range statuses {
		statusByKey[string(status.Platform)+"/"+status.AccountID] = status
	}
	fmt.Fprintln(stdout, "PLATFORM\tACCOUNT\tENABLED\tSTATE\tERROR")
	for _, channel := range channels {
		status := statusByKey[string(channel.Platform)+"/"+channel.AccountID]
		state := string(status.State)
		if state == "" {
			state = "unknown"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%t\t%s\t%s\n", channel.Platform, channel.AccountID, channel.Enabled, state, status.LastError)
	}
	return nil
}

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	opts, err := parseTopLevelOptions(args, true)
	if err != nil {
		return err
	}
	resolved, err := resolveConfigspaceFromFlags(opts.configspace, opts.root, opts.positional)
	if err != nil {
		return err
	}
	report := diagnostics.Collect(ctx, diagnostics.Options{ConfigspacePath: resolved, HomePath: defaultHome()})
	if opts.jsonOutput {
		return diagnostics.RenderJSON(stdout, report)
	}
	return diagnostics.RenderText(stdout, report, opts.verbose)
}

func runStatus(ctx context.Context, args []string, stdout io.Writer) error {
	opts, err := parseTopLevelOptions(args, false)
	if err != nil {
		return err
	}
	resolved, err := resolveConfigspaceFromFlags(opts.configspace, opts.root, opts.positional)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(resolved)
	if err != nil {
		return err
	}
	channels, err := configspace.LoadChannels(resolved)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	snapshot, err := db.StatusSnapshot(ctx, cfg.ID)
	if err != nil {
		return err
	}
	statusByKey := map[string]model.ConnectorStatus{}
	for _, status := range snapshot.Connectors {
		statusByKey[string(status.Platform)+"/"+status.AccountID] = status
	}
	fmt.Fprintf(stdout, "id: %s\nname: %s\nharness: %s\nworkspace: %s\nconfigspace: %s\nchannels: %d\nactive_sessions: %d\npending_permissions: %d\nrecent_errors: %d\n",
		cfg.ID, cfg.Name, cfg.Harness.Provider, cfg.WorkspacePath, cfg.ConfigspacePath, len(channels), snapshot.ActiveSessions, snapshot.PendingPermissions, len(snapshot.RecentErrors))
	if len(channels) > 0 {
		fmt.Fprintln(stdout, "connectors:")
		for _, channel := range channels {
			status := statusByKey[string(channel.Platform)+"/"+channel.AccountID]
			state := string(status.State)
			if state == "" {
				state = "unknown"
			}
			fmt.Fprintf(stdout, "- %s/%s enabled=%t state=%s", channel.Platform, channel.AccountID, channel.Enabled, state)
			if status.LastError != "" {
				fmt.Fprintf(stdout, " error=%s", status.LastError)
			}
			fmt.Fprintln(stdout)
		}
	}
	return nil
}

func runLogs(ctx context.Context, args []string, stdout io.Writer) error {
	opts, err := parseTopLevelOptions(args, false)
	if err != nil {
		return err
	}
	configDir, err := resolveConfigspaceFromFlags(opts.configspace, opts.root, opts.positional)
	if err != nil {
		return err
	}
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	var lastID int64
	recent, err := db.RecentEvents(ctx, cfg.ID, opts.lines)
	if err != nil {
		return err
	}
	for i := len(recent) - 1; i >= 0; i-- {
		event := recent[i]
		if event.ID > lastID {
			lastID = event.ID
		}
		printLogEvent(stdout, event)
	}
	if !opts.follow {
		return nil
	}
	for {
		events, err := db.EventsAfter(ctx, cfg.ID, lastID, 100)
		if err != nil {
			return err
		}
		for _, event := range events {
			if event.ID > lastID {
				lastID = event.ID
			}
			printLogEvent(stdout, event)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func printLogEvent(stdout io.Writer, event model.Event) {
	fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", event.At.Format(time.RFC3339), event.Type, event.Scope, event.Message)
}

type runtimeHarness struct {
	cfg             model.AssistantConfig
	store           *store.Store
	sender          assistant.Sender
	mu              sync.Mutex
	runtimes        map[string]*acp.Runtime
	sessionProfiles map[string]harnesspkg.LaunchProfile
	acpToLocal      map[string]string
	permissionReply map[string]func(string) error
	onPermission    func(context.Context, string, string, []string) (model.PendingPermission, error)
}

func newRuntimeHarness(cfg model.AssistantConfig, db *store.Store) *runtimeHarness {
	return &runtimeHarness{
		cfg:             cfg,
		store:           db,
		runtimes:        map[string]*acp.Runtime{},
		sessionProfiles: map[string]harnesspkg.LaunchProfile{},
		acpToLocal:      map[string]string{},
		permissionReply: map[string]func(string) error{},
	}
}

func (h *runtimeHarness) EnsureSession(ctx context.Context, req assistant.EnsureSessionRequest) (assistant.EnsureSessionResult, error) {
	profile, err := h.launchProfile(req.PermissionMode)
	if err != nil {
		return assistant.EnsureSessionResult{}, err
	}
	h.mu.Lock()
	h.sessionProfiles[req.LocalSessionID] = profile
	knownACPSession := req.CurrentACPSession != "" && h.acpToLocal[req.CurrentACPSession] != ""
	h.mu.Unlock()
	if knownACPSession {
		h.rememberACPSession(req.CurrentACPSession, req.LocalSessionID)
		return assistant.EnsureSessionResult{ACPSessionID: req.CurrentACPSession, ExternalSessionID: req.ExternalSessionID}, nil
	}
	runtime, err := h.runtime(ctx, profile)
	if err != nil {
		return assistant.EnsureSessionResult{}, err
	}
	if req.ExternalSessionID != "" && runtime.Capabilities().Session.LoadSession {
		sessionID, err := runtime.LoadSession(ctx, req.ExternalSessionID)
		if err == nil {
			h.rememberACPSession(sessionID, req.LocalSessionID)
			return assistant.EnsureSessionResult{ACPSessionID: sessionID, ExternalSessionID: req.ExternalSessionID}, nil
		}
	}
	sessionID, err := runtime.NewSession(ctx)
	if err != nil {
		return assistant.EnsureSessionResult{}, err
	}
	h.rememberACPSession(sessionID, req.LocalSessionID)
	return assistant.EnsureSessionResult{ACPSessionID: sessionID, ExternalSessionID: sessionID}, nil
}

func (h *runtimeHarness) Prompt(ctx context.Context, req assistant.PromptRequest) (assistant.PromptResult, error) {
	h.mu.Lock()
	profile, ok := h.sessionProfiles[req.LocalSessionID]
	h.mu.Unlock()
	if !ok {
		var err error
		profile, err = h.launchProfile(model.PermissionManual)
		if err != nil {
			return assistant.PromptResult{}, err
		}
	}
	runtime, err := h.runtime(ctx, profile)
	if err != nil {
		return assistant.PromptResult{}, err
	}
	finalText, err := runtime.Prompt(ctx, req.ACPSessionID, req.Text)
	if err != nil {
		return assistant.PromptResult{}, err
	}
	return assistant.PromptResult{FinalText: finalText}, nil
}

func (h *runtimeHarness) SwitchMode(ctx context.Context, req assistant.SwitchModeRequest) (assistant.SwitchModeResult, error) {
	profile, err := h.launchProfile(req.Mode)
	if err != nil {
		return assistant.SwitchModeResult{}, err
	}
	h.mu.Lock()
	h.sessionProfiles[req.LocalSessionID] = profile
	h.mu.Unlock()
	runtime, err := h.runtime(ctx, profile)
	if err != nil {
		return assistant.SwitchModeResult{}, err
	}
	sessionID := req.ExternalSessionID
	if sessionID != "" && runtime.Capabilities().Session.LoadSession {
		loaded, err := runtime.LoadSession(ctx, sessionID)
		if err == nil {
			h.rememberACPSession(loaded, req.LocalSessionID)
			return assistant.SwitchModeResult{ACPSessionID: loaded, LaunchProfileKey: profile.Key}, nil
		}
	}
	created, err := runtime.NewSession(ctx)
	if err != nil {
		return assistant.SwitchModeResult{}, err
	}
	h.rememberACPSession(created, req.LocalSessionID)
	return assistant.SwitchModeResult{ACPSessionID: created, LaunchProfileKey: profile.Key}, nil
}

func (h *runtimeHarness) ResolvePermission(ctx context.Context, shortID, option string) error {
	h.mu.Lock()
	reply := h.permissionReply[strings.ToUpper(shortID)]
	delete(h.permissionReply, strings.ToUpper(shortID))
	h.mu.Unlock()
	if reply == nil {
		return nil
	}
	if option == "reject" {
		return reply("reject")
	}
	return reply(option)
}

func (h *runtimeHarness) launchProfile(mode model.PermissionMode) (harnesspkg.LaunchProfile, error) {
	if strings.TrimSpace(h.cfg.ConfigspacePath) == "" {
		return harnesspkg.ResolveLaunchProfile(h.cfg.Harness.Provider, mode, harnesspkg.ProfileOptions{Command: h.cfg.Harness.Command, Args: h.cfg.Harness.Args})
	}
	overlay, err := harnesspkg.PrepareOverlay(h.cfg, defaultHome())
	if err != nil {
		return harnesspkg.LaunchProfile{}, err
	}
	return harnesspkg.ResolveLaunchProfile(h.cfg.Harness.Provider, mode, harnesspkg.ProfileOptions{
		Command:         h.cfg.Harness.Command,
		Args:            h.cfg.Harness.Args,
		Env:             overlay.Env,
		ClaudePluginDir: overlay.ClaudePluginDir,
		PromptPrefix:    overlay.PromptPrefix,
		ProcessDir:      overlay.ProcessDir,
	})
}

func (h *runtimeHarness) Stop() error {
	for _, runtime := range h.runtimes {
		runtime.Stop()
	}
	return nil
}

func (h *runtimeHarness) runtime(ctx context.Context, profile harnesspkg.LaunchProfile) (*acp.Runtime, error) {
	h.mu.Lock()
	runtime := h.runtimes[profile.Key]
	if runtime == nil {
		runtime = acp.NewRuntime(acp.Config{
			Command:      profile.Command,
			Args:         profile.Args,
			Env:          profile.Env,
			Workspace:    h.cfg.WorkspacePath,
			ProcessDir:   profile.ProcessDir,
			PromptPrefix: profile.PromptPrefix,
			EffortLevel:  profile.EffortLevel,
			OnEvent:      h.handleACPEvent,
			OnRequest:    h.handleACPRequest,
			OnPromptText: h.handleACPPromptText,
		})
		h.runtimes[profile.Key] = runtime
	}
	h.mu.Unlock()
	if err := runtime.Start(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (h *runtimeHarness) rememberACPSession(acpSessionID, localSessionID string) {
	if acpSessionID == "" || localSessionID == "" {
		return
	}
	h.mu.Lock()
	h.acpToLocal[acpSessionID] = localSessionID
	h.mu.Unlock()
}

func (h *runtimeHarness) handleACPPromptText(acpSessionID, text string) {
	if strings.TrimSpace(text) == "" || h.sender == nil || h.store == nil {
		return
	}
	h.mu.Lock()
	localSessionID := h.acpToLocal[acpSessionID]
	h.mu.Unlock()
	if localSessionID == "" {
		return
	}
	session, err := h.store.SessionByID(context.Background(), localSessionID)
	if err != nil {
		return
	}
	_ = h.sender.Send(context.Background(), model.OutboundMessage{
		AssistantID:      session.Binding.AssistantID,
		Platform:         session.Binding.Platform,
		AccountID:        session.Binding.AccountID,
		PrivateChannelID: session.Binding.PrivateChannelID,
		PlatformUserID:   session.Binding.PlatformUserID,
		Text:             text,
		CreatedAt:        time.Now().UTC(),
	})
}

func (h *runtimeHarness) handleACPEvent(event acp.Event) {
	if h.store == nil {
		return
	}
	_ = h.store.RecordEvent(context.Background(), model.Event{
		AssistantID: h.cfg.ID,
		Type:        model.EventACP,
		Scope:       event.Method,
		Message:     event.Method,
		At:          time.Now().UTC(),
	})
}

func (h *runtimeHarness) handleACPRequest(req acp.Request) bool {
	if req.Method != "session/request_permission" {
		h.handleACPEvent(acp.Event{Method: req.Method, Params: req.Params})
		return false
	}
	var payload struct {
		SessionID string `json:"sessionId"`
		Options   []struct {
			ID       string `json:"id"`
			OptionID string `json:"optionId"`
		} `json:"options"`
	}
	if err := json.Unmarshal(req.Params, &payload); err != nil {
		_ = req.Respond(cancelledPermissionResponse())
		return true
	}
	h.mu.Lock()
	localSessionID := h.acpToLocal[payload.SessionID]
	h.mu.Unlock()
	if localSessionID == "" || h.onPermission == nil {
		_ = req.Respond(cancelledPermissionResponse())
		return true
	}
	options := make([]string, 0, len(payload.Options))
	for _, option := range payload.Options {
		id := option.ID
		if id == "" {
			id = option.OptionID
		}
		if id != "" {
			options = append(options, id)
		}
	}
	permission, err := h.onPermission(context.Background(), localSessionID, req.ID, options)
	if err != nil {
		_ = req.Respond(cancelledPermissionResponse())
		return true
	}
	h.mu.Lock()
	h.permissionReply[strings.ToUpper(permission.ShortApprovalID)] = func(option string) error {
		return req.Respond(selectedPermissionResponse(option))
	}
	h.mu.Unlock()
	return true
}

func selectedPermissionResponse(optionID string) map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optionID}}
}

func cancelledPermissionResponse() map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

type connectorSender struct {
	mu       sync.Mutex
	accounts map[string]im.Account
}

func newConnectorSender() *connectorSender {
	return &connectorSender{accounts: map[string]im.Account{}}
}

func (s *connectorSender) Register(account im.Account) {
	status := account.Status()
	key := string(status.Platform) + "/" + status.AccountID
	s.mu.Lock()
	s.accounts[key] = account
	s.mu.Unlock()
}

func (s *connectorSender) Send(ctx context.Context, msg model.OutboundMessage) error {
	key := string(msg.Platform) + "/" + msg.AccountID
	s.mu.Lock()
	account := s.accounts[key]
	s.mu.Unlock()
	if account == nil {
		return fmt.Errorf("connector account %s is not registered", key)
	}
	return account.Send(ctx, msg)
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type registry struct {
	Assistants []registryEntry `yaml:"assistants"`
}

type registryEntry struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	ConfigspacePath string `yaml:"configspace_path"`
	WorkspacePath   string `yaml:"workspace_path"`
	CreatedAt       string `yaml:"created_at"`
}

func registerAssistant(cfg model.AssistantConfig) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}
	entry := registryEntry{ID: cfg.ID, Name: cfg.Name, ConfigspacePath: cfg.ConfigspacePath, WorkspacePath: cfg.WorkspacePath, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	replaced := false
	for i := range reg.Assistants {
		if reg.Assistants[i].ID == cfg.ID {
			reg.Assistants[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		reg.Assistants = append(reg.Assistants, entry)
	}
	return saveRegistry(reg)
}

func unregisterAssistant(id string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}
	var next []registryEntry
	for _, entry := range reg.Assistants {
		if entry.ID != id {
			next = append(next, entry)
		}
	}
	reg.Assistants = next
	return saveRegistry(reg)
}

func loadRegistry() (registry, error) {
	path := registryPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return registry{}, nil
	}
	if err != nil {
		return registry{}, err
	}
	var reg registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return registry{}, err
	}
	return reg, nil
}

func saveRegistry(reg registry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	path := registryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resolveConfigspace(args []string) (string, error) {
	for i, arg := range args {
		if arg == "--configspace" && i+1 < len(args) {
			return absPath(args[i+1]), nil
		}
		if strings.HasPrefix(arg, "--configspace=") {
			return absPath(strings.TrimPrefix(arg, "--configspace=")), nil
		}
		if arg == "--root" && i+1 < len(args) {
			return absPath(filepath.Join(args[i+1], "config")), nil
		}
		if strings.HasPrefix(arg, "--root=") {
			return absPath(filepath.Join(strings.TrimPrefix(arg, "--root="), "config")), nil
		}
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		reg, err := loadRegistry()
		if err != nil {
			return "", err
		}
		for _, entry := range reg.Assistants {
			if entry.ID == arg || entry.Name == arg {
				return entry.ConfigspacePath, nil
			}
		}
		if _, err := os.Stat(filepath.Join(arg, configspace.AssistantFile)); err == nil {
			return absPath(arg), nil
		}
	}
	return "", fmt.Errorf("--configspace or assistant id is required")
}

func resolveConfigspaceFromFlags(configDir, rootPath string, positional []string) (string, error) {
	if strings.TrimSpace(configDir) != "" {
		return absPath(configDir), nil
	}
	if strings.TrimSpace(rootPath) != "" {
		return absPath(filepath.Join(rootPath, "config")), nil
	}
	return resolveConfigspace(positional)
}

type topLevelOptions struct {
	configspace string
	root        string
	verbose     bool
	jsonOutput  bool
	follow      bool
	lines       int
	positional  []string
}

func parseTopLevelOptions(args []string, allowDoctorFlags bool) (topLevelOptions, error) {
	opts := topLevelOptions{lines: 100}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--configspace":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--configspace requires a path")
			}
			opts.configspace = args[i]
		case strings.HasPrefix(arg, "--configspace="):
			opts.configspace = strings.TrimPrefix(arg, "--configspace=")
		case arg == "--root":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--root requires a path")
			}
			opts.root = args[i]
		case strings.HasPrefix(arg, "--root="):
			opts.root = strings.TrimPrefix(arg, "--root=")
		case arg == "--verbose" && allowDoctorFlags:
			opts.verbose = true
		case arg == "--json" && allowDoctorFlags:
			opts.jsonOutput = true
		case arg == "--follow":
			opts.follow = true
		case arg == "--lines":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--lines requires a number")
			}
			lines, err := strconv.Atoi(args[i])
			if err != nil || lines < 0 {
				return opts, fmt.Errorf("--lines must be a non-negative number")
			}
			opts.lines = lines
		case strings.HasPrefix(arg, "--lines="):
			lines, err := strconv.Atoi(strings.TrimPrefix(arg, "--lines="))
			if err != nil || lines < 0 {
				return opts, fmt.Errorf("--lines must be a non-negative number")
			}
			opts.lines = lines
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %s", arg)
		default:
			opts.positional = append(opts.positional, arg)
		}
	}
	return opts, nil
}

func hasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func promptDefault(reader *bufio.Reader, stdout io.Writer, label, fallback string) string {
	fmt.Fprintf(stdout, "%s [%s]: ", label, fallback)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback
	}
	return line
}

func printOnboarding(platform model.Platform, setupURL string, stdout io.Writer) {
	switch platform {
	case model.PlatformFeishu:
		fmt.Fprintln(stdout, "Feishu setup: enable bot private messages, event subscription, and app credentials in Feishu Open Platform.")
	case model.PlatformQQBot:
		fmt.Fprintln(stdout, "QQ Bot setup: enable C2C messaging and gateway intents in QQ Bot management console.")
	}
	if setupURL == "" {
		fmt.Fprintln(stdout, "manual credential fallback: provide app_id and app_secret as env or file secret references.")
		return
	}
	fmt.Fprintln(stdout, setupURL)
	if code, err := qrcode.New(setupURL, qrcode.Medium); err == nil {
		fmt.Fprintln(stdout, code.ToString(false))
	}
}

func printUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, `Usage:
  acpa assistant create --name NAME [--root PATH] [--harness codex|claude]
  acpa assistant list
  acpa assistant inspect <assistant-id|--root PATH|--configspace PATH>
  acpa assistant start <assistant-id|--root PATH|--configspace PATH> [--foreground]
  acpa assistant stop <assistant-id|--root PATH|--configspace PATH>
  acpa assistant restart <assistant-id|--root PATH|--configspace PATH>
  acpa assistant status <assistant-id|--root PATH|--configspace PATH>
  acpa assistant autostart enable|disable <assistant-id|--root PATH|--configspace PATH>
  acpa daemon start|stop|restart|status
  acpa console
  acpa channel add feishu|qqbot --root PATH|--configspace PATH [credential flags]
  acpa channel status <assistant-id|--root PATH|--configspace PATH>
  acpa doctor <assistant-id|--root PATH|--configspace PATH> [--verbose|--json]
  acpa status <assistant-id|--root PATH|--configspace PATH>
  acpa logs <assistant-id|--root PATH|--configspace PATH> [--lines N] [--follow]`)
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

func defaultHome() string {
	if home := os.Getenv("ACPA_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".acpa"
	}
	return filepath.Join(userHome, ".acpa")
}

func daemonClient() (daemon.Client, error) {
	exe, err := os.Executable()
	if err != nil {
		return daemon.Client{}, err
	}
	return daemon.Client{Home: defaultHome(), Executable: exe}, nil
}

func registryPath() string {
	return filepath.Join(defaultHome(), "assistants.yaml")
}

func absPath(path string) string {
	if path == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
