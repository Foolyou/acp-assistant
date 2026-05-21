package assistant

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	harnesspkg "github.com/Foolyou/acp-assistant/internal/harness"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
	"github.com/Foolyou/acp-assistant/internal/workspace"
)

type Harness interface {
	EnsureSession(context.Context, EnsureSessionRequest) (EnsureSessionResult, error)
	Prompt(context.Context, PromptRequest) (PromptResult, error)
	SwitchMode(context.Context, SwitchModeRequest) (SwitchModeResult, error)
}

type PermissionResolver interface {
	ResolvePermission(context.Context, string, string) error
}

type Sender interface {
	Send(context.Context, model.OutboundMessage) error
}

type RuntimeConfig struct {
	AssistantID     string
	Provider        model.HarnessProvider
	Store           *store.Store
	Harness         Harness
	Sender          Sender
	Policy          model.PolicySet
	Memory          *workspace.MemoryManager
	ChannelOptions  map[string]map[string]string
	ACPAHome        string
	ConfigspacePath string
}

type Runtime struct {
	cfg RuntimeConfig
}

type EnsureSessionRequest struct {
	LocalSessionID    string
	Binding           model.SessionBindingKey
	PermissionMode    model.PermissionMode
	LaunchProfileKey  string
	CurrentACPSession string
	ExternalSessionID string
}

type EnsureSessionResult struct {
	ACPSessionID      string
	ExternalSessionID string
}

type PromptRequest struct {
	LocalSessionID string
	ACPSessionID   string
	Text           string
}

type PromptResult struct {
	FinalText string
}

type SwitchModeRequest struct {
	LocalSessionID    string
	CurrentACPSession string
	ExternalSessionID string
	Mode              model.PermissionMode
}

type SwitchModeResult struct {
	ACPSessionID     string
	LaunchProfileKey string
}

type PermissionRequest struct {
	LocalSessionID    string
	ACPRequestID      string
	Options           []string
	Timeout           time.Duration
	TimeoutResolution string
}

type commandErrorCategory string

const (
	commandErrorFailure          commandErrorCategory = "failure"
	commandErrorUnknown          commandErrorCategory = "unknown"
	commandErrorPermissionDenied commandErrorCategory = "permission_denied"
)

type commandResult struct {
	Text string
}

type commandError struct {
	Category commandErrorCategory
	Message  string
}

func (e commandError) Error() string {
	return e.Message
}

func NewRuntime(cfg RuntimeConfig) *Runtime {
	if cfg.Policy.Assistant.AllowedModes == nil {
		defaults := model.DefaultPolicySet()
		cfg.Policy.Assistant = defaults.Assistant
		if cfg.Policy.Accounts == nil {
			cfg.Policy.Accounts = defaults.Accounts
		}
		if cfg.Policy.Users == nil {
			cfg.Policy.Users = defaults.Users
		}
	}
	return &Runtime{cfg: cfg}
}

