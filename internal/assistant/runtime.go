package assistant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	cronpkg "github.com/Foolyou/acp-assistant/internal/cron"
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

type cronContextKey struct{}

var harnessCronToolPattern = regexp.MustCompile("(?s)```acpa-cron\\s*(.*?)\\s*```")

type harnessCronToolCall struct {
	Action       string `json:"action"`
	JobID        string `json:"job_id,omitempty"`
	Name         string `json:"name,omitempty"`
	ScheduleType string `json:"schedule_type,omitempty"`
	ScheduleExpr string `json:"schedule_expr,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	Message      string `json:"message,omitempty"`
	Target       string `json:"target,omitempty"`
	Delivery     string `json:"delivery,omitempty"`
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
	if handled, err := r.handleHarnessCronTool(ctx, msg, result.FinalText); handled || err != nil {
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

func (r *Runtime) handleHarnessCronTool(ctx context.Context, msg model.InboundMessage, text string) (bool, error) {
	matches := harnessCronToolPattern.FindStringSubmatch(text)
	if matches == nil {
		return false, nil
	}
	if !r.isOwnerAdmin(msg.BindingKey()) {
		return true, r.sendCommandReply(ctx, msg, "Owner permission is required for cron tools.")
	}
	var call harnessCronToolCall
	if err := json.Unmarshal([]byte(strings.TrimSpace(matches[1])), &call); err != nil {
		return true, r.sendCommandReply(ctx, msg, "Command failed: invalid cron tool JSON: "+err.Error())
	}
	reply, err := r.executeHarnessCronTool(ctx, msg, call)
	if err != nil {
		return true, r.sendCommandReply(ctx, msg, "Command failed: "+err.Error())
	}
	if strings.TrimSpace(reply) == "" {
		return true, nil
	}
	return true, r.sendCommandReply(ctx, msg, reply)
}

func (r *Runtime) executeHarnessCronTool(ctx context.Context, msg model.InboundMessage, call harnessCronToolCall) (string, error) {
	switch normalizeHarnessCronAction(call.Action) {
	case "create":
		opts := cronCommandOptions{
			Name:         defaultString(call.Name, "cron job"),
			ScheduleType: model.CronScheduleType(strings.TrimSpace(call.ScheduleType)),
			ScheduleExpr: strings.TrimSpace(call.ScheduleExpr),
			Timezone:     defaultString(call.Timezone, "UTC"),
			Prompt:       strings.TrimSpace(call.Message),
			Target:       model.CronTarget(defaultString(call.Target, string(model.CronTargetIsolated))),
			DeliveryMode: model.CronDeliveryMode(defaultString(call.Delivery, string(model.CronDeliveryOrigin))),
		}
		if opts.ScheduleType != model.CronScheduleTypeAt && opts.ScheduleType != model.CronScheduleTypeEvery && opts.ScheduleType != model.CronScheduleTypeCron {
			return "", fmt.Errorf("unsupported schedule type %q", call.ScheduleType)
		}
		if opts.ScheduleExpr == "" {
			return "", fmt.Errorf("schedule_expr is required")
		}
		if opts.Prompt == "" {
			return "", fmt.Errorf("message is required")
		}
		if opts.Target != model.CronTargetIsolated && opts.Target != model.CronTargetMain {
			return "", fmt.Errorf("unsupported cron target %q", opts.Target)
		}
		if opts.DeliveryMode != model.CronDeliveryOrigin && opts.DeliveryMode != model.CronDeliveryNone {
			return "", fmt.Errorf("unsupported delivery mode %q", opts.DeliveryMode)
		}
		job, err := r.createCronJob(ctx, msg, opts)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Cron job created: %s %s next %s", job.ID, job.Name, formatCommandTime(job.NextRunAt)), nil
	case "delete":
		jobID := strings.TrimSpace(call.JobID)
		if jobID == "" {
			return "", fmt.Errorf("job_id is required")
		}
		if err := r.cfg.Store.RemoveCronJob(ctx, msg.AssistantID, jobID); err != nil {
			return "", err
		}
		return "Cron job removed: " + jobID, nil
	case "list":
		return r.commandCronList(ctx, msg.BindingKey())
	default:
		return "", fmt.Errorf("unsupported cron tool action %q", call.Action)
	}
}

func normalizeHarnessCronAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	action = strings.TrimPrefix(action, "cron")
	if action == "remove" {
		return "delete"
	}
	return action
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func (r *Runtime) ExecuteCronRun(ctx context.Context, run model.CronRun) error {
	if _, ok := ctx.Value(cronContextKey{}).(bool); ok {
		return fmt.Errorf("nested cron execution is not allowed")
	}
	ctx = context.WithValue(ctx, cronContextKey{}, true)
	job := run.Job
	if job.ID == "" {
		loaded, err := r.cfg.Store.CronJob(ctx, run.AssistantID, run.JobID)
		if err != nil {
			return err
		}
		job = loaded
	}
	status := model.CronRunStatusSucceeded
	finalText := ""
	errorText := ""
	localSessionID := ""
	acpSessionID := ""
	externalSessionID := ""
	var nextRunAt *time.Time
	if run.Manual {
		if job.Enabled && !job.NextRunAt.IsZero() {
			next := job.NextRunAt
			nextRunAt = &next
		}
	} else if job.ScheduleType != model.CronScheduleTypeAt {
		next, err := cronpkg.NextRun(job.ScheduleType, job.ScheduleExpr, job.Timezone, time.Now().UTC(), run.DueAt)
		if err != nil {
			status = model.CronRunStatusFailed
			errorText = err.Error()
		} else {
			nextRunAt = &next
		}
	}

	if errorText == "" {
		session, err := r.ensureCronSession(ctx, job)
		if err != nil {
			status = model.CronRunStatusFailed
			errorText = err.Error()
		} else if r.cfg.Harness == nil {
			status = model.CronRunStatusFailed
			errorText = "harness is not configured"
			localSessionID = session.ID
		} else {
			localSessionID = session.ID
			ensured, err := r.cfg.Harness.EnsureSession(ctx, EnsureSessionRequest{
				LocalSessionID:    session.ID,
				Binding:           session.Binding,
				PermissionMode:    session.PermissionMode,
				LaunchProfileKey:  session.LaunchProfileKey,
				CurrentACPSession: session.ACPSessionID,
				ExternalSessionID: session.ExternalSessionID,
			})
			if err != nil {
				status = model.CronRunStatusFailed
				errorText = err.Error()
			} else {
				if ensured.ACPSessionID != "" || ensured.ExternalSessionID != "" {
					if err := r.cfg.Store.UpdateSessionACP(ctx, session.ID, ensured.ACPSessionID, ensured.ExternalSessionID); err != nil {
						status = model.CronRunStatusFailed
						errorText = err.Error()
					}
					session.ACPSessionID = ensured.ACPSessionID
					session.ExternalSessionID = ensured.ExternalSessionID
				}
				acpSessionID = session.ACPSessionID
				externalSessionID = session.ExternalSessionID
			}
			if errorText == "" {
				result, err := r.cfg.Harness.Prompt(ctx, PromptRequest{LocalSessionID: session.ID, ACPSessionID: session.ACPSessionID, Text: job.Prompt})
				if err != nil {
					status = model.CronRunStatusFailed
					errorText = err.Error()
				} else {
					finalText = result.FinalText
				}
			}
		}
	}
	completed, err := r.cfg.Store.CompleteCronRun(ctx, run.ID, status, finalText, errorText, localSessionID, acpSessionID, externalSessionID, nextRunAt)
	if err != nil {
		return err
	}
	completed.Job = job
	return r.deliverCronRun(ctx, completed)
}

func (r *Runtime) RunDueCronJobs(ctx context.Context, now time.Time, limit int) error {
	claims, err := r.cfg.Store.ClaimDueCronRuns(ctx, r.cfg.AssistantID, now.UTC(), limit)
	if err != nil {
		return err
	}
	for _, claim := range claims {
		if err := r.ExecuteCronRun(ctx, claim); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) StartCronScheduler(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		interval = time.Minute
	}
	runCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case now := <-ticker.C:
				_ = r.RunDueCronJobs(runCtx, now.UTC(), 10)
			}
		}
	}()
	return cancel
}

func (r *Runtime) ensureCronSession(ctx context.Context, job model.CronJob) (model.LocalSession, error) {
	key := job.Creator
	key.AssistantID = job.AssistantID
	if job.Target == model.CronTargetIsolated {
		key.ConversationKey = "cron:" + job.ID
		key.ThreadKey = ""
	}
	session, err := r.cfg.Store.ActiveSessionForBinding(ctx, key)
	if err == nil {
		return session, nil
	}
	if err != sql.ErrNoRows {
		return model.LocalSession{}, err
	}
	mode := job.PermissionMode
	if mode == "" {
		mode = model.PermissionManual
	}
	return r.cfg.Store.CreateSession(ctx, key, mode, string(mode))
}

func (r *Runtime) deliverCronRun(ctx context.Context, run model.CronRun) error {
	if r.cfg.Sender == nil || run.Job.DeliveryMode == model.CronDeliveryNone {
		return nil
	}
	text := strings.TrimSpace(run.FinalText)
	if run.Status != model.CronRunStatusSucceeded {
		text = "Cron job " + run.Job.Name + " failed: " + run.Error
	} else if text == "" || strings.HasPrefix(text, "[SILENT]") {
		return nil
	}
	return r.cfg.Sender.Send(ctx, model.OutboundMessage{
		AssistantID:      run.Job.AssistantID,
		Platform:         run.Job.Creator.Platform,
		AccountID:        run.Job.Creator.AccountID,
		PrivateChannelID: run.Job.Creator.PrivateChannelID,
		PlatformUserID:   run.Job.Creator.PlatformUserID,
		Text:             text,
		CreatedAt:        time.Now().UTC(),
	})
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
	case "cron":
		text, err := r.commandCron(ctx, msg, fields[1:])
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
	case "new", "clear", "session", "mode", "approve", "reject", "memory", "help", "status", "skills", "cron":
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
	case "mode", "memory", "cron":
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
			"/cron add|list|run|pause|resume|remove|runs - manage scheduled assistant work",
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

func (r *Runtime) commandCron(ctx context.Context, msg model.InboundMessage, args []string) (string, error) {
	if _, ok := ctx.Value(cronContextKey{}).(bool); ok {
		return "", commandError{Category: commandErrorPermissionDenied, Message: "Cron jobs cannot manage cron jobs."}
	}
	if len(args) == 0 {
		return r.commandCronList(ctx, msg.BindingKey())
	}
	switch strings.ToLower(args[0]) {
	case "add":
		return r.commandCronAdd(ctx, msg, args[1:])
	case "list":
		return r.commandCronList(ctx, msg.BindingKey())
	case "pause":
		return r.commandCronSetEnabled(ctx, msg.BindingKey(), args[1:], false)
	case "resume":
		return r.commandCronSetEnabled(ctx, msg.BindingKey(), args[1:], true)
	case "remove":
		if len(args) != 2 {
			return "", commandError{Category: commandErrorFailure, Message: "/cron remove requires a job id"}
		}
		if err := r.cfg.Store.RemoveCronJob(ctx, msg.AssistantID, args[1]); err != nil {
			return "", err
		}
		return "Cron job removed: " + args[1], nil
	case "run":
		if len(args) != 2 {
			return "", commandError{Category: commandErrorFailure, Message: "/cron run requires a job id"}
		}
		run, err := r.cfg.Store.CreateManualCronRun(ctx, msg.AssistantID, args[1], time.Now().UTC())
		if err != nil {
			return "", err
		}
		if err := r.ExecuteCronRun(ctx, run); err != nil {
			return "", err
		}
		return "Cron run completed: " + run.ID, nil
	case "runs":
		if len(args) != 2 {
			return "", commandError{Category: commandErrorFailure, Message: "/cron runs requires a job id"}
		}
		return r.commandCronRuns(ctx, msg.AssistantID, args[1])
	default:
		return "", commandError{Category: commandErrorFailure, Message: "unknown /cron action " + args[0]}
	}
}

func (r *Runtime) commandCronAdd(ctx context.Context, msg model.InboundMessage, args []string) (string, error) {
	opts, err := parseCronOptions(args)
	if err != nil {
		return "", commandError{Category: commandErrorFailure, Message: err.Error()}
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return "", commandError{Category: commandErrorFailure, Message: "/cron add requires --message"}
	}
	job, err := r.createCronJob(ctx, msg, opts)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Cron job created: %s %s next %s", job.ID, job.Name, formatCommandTime(job.NextRunAt)), nil
}

func (r *Runtime) createCronJob(ctx context.Context, msg model.InboundMessage, opts cronCommandOptions) (model.CronJob, error) {
	now := time.Now().UTC()
	next, err := cronpkg.NextRun(opts.ScheduleType, opts.ScheduleExpr, opts.Timezone, now, now)
	if err != nil {
		return model.CronJob{}, err
	}
	policy := r.effectivePolicy(msg.BindingKey())
	mode := policy.DefaultMode
	if mode == "" {
		mode = model.PermissionManual
	}
	job, err := r.cfg.Store.CreateCronJob(ctx, model.CronJob{
		AssistantID:    msg.AssistantID,
		Name:           opts.Name,
		Enabled:        true,
		ScheduleType:   opts.ScheduleType,
		ScheduleExpr:   opts.ScheduleExpr,
		Timezone:       opts.Timezone,
		Prompt:         opts.Prompt,
		Target:         opts.Target,
		DeliveryMode:   opts.DeliveryMode,
		Creator:        msg.BindingKey(),
		PermissionMode: mode,
		NextRunAt:      next,
	})
	if err != nil {
		return model.CronJob{}, err
	}
	return job, nil
}

func (r *Runtime) commandCronList(ctx context.Context, key model.SessionBindingKey) (string, error) {
	jobs, err := r.cfg.Store.ListCronJobs(ctx, key.AssistantID)
	if err != nil {
		return "", err
	}
	if len(jobs) == 0 {
		return "No cron jobs.", nil
	}
	lines := []string{"Cron jobs:"}
	for _, job := range jobs {
		state := "paused"
		if job.Enabled {
			state = "enabled"
		}
		lines = append(lines, fmt.Sprintf("%s %s [%s] %s %s next %s", job.ID, job.Name, state, job.ScheduleType, job.ScheduleExpr, formatCommandTime(job.NextRunAt)))
	}
	return strings.Join(lines, "\n"), nil
}

func (r *Runtime) commandCronSetEnabled(ctx context.Context, key model.SessionBindingKey, args []string, enabled bool) (string, error) {
	if len(args) != 1 {
		action := "pause"
		if enabled {
			action = "resume"
		}
		return "", commandError{Category: commandErrorFailure, Message: "/cron " + action + " requires a job id"}
	}
	job, err := r.cfg.Store.CronJob(ctx, key.AssistantID, args[0])
	if err != nil {
		return "", err
	}
	var next time.Time
	if enabled {
		next, err = cronpkg.NextRun(job.ScheduleType, job.ScheduleExpr, job.Timezone, time.Now().UTC(), time.Now().UTC())
		if err != nil {
			return "", err
		}
	}
	job, err = r.cfg.Store.SetCronJobEnabled(ctx, key.AssistantID, args[0], enabled, next)
	if err != nil {
		return "", err
	}
	if enabled {
		return "Cron job resumed: " + job.ID + " next " + formatCommandTime(job.NextRunAt), nil
	}
	return "Cron job paused: " + job.ID, nil
}

func (r *Runtime) commandCronRuns(ctx context.Context, assistantID, jobID string) (string, error) {
	runs, err := r.cfg.Store.RecentCronRuns(ctx, assistantID, jobID, 5)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "No cron runs for " + jobID + ".", nil
	}
	lines := []string{"Cron runs:"}
	for _, run := range runs {
		line := fmt.Sprintf("%s %s started %s", run.ID, run.Status, formatCommandTime(run.StartedAt))
		if strings.TrimSpace(run.Error) != "" {
			line += " error " + run.Error
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

type cronCommandOptions struct {
	Name         string
	ScheduleType model.CronScheduleType
	ScheduleExpr string
	Timezone     string
	Prompt       string
	Target       model.CronTarget
	DeliveryMode model.CronDeliveryMode
}

func parseCronOptions(args []string) (cronCommandOptions, error) {
	opts := cronCommandOptions{Name: "cron job", Timezone: "UTC", Target: model.CronTargetIsolated, DeliveryMode: model.CronDeliveryOrigin}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--name requires a value")
			}
			opts.Name = args[i]
		case "--every":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--every requires a value")
			}
			if opts.ScheduleType != "" {
				return opts, fmt.Errorf("only one schedule flag is allowed")
			}
			opts.ScheduleType = model.CronScheduleTypeEvery
			opts.ScheduleExpr = args[i]
		case "--at":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--at requires a value")
			}
			if opts.ScheduleType != "" {
				return opts, fmt.Errorf("only one schedule flag is allowed")
			}
			opts.ScheduleType = model.CronScheduleTypeAt
			opts.ScheduleExpr = args[i]
		case "--cron":
			i++
			if i+4 >= len(args) {
				return opts, fmt.Errorf("--cron requires five fields")
			}
			if opts.ScheduleType != "" {
				return opts, fmt.Errorf("only one schedule flag is allowed")
			}
			opts.ScheduleType = model.CronScheduleTypeCron
			opts.ScheduleExpr = strings.Join(args[i:i+5], " ")
			i += 4
		case "--tz":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--tz requires a value")
			}
			opts.Timezone = args[i]
		case "--target":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--target requires a value")
			}
			opts.Target = model.CronTarget(args[i])
			if opts.Target != model.CronTargetIsolated && opts.Target != model.CronTargetMain {
				return opts, fmt.Errorf("unsupported cron target %q", args[i])
			}
		case "--deliver":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--deliver requires a value")
			}
			opts.DeliveryMode = model.CronDeliveryMode(args[i])
			if opts.DeliveryMode != model.CronDeliveryOrigin && opts.DeliveryMode != model.CronDeliveryNone {
				return opts, fmt.Errorf("unsupported delivery mode %q", args[i])
			}
		case "--message":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("--message requires a value")
			}
			opts.Prompt = strings.Join(args[i:], " ")
			i = len(args)
		default:
			return opts, fmt.Errorf("unknown cron option %s", args[i])
		}
	}
	if opts.ScheduleType == "" {
		return opts, fmt.Errorf("/cron add requires --at, --every, or --cron")
	}
	return opts, nil
}

func formatCommandTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.UTC().Format(time.RFC3339)
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
