package assistant

import (
	"context"
	"database/sql"
	"fmt"
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
	AssistantID string
	Provider    model.HarnessProvider
	Store       *store.Store
	Harness     Harness
	Sender      Sender
	Policy      model.PolicySet
	Memory      *workspace.MemoryManager
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

func NewRuntime(cfg RuntimeConfig) *Runtime {
	if cfg.Policy.Assistant.AllowedModes == nil {
		cfg.Policy = model.DefaultPolicySet()
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
		return r.handleCommand(ctx, msg)
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

func (r *Runtime) handleCommand(ctx context.Context, msg model.InboundMessage) error {
	fields := strings.Fields(strings.TrimSpace(msg.Text))
	if len(fields) == 0 {
		return nil
	}
	command := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	switch command {
	case "new":
		return r.commandNew(ctx, msg.BindingKey())
	case "session":
		return r.commandSession(ctx, msg.BindingKey(), fields[1:])
	case "mode":
		return r.commandMode(ctx, msg.BindingKey(), fields[1:])
	case "approve", "reject":
		if len(fields) != 2 {
			return fmt.Errorf("%s requires an approval id", command)
		}
		shortID := strings.ToUpper(fields[1])
		permission, err := r.cfg.Store.PermissionByShortID(ctx, shortID)
		if err != nil {
			return err
		}
		option := permissionCommandOption(command, permission.Options)
		if _, err := r.cfg.Store.ResolvePermission(ctx, shortID, msg.BindingKey(), option); err != nil {
			return err
		}
		if resolver, ok := r.cfg.Harness.(PermissionResolver); ok {
			return resolver.ResolvePermission(ctx, shortID, option)
		}
		return nil
	case "memory":
		return r.commandMemory(ctx, msg, fields)
	default:
		return fmt.Errorf("unknown command %s", fields[0])
	}
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
	case "new", "session", "mode", "approve", "reject", "memory":
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

func (r *Runtime) commandNew(ctx context.Context, key model.SessionBindingKey) error {
	mode, err := r.defaultMode(ctx, key)
	if err != nil {
		return err
	}
	session, err := r.cfg.Store.CreateSession(ctx, key, mode, string(mode))
	if err != nil {
		return err
	}
	return r.cfg.Store.SetActiveSession(ctx, key, session.ID)
}

func (r *Runtime) commandSession(ctx context.Context, key model.SessionBindingKey, args []string) error {
	if len(args) == 0 {
		sessions, err := r.cfg.Store.ListSessionsForBinding(ctx, key)
		if err != nil {
			return err
		}
		if r.cfg.Sender != nil {
			var lines []string
			for _, session := range sessions {
				lines = append(lines, session.ID+" "+string(session.PermissionMode))
			}
			_ = r.cfg.Sender.Send(ctx, model.OutboundMessage{AssistantID: key.AssistantID, Platform: key.Platform, AccountID: key.AccountID, PrivateChannelID: key.PrivateChannelID, PlatformUserID: key.PlatformUserID, Text: strings.Join(lines, "\n"), CreatedAt: time.Now().UTC()})
		}
		return nil
	}
	session, err := r.cfg.Store.SessionByID(ctx, args[0])
	if err != nil {
		return err
	}
	if session.Binding != key {
		return fmt.Errorf("session %s does not belong to sender", args[0])
	}
	return r.cfg.Store.SetActiveSession(ctx, key, session.ID)
}

func (r *Runtime) commandMode(ctx context.Context, key model.SessionBindingKey, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("/mode requires a mode")
	}
	if args[0] == "default" {
		if len(args) != 2 {
			return fmt.Errorf("/mode default requires a mode")
		}
		mode := model.PermissionMode(args[1])
		policy := r.effectivePolicy(key)
		if !policy.CanSetDefaultMode {
			return fmt.Errorf("sender cannot set default mode")
		}
		if err := r.validateMode(key, mode); err != nil {
			return err
		}
		return r.cfg.Store.SetBindingDefaultMode(ctx, key, mode)
	}
	mode := model.PermissionMode(args[0])
	if err := r.validateMode(key, mode); err != nil {
		return err
	}
	session, err := r.ensureActiveSession(ctx, key)
	if err != nil {
		return err
	}
	profile, err := harnesspkg.ResolveLaunchProfile(r.cfg.Provider, mode, harnesspkg.ProfileOptions{})
	if err != nil {
		return err
	}
	if r.cfg.Harness != nil {
		result, err := r.cfg.Harness.SwitchMode(ctx, SwitchModeRequest{LocalSessionID: session.ID, CurrentACPSession: session.ACPSessionID, ExternalSessionID: session.ExternalSessionID, Mode: mode})
		if err != nil {
			return err
		}
		if result.LaunchProfileKey != "" {
			profile.Key = result.LaunchProfileKey
		}
		if result.ACPSessionID != "" {
			if err := r.cfg.Store.UpdateSessionACP(ctx, session.ID, result.ACPSessionID, session.ExternalSessionID); err != nil {
				return err
			}
		}
	}
	return r.cfg.Store.UpdateSessionMode(ctx, session.ID, mode, profile.Key)
}

func (r *Runtime) commandMemory(ctx context.Context, msg model.InboundMessage, fields []string) error {
	if r.cfg.Memory == nil {
		return fmt.Errorf("memory manager is not configured")
	}
	if len(fields) >= 4 && fields[1] == "set" {
		content := strings.TrimSpace(strings.TrimPrefix(msg.Text, strings.Join(fields[:3], " ")))
		_, err := r.cfg.Memory.Update(ctx, model.MemoryUpdate{Target: fields[2], Content: content, Origin: model.MemoryOriginUser, ActorID: msg.PlatformUserID})
		return err
	}
	if len(fields) == 4 && fields[1] == "rollback" {
		var revision int64
		if _, err := fmt.Sscanf(fields[3], "%d", &revision); err != nil {
			return err
		}
		return r.cfg.Memory.Rollback(ctx, fields[2], revision, msg.PlatformUserID)
	}
	return fmt.Errorf("unsupported memory command")
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