func (r *Runtime) HandleInbound(ctx context.Context, msg model.InboundMessage) error {
	if msg.AssistantID == "" {
		msg.AssistantID = r.cfg.AssistantID
	}
	key := msg.BindingKey()
	duplicate, err := r.cfg.Store.RememberIdempotency(ctx, msg.AssistantID, msg.Platform, msg.AccountID, msg.MessageID)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	if isCommandText(msg.Text) {
		result, err := r.handleCommand(ctx, msg)
		if err != nil {
			return r.sendCommandError(ctx, msg, err)
		}
		if strings.TrimSpace(result.Text) != "" {
			return r.sendCommandReply(ctx, msg, result.Text)
		}
		return nil
	}
	session, err := r.ensureActiveSession(ctx, key)
	if err != nil {
		return err
	}
	if r.cfg.Harness == nil {
		return fmt.Errorf("harness is not configured")
	}
	ensured, err := r.cfg.Harness.EnsureSession(ctx, EnsureSessionRequest{
		LocalSessionID:    session.ID,
		Binding:           key,
		PermissionMode:    session.PermissionMode,
		LaunchProfileKey:  session.LaunchProfileKey,
		CurrentACPSession: session.ACPSessionID,
		ExternalSessionID: session.ExternalSessionID,
	})
	if err != nil {
		return err
	}
	if ensured.ACPSessionID != "" || ensured.ExternalSessionID != "" {
		if err := r.cfg.Store.UpdateSessionACP(ctx, session.ID, ensured.ACPSessionID, ensured.ExternalSessionID); err != nil {
			return err
		}
		session.ACPSessionID = ensured.ACPSessionID
	}
	result, err := r.cfg.Harness.Prompt(ctx, PromptRequest{LocalSessionID: session.ID, ACPSessionID: session.ACPSessionID, Text: msg.Text})
	if err != nil {
		return err
	}
	if r.cfg.Sender != nil && strings.TrimSpace(result.FinalText) != "" {
		return r.cfg.Sender.Send(ctx, model.OutboundMessage{
			AssistantID:      msg.AssistantID,
			Platform:         msg.Platform,
			AccountID:        msg.AccountID,
			PrivateChannelID: msg.PrivateChannelID,
			PlatformUserID:   msg.PlatformUserID,
			Text:             result.FinalText,
			CreatedAt:        time.Now().UTC(),
		})
	}
	return nil
}

func (r *Runtime) sendCommandError(ctx context.Context, msg model.InboundMessage, commandErr error) error {
	var categorized commandError
	if err, ok := commandErr.(commandError); ok {
		categorized = err
	} else {
		categorized = commandError{Category: commandErrorFailure, Message: commandErr.Error()}
	}
	switch categorized.Category {
	case commandErrorUnknown:
		return r.sendCommandReply(ctx, msg, categorized.Message+"\nUse /help to see available commands.")
	case commandErrorPermissionDenied:
		return r.sendCommandReply(ctx, msg, categorized.Message)
	default:
		return r.sendCommandReply(ctx, msg, "Command failed: "+categorized.Message)
	}
}

func (r *Runtime) sendCommandReply(ctx context.Context, msg model.InboundMessage, text string) error {
	if r.cfg.Sender == nil {
		return nil
	}
	return r.cfg.Sender.Send(ctx, model.OutboundMessage{
		AssistantID:      msg.AssistantID,
		Platform:         msg.Platform,
		AccountID:        msg.AccountID,
		PrivateChannelID: msg.PrivateChannelID,
		PlatformUserID:   msg.PlatformUserID,
		Text:             text,
		CreatedAt:        time.Now().UTC(),
	})
}

func (r *Runtime) RecordPermissionRequest(ctx context.Context, req PermissionRequest) (model.PendingPermission, error) {
	session, err := r.cfg.Store.SessionByID(ctx, req.LocalSessionID)
	if err != nil {
		return model.PendingPermission{}, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	permission, err := r.cfg.Store.CreatePermission(ctx, model.PendingPermission{
		LocalSessionID:    req.LocalSessionID,
		Owner:             session.Binding,
		ACPRequestID:      req.ACPRequestID,
		Options:           req.Options,
		ExpiresAt:         time.Now().UTC().Add(timeout),
		TimeoutResolution: req.TimeoutResolution,
	})
	if err != nil {
		return model.PendingPermission{}, err
	}
	if r.cfg.Sender != nil {
		text := fmt.Sprintf("Permission requested. Reply approve %s to allow or reject %s to deny.", permission.ShortApprovalID, permission.ShortApprovalID)
		_ = r.cfg.Sender.Send(ctx, model.OutboundMessage{
			AssistantID:      session.Binding.AssistantID,
			Platform:         session.Binding.Platform,
			AccountID:        session.Binding.AccountID,
			PrivateChannelID: session.Binding.PrivateChannelID,
			PlatformUserID:   session.Binding.PlatformUserID,
			Text:             text,
			CreatedAt:        time.Now().UTC(),
			PermissionPrompt: &model.PermissionPrompt{
				ShortApprovalID: permission.ShortApprovalID,
				Options:         append([]string(nil), permission.Options...),
				Text:            text,
			},
		})
	}
	return permission, nil
}

func (r *Runtime) HandlePermissionDecision(ctx context.Context, decision model.PermissionDecision) error {
	shortID := strings.ToUpper(strings.TrimSpace(decision.ShortApprovalID))
	if shortID == "" {
		return fmt.Errorf("permission decision requires an approval id")
	}
	if decision.AssistantID == "" {
		decision.AssistantID = r.cfg.AssistantID
	}
	if decision.EventID != "" {
		duplicate, err := r.cfg.Store.RememberIdempotency(ctx, decision.AssistantID, decision.Platform, decision.AccountID, "permission_decision:"+decision.EventID)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
	}
	permission, err := r.cfg.Store.PermissionByShortID(ctx, shortID)
	if err != nil {
		return err
	}
	owner := model.SessionBindingKey{
		AssistantID:      decision.AssistantID,
		Platform:         decision.Platform,
		AccountID:        decision.AccountID,
		PrivateChannelID: decision.PrivateChannelID,
		PlatformUserID:   decision.PlatformUserID,
	}
	if permission.Owner != owner {
		return fmt.Errorf("permission %s belongs to a different owner", shortID)
	}
	if permission.Status != "pending" {
		return nil
	}
	option := permissionDecisionOption(decision.Option, permission.Options)
	if _, err := r.cfg.Store.ResolvePermission(ctx, shortID, owner, option); err != nil {
		return err
	}
	if resolver, ok := r.cfg.Harness.(PermissionResolver); ok {
		return resolver.ResolvePermission(ctx, shortID, option)
	}
	return nil
}

func (r *Runtime) ExpirePermissions(ctx context.Context, now time.Time) error {
	expired, err := r.cfg.Store.ExpirePermissions(ctx, now)
	if err != nil {
		return err
	}
	for _, permission := range expired {
		if resolver, ok := r.cfg.Harness.(PermissionResolver); ok {
			_ = resolver.ResolvePermission(ctx, permission.ShortApprovalID, permission.TimeoutResolution)
		}
		if r.cfg.Sender == nil {
			continue
		}
		_ = r.cfg.Sender.Send(ctx, model.OutboundMessage{
			AssistantID:      permission.Owner.AssistantID,
			Platform:         permission.Owner.Platform,
			AccountID:        permission.Owner.AccountID,
			PrivateChannelID: permission.Owner.PrivateChannelID,
			PlatformUserID:   permission.Owner.PlatformUserID,
			Text:             "Permission request " + permission.ShortApprovalID + " timed out.",
			CreatedAt:        time.Now().UTC(),
		})
	}
	return nil
}

func (r *Runtime) ensureActiveSession(ctx context.Context, key model.SessionBindingKey) (model.LocalSession, error) {
	session, err := r.cfg.Store.ActiveSessionForBinding(ctx, key)
	if err == nil {
		return session, nil
	}
	if err != sql.ErrNoRows {
		return model.LocalSession{}, err
	}
	mode, err := r.defaultMode(ctx, key)
	if err != nil {
		return model.LocalSession{}, err
	}
	return r.cfg.Store.CreateSession(ctx, key, mode, string(mode))
}

func (r *Runtime) handleCommand(ctx context.Context, msg model.InboundMessage) (commandResult, error) {
	fields := strings.Fields(strings.TrimSpace(msg.Text))
	if len(fields) == 0 {
		return commandResult{}, nil
	}
	command := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if !r.commandAllowed(msg.BindingKey(), command, fields[1:]) {
		return commandResult{}, commandError{Category: commandErrorPermissionDenied, Message: "Owner permission is required for " + fields[0] + "."}
	}
	switch command {
	case "help":
		return commandResult{Text: r.commandHelp(msg.BindingKey())}, nil
	case "status":
		return r.commandStatus(ctx, msg.BindingKey())
	case "skills":
		return r.commandSkills(fields[1:], msg.BindingKey())
	case "new":
		text, err := r.commandNew(ctx, msg.BindingKey())
		return commandResult{Text: text}, err
	case "clear":
		text, err := r.commandClear(ctx, msg.BindingKey())
		return commandResult{Text: text}, err
	case "session":
		text, err := r.commandSession(ctx, msg.BindingKey(), fields[1:])
		return commandResult{Text: text}, err
	case "mode":
		text, err := r.commandMode(ctx, msg.BindingKey(), fields[1:])
		return commandResult{Text: text}, err
	case "approve", "reject":
		if len(fields) != 2 {
			return commandResult{}, commandError{Category: commandErrorFailure, Message: command + " requires an approval id"}
		}
		shortID := strings.ToUpper(fields[1])
		permission, err := r.cfg.Store.PermissionByShortID(ctx, shortID)
		if err != nil {
			return commandResult{}, err
		}
		option := permissionCommandOption(command, permission.Options)
		if _, err := r.cfg.Store.ResolvePermission(ctx, shortID, msg.BindingKey(), option); err != nil {
			return commandResult{}, err
		}
		if resolver, ok := r.cfg.Harness.(PermissionResolver); ok {
			if err := resolver.ResolvePermission(ctx, shortID, option); err != nil {
				return commandResult{}, err
			}
		}
		return commandResult{Text: "Permission " + shortID + " " + permissionCommandPastTense(command) + "."}, nil
	case "memory":
		text, err := r.commandMemory(ctx, msg, fields)
		return commandResult{Text: text}, err
	default:
		return commandResult{}, commandError{Category: commandErrorUnknown, Message: "Unknown command " + fields[0] + "."}
	}
}

func permissionCommandPastTense(command string) string {
	if command == "reject" {
		return "rejected"
	}
	return "approved"
}

func isCommandText(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	if strings.HasPrefix(fields[0], "/") {
		return true
	}
	switch strings.ToLower(fields[0]) {
	case "new", "clear", "session", "mode", "approve", "reject", "memory", "help", "status", "skills":
		return true
	default:
		return false
	}
}

func permissionCommandOption(command string, options []string) string {
	if command == "reject" {
		return preferredPermissionOption(options, []string{"reject", "rejected", "deny", "denied", "abort"}, "reject")
	}
	return preferredPermissionOption(options, []string{"approve", "approved", "allow", "allowed", "accept", "accepted"}, "approve")
}

func permissionDecisionOption(raw string, options []string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "reject", "rejected", "deny", "denied", "abort":
		return permissionCommandOption("reject", options)
	default:
		return permissionCommandOption("approve", options)
	}
}

func preferredPermissionOption(options, preferred []string, fallback string) string {
	if len(options) == 0 {
		return fallback
	}
	for _, want := range preferred {
		for _, option := range options {
			if strings.EqualFold(option, want) {
				return option
			}
		}
	}
	if fallback == "reject" {
		return options[len(options)-1]
	}
	return options[0]
}

func (r *Runtime) commandAllowed(key model.SessionBindingKey, command string, args []string) bool {
	if command == "approve" || command == "reject" {
		return true
	}
	if command == "skills" && len(args) > 0 && strings.EqualFold(args[0], "verbose") {
		return r.isOwnerAdmin(key)
	}
	switch command {
	case "mode", "memory":
		return r.isOwnerAdmin(key)
	default:
		return true
	}
}

func (r *Runtime) isOwnerAdmin(key model.SessionBindingKey) bool {
	policy := r.effectivePolicy(key)
	if policy.Admin || policy.CanSetDefaultMode {
		return true
	}
	options := r.channelOptions(key)
	for _, option := range []string{"owner_open_id", "owner_user_id", "admin_open_id", "admin_user_id"} {
		if strings.TrimSpace(options[option]) != "" && options[option] == key.PlatformUserID {
			return true
		}
	}
	for _, option := range []string{"owner_open_ids", "owner_user_ids", "admin_open_ids", "admin_user_ids"} {
		for _, value := range strings.Split(options[option], ",") {
			if strings.TrimSpace(value) == key.PlatformUserID {
				return true
			}
		}
	}
	return false
}

func (r *Runtime) channelOptions(key model.SessionBindingKey) map[string]string {
	if r.cfg.ChannelOptions == nil {
		return nil
	}
	if options, ok := r.cfg.ChannelOptions[key.AccountPolicyKey()]; ok {
		return options
	}
	return nil
}

func (r *Runtime) commandHelp(key model.SessionBindingKey) string {
	owner := r.isOwnerAdmin(key)
	lines := []string{
		"Available commands:",
		"/help - show available commands",
		"/status - show current session status",
		"/session - list your sessions",
		"/session <id> - switch to one of your sessions",
		"/clear - start a fresh session",
		"/skills - list effective skills",
		"/approve <id> - approve a pending action",
		"/reject <id> - reject a pending action",
	}
	if owner {
		lines = append(lines,
			"/mode <manual|yolo|full_auto> - change current session permission mode",
			"/mode default <manual|yolo|full_auto> - change your default permission mode",
			"/skills verbose - show skill source layers and paths",
			"/memory set <target> <content> - update workspace memory",
			"/memory rollback <target> <revision> - roll back workspace memory",
		)
	}
	return strings.Join(lines, "\n")
}

func (r *Runtime) commandStatus(ctx context.Context, key model.SessionBindingKey) (commandResult, error) {
	session, err := r.ensureActiveSession(ctx, key)
	if err != nil {
		return commandResult{}, err
	}
	pending, err := r.cfg.Store.PendingPermissionCountForOwner(ctx, key)
	if err != nil {
		return commandResult{}, err
	}
	connector := "unknown"
	if status, err := r.cfg.Store.ConnectorStatus(ctx, key.AssistantID, key.Platform, key.AccountID); err == nil {
		connector = string(status.State)
		if strings.TrimSpace(status.Message) != "" {
			connector += " - " + status.Message
		}
		if strings.TrimSpace(status.LastError) != "" {
			connector += " - " + status.LastError
		}
	} else if err != sql.ErrNoRows {
		return commandResult{}, err
	}
	lines := []string{
		"Status:",
		"session: " + session.ID,
		"mode: " + string(session.PermissionMode),
		"harness: " + string(r.cfg.Provider),
		"connector: " + string(key.Platform) + "/" + key.AccountID + " " + connector,
		fmt.Sprintf("pending permissions: %d", pending),
	}
	return commandResult{Text: strings.Join(lines, "\n")}, nil
}

func (r *Runtime) commandSkills(args []string, key model.SessionBindingKey) (commandResult, error) {
	verbose := len(args) > 0 && strings.EqualFold(args[0], "verbose")
	if len(args) > 0 && !verbose {
		return commandResult{}, commandError{Category: commandErrorFailure, Message: "/skills supports only the optional verbose argument"}
	}
	skills, err := harnesspkg.ListSkills(r.cfg.ACPAHome, r.cfg.ConfigspacePath, r.cfg.Provider)
	if err != nil {
		return commandResult{}, err
	}
	if len(skills) == 0 {
		return commandResult{Text: "No ACPA-managed skills are configured."}, nil
	}
	if !verbose {
		lines := []string{"Effective skills:"}
		for _, skill := range skills {
			line := "- " + skill.Name
			if skill.Description != "" {
				line += ": " + skill.Description
			}
			lines = append(lines, line)
		}
		return commandResult{Text: strings.Join(lines, "\n")}, nil
	}
	_ = key
	return commandResult{Text: verboseSkillsText(skills)}, nil
}

func verboseSkillsText(skills []model.SkillInfo) string {
	byLayer := map[string][]model.SkillInfo{}
	var layers []string
	for _, skill := range skills {
		if _, ok := byLayer[skill.Layer]; !ok {
			layers = append(layers, skill.Layer)
		}
		byLayer[skill.Layer] = append(byLayer[skill.Layer], skill)
	}
	sort.Strings(layers)
	lines := []string{"Effective skills (verbose):"}
	for _, layer := range layers {
		lines = append(lines, layer+":")
		for _, skill := range byLayer[layer] {
			lines = append(lines, "- "+skill.Name)
			if skill.SourcePath != "" {
				lines = append(lines, "  source: "+skill.SourcePath)
			}
			if skill.OverlayPath != "" {
				lines = append(lines, "  overlay: "+skill.OverlayPath)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (r *Runtime) commandNew(ctx context.Context, key model.SessionBindingKey) (string, error) {
	mode, err := r.defaultMode(ctx, key)
	if err != nil {
		return "", err
	}
	session, err := r.cfg.Store.CreateSession(ctx, key, mode, string(mode))
	if err != nil {
		return "", err
	}
	if err := r.cfg.Store.SetActiveSession(ctx, key, session.ID); err != nil {
		return "", err
	}
	return "New session " + session.ID + " created with mode " + string(mode), nil
}

func (r *Runtime) commandClear(ctx context.Context, key model.SessionBindingKey) (string, error) {
	mode, err := r.defaultMode(ctx, key)
	if err != nil {
		return "", err
	}
	session, err := r.cfg.Store.CreateSession(ctx, key, mode, string(mode))
	if err != nil {
		return "", err
	}
	if err := r.cfg.Store.SetActiveSession(ctx, key, session.ID); err != nil {
		return "", err
	}
	return "Cleared context. New session " + session.ID + " is active with mode " + string(mode) + ".", nil
}

func (r *Runtime) commandSession(ctx context.Context, key model.SessionBindingKey, args []string) (string, error) {
	if len(args) == 0 {
		sessions, err := r.cfg.Store.ListSessionsForBinding(ctx, key)
		if err != nil {
			return "", err
		}
		if len(sessions) == 0 {
			return "No sessions yet. Send a message or /clear to start one.", nil
		}
		var lines []string
		lines = append(lines, "Sessions:")
		for _, session := range sessions {
			lines = append(lines, session.ID+" "+string(session.PermissionMode))
		}
		return strings.Join(lines, "\n"), nil
	}
	session, err := r.cfg.Store.SessionByID(ctx, args[0])
	if err != nil {
		return "", err
	}
	if session.Binding != key {
		return "", fmt.Errorf("session %s does not belong to sender", args[0])
	}
	if err := r.cfg.Store.SetActiveSession(ctx, key, session.ID); err != nil {
		return "", err
	}
	return "Active session switched to " + session.ID + " (" + string(session.PermissionMode) + ")", nil
}

func (r *Runtime) commandMode(ctx context.Context, key model.SessionBindingKey, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("/mode requires a mode")
	}
	if args[0] == "default" {
		if len(args) != 2 {
			return "", fmt.Errorf("/mode default requires a mode")
		}
		mode := model.PermissionMode(args[1])
		if err := r.validateMode(key, mode); err != nil {
			return "", err
		}
		if err := r.cfg.Store.SetBindingDefaultMode(ctx, key, mode); err != nil {
			return "", err
		}
		return "Default mode set to " + string(mode) + ". " + modeHint(mode), nil
	}
	mode := model.PermissionMode(args[0])
	if err := r.validateMode(key, mode); err != nil {
		return "", err
	}
	session, err := r.ensureActiveSession(ctx, key)
	if err != nil {
		return "", err
	}
	profile, err := harnesspkg.ResolveLaunchProfile(r.cfg.Provider, mode, harnesspkg.ProfileOptions{})
	if err != nil {
		return "", err
	}
	if r.cfg.Harness != nil {
		result, err := r.cfg.Harness.SwitchMode(ctx, SwitchModeRequest{LocalSessionID: session.ID, CurrentACPSession: session.ACPSessionID, ExternalSessionID: session.ExternalSessionID, Mode: mode})
		if err != nil {
			return "", err
		}
		if result.LaunchProfileKey != "" {
			profile.Key = result.LaunchProfileKey
		}
		if result.ACPSessionID != "" {
			if err := r.cfg.Store.UpdateSessionACP(ctx, session.ID, result.ACPSessionID, session.ExternalSessionID); err != nil {
				return "", err
			}
		}
	}
	if err := r.cfg.Store.UpdateSessionMode(ctx, session.ID, mode, profile.Key); err != nil {
		return "", err
	}
	return "Mode switched to " + string(mode) + ". " + modeHint(mode), nil
}

func modeHint(mode model.PermissionMode) string {
	switch mode {
	case model.PermissionManual:
		return "Privileged actions will request authorization."
	case model.PermissionYolo:
		return "Subsequent actions may skip authorization; use only for trusted tasks."
	case model.PermissionFullAuto:
		return "The assistant will execute supported actions automatically when the harness allows it."
	default:
		return ""
	}
}

func (r *Runtime) commandMemory(ctx context.Context, msg model.InboundMessage, fields []string) (string, error) {
	if r.cfg.Memory == nil {
		return "", fmt.Errorf("memory manager is not configured")
	}
	if len(fields) >= 4 && fields[1] == "set" {
		content := strings.TrimSpace(strings.TrimPrefix(msg.Text, strings.Join(fields[:3], " ")))
		_, err := r.cfg.Memory.Update(ctx, model.MemoryUpdate{Target: fields[2], Content: content, Origin: model.MemoryOriginUser, ActorID: msg.PlatformUserID})
		if err != nil {
			return "", err
		}
		return "Memory " + fields[2] + " updated", nil
	}
	if len(fields) == 4 && fields[1] == "rollback" {
		var revision int64
		if _, err := fmt.Sscanf(fields[3], "%d", &revision); err != nil {
			return "", err
		}
		if err := r.cfg.Memory.Rollback(ctx, fields[2], revision, msg.PlatformUserID); err != nil {
			return "", err
		}
		return "Memory " + fields[2] + " rolled back to revision " + fields[3], nil
	}
	return "", fmt.Errorf("unsupported memory command")
}

func (r *Runtime) defaultMode(ctx context.Context, key model.SessionBindingKey) (model.PermissionMode, error) {
	if mode, err := r.cfg.Store.BindingDefaultMode(ctx, key); err == nil && mode != "" {
		return mode, nil
	}
	policy := r.effectivePolicy(key)
	if policy.DefaultMode == "" {
		return model.PermissionManual, nil
	}
	return policy.DefaultMode, nil
}

func (r *Runtime) validateMode(key model.SessionBindingKey, mode model.PermissionMode) error {
	policy := r.effectivePolicy(key)
	if !modeAllowed(policy.AllowedModes, mode) {
		return fmt.Errorf("permission mode %s is not allowed", mode)
	}
	if !harnesspkg.SupportsMode(r.cfg.Provider, mode) {
		return fmt.Errorf("%s does not support permission mode %s", r.cfg.Provider, mode)
	}
	return nil
}

func (r *Runtime) effectivePolicy(key model.SessionBindingKey) model.Policy {
	policy := r.cfg.Policy.Assistant
	if policy.AllowedModes == nil {
		policy = model.DefaultPolicySet().Assistant
	}
	if account, ok := r.cfg.Policy.Accounts[key.AccountPolicyKey()]; ok {
		policy = mergePolicy(policy, account)
	}
	if user, ok := r.cfg.Policy.Users[key.UserPolicyKey()]; ok {
		policy = mergePolicy(policy, user)
	}
	return policy
}

func mergePolicy(base, override model.Policy) model.Policy {
	if len(override.AllowedModes) > 0 {
		base.AllowedModes = intersectModes(base.AllowedModes, override.AllowedModes)
	}
	if override.DefaultMode != "" {
		base.DefaultMode = override.DefaultMode
	}
	if override.CanSetDefaultMode {
		base.CanSetDefaultMode = true
	}
	if override.Admin {
		base.Admin = true
	}
	return base
}

func modeAllowed(allowed []model.PermissionMode, mode model.PermissionMode) bool {
	for _, candidate := range allowed {
		if candidate == mode {
			return true
		}
	}
	return false
}

func intersectModes(a, b []model.PermissionMode) []model.PermissionMode {
	set := map[model.PermissionMode]struct{}{}
	for _, mode := range b {
		set[mode] = struct{}{}
	}
	var out []model.PermissionMode
	for _, mode := range a {
		if _, ok := set[mode]; ok {
			out = append(out, mode)
		}
	}
	return out
}
