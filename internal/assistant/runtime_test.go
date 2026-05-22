package assistant_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/assistant"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

type fakeHarness struct {
	prompts             []assistant.PromptRequest
	resolvedPermissions []string
	finalText           string
	ensureErr           error
	promptErr           error
}

func (f *fakeHarness) EnsureSession(ctx context.Context, req assistant.EnsureSessionRequest) (assistant.EnsureSessionResult, error) {
	if f.ensureErr != nil {
		return assistant.EnsureSessionResult{}, f.ensureErr
	}
	return assistant.EnsureSessionResult{ACPSessionID: "acp-" + req.LocalSessionID, ExternalSessionID: "external-" + req.LocalSessionID}, nil
}

func (f *fakeHarness) Prompt(ctx context.Context, req assistant.PromptRequest) (assistant.PromptResult, error) {
	f.prompts = append(f.prompts, req)
	if f.promptErr != nil {
		return assistant.PromptResult{}, f.promptErr
	}
	if f.finalText != "" {
		return assistant.PromptResult{FinalText: f.finalText}, nil
	}
	return assistant.PromptResult{FinalText: "reply: " + req.Text}, nil
}

func (f *fakeHarness) SwitchMode(ctx context.Context, req assistant.SwitchModeRequest) (assistant.SwitchModeResult, error) {
	return assistant.SwitchModeResult{ACPSessionID: "switched-" + req.LocalSessionID, LaunchProfileKey: string(req.Mode)}, nil
}

func (f *fakeHarness) ResolvePermission(ctx context.Context, shortID, option string) error {
	f.resolvedPermissions = append(f.resolvedPermissions, shortID+":"+option)
	return nil
}

type fakeSender struct {
	messages []model.OutboundMessage
}

func (s *fakeSender) Send(ctx context.Context, msg model.OutboundMessage) error {
	s.messages = append(s.messages, msg)
	return nil
}

func TestRuntimeRoutesPrivateMessagesCommandsAndOwnerPermissions(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual, model.PermissionFullAuto, model.PermissionYolo}, DefaultMode: model.PermissionManual, CanSetDefaultMode: true},
		},
	})

	inbound := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "hello",
	}
	if err := rt.HandleInbound(ctx, inbound); err != nil {
		t.Fatalf("handle inbound: %v", err)
	}
	if len(h.prompts) != 1 || h.prompts[0].Text != "hello" {
		t.Fatalf("prompt was not dispatched: %#v", h.prompts)
	}
	if err := rt.HandleInbound(ctx, inbound); err != nil {
		t.Fatalf("duplicate inbound should not error: %v", err)
	}
	if len(h.prompts) != 1 {
		t.Fatalf("duplicate inbound dispatched another prompt: %#v", h.prompts)
	}

	modeMsg := inbound
	modeMsg.MessageID = "m2"
	modeMsg.Text = "/mode full_auto"
	if err := rt.HandleInbound(ctx, modeMsg); err != nil {
		t.Fatalf("mode command: %v", err)
	}
	active, err := db.ActiveSessionForBinding(ctx, inbound.BindingKey())
	if err != nil {
		t.Fatal(err)
	}
	if active.PermissionMode != model.PermissionFullAuto {
		t.Fatalf("mode was not switched: %#v", active)
	}

	perm, err := rt.RecordPermissionRequest(ctx, assistant.PermissionRequest{
		LocalSessionID:    active.ID,
		ACPRequestID:      "req-1",
		Options:           []string{"approve", "reject"},
		TimeoutResolution: "reject",
	})
	if err != nil {
		t.Fatalf("record permission: %v", err)
	}
	if len(s.messages) == 0 || strings.Contains(s.messages[len(s.messages)-1].Text, "/approve") || !strings.Contains(s.messages[len(s.messages)-1].Text, "approve "+perm.ShortApprovalID) {
		t.Fatalf("permission prompt should use non-slash approval commands, got %#v", s.messages)
	}
	if s.messages[len(s.messages)-1].PermissionPrompt == nil || s.messages[len(s.messages)-1].PermissionPrompt.ShortApprovalID != perm.ShortApprovalID {
		t.Fatalf("permission prompt should include structured payload, got %#v", s.messages[len(s.messages)-1])
	}
	attacker := inbound
	attacker.MessageID = "m3"
	attacker.PlatformUserID = "user-b"
	attacker.Text = "/approve " + perm.ShortApprovalID
	messageCount := len(s.messages)
	if err := rt.HandleInbound(ctx, attacker); err != nil {
		t.Fatalf("non-owner approval command error should be reported to sender: %v", err)
	}
	if len(s.messages) != messageCount+1 || !strings.Contains(s.messages[len(s.messages)-1].Text, "belongs to a different owner") {
		t.Fatalf("non-owner approval should be rejected in chat, got %#v", s.messages[messageCount:])
	}
	owner := inbound
	owner.MessageID = "m4"
	owner.Text = "/approve " + perm.ShortApprovalID
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("owner approval: %v", err)
	}
	resolved, err := db.PermissionByShortID(ctx, perm.ShortApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ResolvedOption != "approve" {
		t.Fatalf("permission was not approved: %#v", resolved)
	}
	if len(h.resolvedPermissions) != 1 || h.resolvedPermissions[0] != perm.ShortApprovalID+":approve" {
		t.Fatalf("permission was not forwarded to harness: %#v", h.resolvedPermissions)
	}
}

func TestRuntimeAcceptsBareApprovalAndMapsACPOptions(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	inbound := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "hello",
	}
	if err := rt.HandleInbound(ctx, inbound); err != nil {
		t.Fatalf("handle inbound: %v", err)
	}
	active, err := db.ActiveSessionForBinding(ctx, inbound.BindingKey())
	if err != nil {
		t.Fatal(err)
	}
	perm, err := rt.RecordPermissionRequest(ctx, assistant.PermissionRequest{
		LocalSessionID:    active.ID,
		ACPRequestID:      "req-1",
		Options:           []string{"approved", "approved-execpolicy-amendment", "abort"},
		TimeoutResolution: "abort",
	})
	if err != nil {
		t.Fatalf("record permission: %v", err)
	}

	approval := inbound
	approval.MessageID = "m2"
	approval.Text = "approve " + perm.ShortApprovalID
	if err := rt.HandleInbound(ctx, approval); err != nil {
		t.Fatalf("bare approve command: %v", err)
	}
	resolved, err := db.PermissionByShortID(ctx, perm.ShortApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ResolvedOption != "approved" {
		t.Fatalf("approval should map to ACP option, got %#v", resolved)
	}
	if len(h.resolvedPermissions) != 1 || h.resolvedPermissions[0] != perm.ShortApprovalID+":approved" {
		t.Fatalf("permission was not forwarded with ACP option: %#v", h.resolvedPermissions)
	}
}

func TestRuntimeReportsCommandOutcomesToSender(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	msg := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "/mode yolo",
	}
	if err := rt.HandleInbound(ctx, msg); err != nil {
		t.Fatalf("command errors should be reported to sender, got returned error: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "Owner permission is required") {
		t.Fatalf("expected permission denied message, got %#v", s.messages)
	}

	unknown := msg
	unknown.MessageID = "m2"
	unknown.Text = "/wat"
	if err := rt.HandleInbound(ctx, unknown); err != nil {
		t.Fatalf("unknown command should be reported to sender: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "Unknown command /wat") || !strings.Contains(s.messages[1].Text, "/help") {
		t.Fatalf("expected unknown command help hint, got %#v", s.messages)
	}

	admin := msg
	admin.MessageID = "m3"
	admin.PlatformUserID = "admin-a"
	admin.Text = "/mode yolo"
	adminRT := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
			Users: map[string]model.Policy{
				admin.BindingKey().UserPolicyKey(): {Admin: true},
			},
		},
	})
	if err := adminRT.HandleInbound(ctx, admin); err != nil {
		t.Fatalf("failing command should be reported to sender: %v", err)
	}
	if len(s.messages) != 3 || !strings.Contains(s.messages[2].Text, "Command failed: permission mode yolo is not allowed") {
		t.Fatalf("expected command failure message, got %#v", s.messages)
	}
}

func TestRuntimeReportsModeSwitchSuccessToSender(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual, model.PermissionFullAuto, model.PermissionYolo}, DefaultMode: model.PermissionManual, CanSetDefaultMode: true},
		},
	})
	msg := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "/mode yolo",
	}
	if err := rt.HandleInbound(ctx, msg); err != nil {
		t.Fatalf("mode switch: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "Mode switched to yolo") || !strings.Contains(s.messages[0].Text, "skip authorization") {
		t.Fatalf("expected mode switch confirmation, got %#v", s.messages)
	}

	defaultMsg := msg
	defaultMsg.MessageID = "m2"
	defaultMsg.Text = "/mode default full_auto"
	if err := rt.HandleInbound(ctx, defaultMsg); err != nil {
		t.Fatalf("default mode switch: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "Default mode set to full_auto") || !strings.Contains(s.messages[1].Text, "automatically") {
		t.Fatalf("expected default mode confirmation, got %#v", s.messages)
	}
}

func TestRuntimeHelpStatusAndClearCommands(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
		ChannelOptions: map[string]map[string]string{
			"feishu/main": {"owner_open_id": "owner-a"},
		},
	})
	if err := db.UpsertConnectorStatus(ctx, model.ConnectorStatus{AssistantID: "alpha", Platform: model.PlatformFeishu, AccountID: "main", State: model.ConnectorStateConnected, Message: "ready"}); err != nil {
		t.Fatalf("connector status: %v", err)
	}
	user := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "/help",
	}
	if err := rt.HandleInbound(ctx, user); err != nil {
		t.Fatalf("help: %v", err)
	}
	if len(s.messages) != 1 || strings.Contains(s.messages[0].Text, "/mode ") || !strings.Contains(s.messages[0].Text, "/status") {
		t.Fatalf("ordinary help should be filtered, got %#v", s.messages)
	}
	owner := user
	owner.PlatformUserID = "owner-a"
	owner.MessageID = "m2"
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("owner help: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "/mode ") || !strings.Contains(s.messages[1].Text, "/skills verbose") {
		t.Fatalf("owner help should include owner commands, got %#v", s.messages)
	}
	status := user
	status.MessageID = "m3"
	status.Text = "/status"
	if err := rt.HandleInbound(ctx, status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(s.messages) != 3 || !strings.Contains(s.messages[2].Text, "session:") || !strings.Contains(s.messages[2].Text, "harness: codex") || !strings.Contains(s.messages[2].Text, "connector: feishu/main connected") {
		t.Fatalf("status output missing expected fields, got %#v", s.messages)
	}
	clear := user
	clear.MessageID = "m4"
	clear.Text = "/clear"
	if err := rt.HandleInbound(ctx, clear); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(s.messages) != 4 || !strings.Contains(s.messages[3].Text, "Cleared context. New session") {
		t.Fatalf("clear should send explicit success, got %#v", s.messages)
	}
}

func TestRuntimeCronCommandsRequireOwnerAndCreateJobs(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
		ChannelOptions: map[string]map[string]string{
			"feishu/main": {"owner_open_id": "owner-a"},
		},
	})
	user := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "/cron list",
	}
	if err := rt.HandleInbound(ctx, user); err != nil {
		t.Fatalf("cron list denied should be reported to sender: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "Owner permission is required") {
		t.Fatalf("cron list should require owner/admin, got %#v", s.messages)
	}
	owner := user
	owner.PlatformUserID = "owner-a"
	owner.MessageID = "m2"
	owner.Text = "/cron add --every 1h --name hourly --message summarize workspace"
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("cron add: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "Cron job created") || !strings.Contains(s.messages[1].Text, "hourly") {
		t.Fatalf("cron add should confirm creation, got %#v", s.messages)
	}
	jobs, err := db.ListCronJobs(ctx, "alpha")
	if err != nil {
		t.Fatalf("list cron jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Prompt != "summarize workspace" || jobs[0].Creator.PlatformUserID != "owner-a" {
		t.Fatalf("unexpected cron job: %#v", jobs)
	}
	owner.MessageID = "m3"
	owner.Text = "/cron list"
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("cron list: %v", err)
	}
	if len(s.messages) != 3 || !strings.Contains(s.messages[2].Text, jobs[0].ID) || !strings.Contains(s.messages[2].Text, "hourly") {
		t.Fatalf("cron list should show created job, got %#v", s.messages)
	}
}

func TestRuntimeExecutesHarnessCronToolCreate(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{finalText: "```acpa-cron\n{\"action\":\"create\",\"name\":\"sleep reminder\",\"schedule_type\":\"at\",\"schedule_expr\":\"2099-05-23T01:10:00+08:00\",\"timezone\":\"Asia/Shanghai\",\"message\":\"提醒我睡觉\",\"target\":\"isolated\",\"delivery\":\"origin\"}\n```"}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
		ChannelOptions: map[string]map[string]string{
			"feishu/main": {"owner_open_id": "owner-a"},
		},
	})
	msg := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "owner-a",
		MessageID:        "m1",
		Text:             "三分钟后提醒我睡觉",
	}
	if err := rt.HandleInbound(ctx, msg); err != nil {
		t.Fatalf("harness cron tool create: %v", err)
	}
	if len(h.prompts) != 1 || h.prompts[0].Text != "三分钟后提醒我睡觉" {
		t.Fatalf("natural language request should go through harness first, got %#v", h.prompts)
	}
	jobs, err := db.ListCronJobs(ctx, "alpha")
	if err != nil {
		t.Fatalf("list cron jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one cron job, got %#v", jobs)
	}
	if jobs[0].ScheduleType != model.CronScheduleTypeAt || jobs[0].Prompt != "提醒我睡觉" || jobs[0].DeliveryMode != model.CronDeliveryOrigin {
		t.Fatalf("unexpected reminder job: %#v", jobs[0])
	}
	wantNext := time.Date(2099, 5, 22, 17, 10, 0, 0, time.UTC)
	if !jobs[0].NextRunAt.Equal(wantNext) {
		t.Fatalf("unexpected next run: got %s want %s", jobs[0].NextRunAt, wantNext)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "Cron job created") || strings.Contains(s.messages[0].Text, "acpa-cron") {
		t.Fatalf("expected cron tool confirmation without raw tool block, got %#v", s.messages)
	}
}

func TestRuntimeExecutesHarnessCronToolListAndDelete(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "owner-a",
		MessageID:        "m1",
		Text:             "列出提醒",
	}
	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "daily",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeEvery,
		ScheduleExpr: "24h",
		Timezone:     "UTC",
		Prompt:       "daily check",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryOrigin,
		Creator:      owner.BindingKey(),
		NextRunAt:    time.Date(2099, 5, 23, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create cron job: %v", err)
	}
	h := &fakeHarness{finalText: "```acpa-cron\n{\"action\":\"list\"}\n```"}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
		ChannelOptions: map[string]map[string]string{
			"feishu/main": {"owner_open_id": "owner-a"},
		},
	})
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("harness cron tool list: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, job.ID) || strings.Contains(s.messages[0].Text, "acpa-cron") {
		t.Fatalf("expected list response without raw tool block, got %#v", s.messages)
	}
	deleteMsg := owner
	deleteMsg.MessageID = "m2"
	deleteMsg.Text = "删除这个提醒"
	h.finalText = "```acpa-cron\n{\"action\":\"delete\",\"job_id\":\"" + job.ID + "\"}\n```"
	if err := rt.HandleInbound(ctx, deleteMsg); err != nil {
		t.Fatalf("harness cron tool delete: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "Cron job removed: "+job.ID) {
		t.Fatalf("expected delete confirmation, got %#v", s.messages)
	}
	jobs, err := db.ListCronJobs(ctx, "alpha")
	if err != nil {
		t.Fatalf("list cron jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("delete tool should remove job, got %#v", jobs)
	}
}

func TestRuntimeExecutesCronRunAndDeliversResult(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	now := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	creator := model.SessionBindingKey{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "owner-a",
	}
	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "report",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeEvery,
		ScheduleExpr: "1h",
		Timezone:     "UTC",
		Prompt:       "summarize workspace",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryOrigin,
		Creator:      creator,
		NextRunAt:    now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create cron job: %v", err)
	}
	run, err := db.CreateManualCronRun(ctx, "alpha", job.ID, now)
	if err != nil {
		t.Fatalf("manual run: %v", err)
	}
	if err := rt.ExecuteCronRun(ctx, run); err != nil {
		t.Fatalf("execute cron run: %v", err)
	}
	if len(h.prompts) != 1 || h.prompts[0].Text != "summarize workspace" {
		t.Fatalf("cron prompt not dispatched: %#v", h.prompts)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "reply: summarize workspace") || s.messages[0].PlatformUserID != "owner-a" {
		t.Fatalf("cron result should be delivered to origin, got %#v", s.messages)
	}
	completed, err := db.CronRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if completed.Status != model.CronRunStatusSucceeded || completed.FinalText != "reply: summarize workspace" || completed.LocalSessionID == "" {
		t.Fatalf("run should be recorded as succeeded, got %#v", completed)
	}
	loaded, err := db.CronJob(ctx, "alpha", job.ID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if !loaded.Enabled || !loaded.NextRunAt.Equal(job.NextRunAt) {
		t.Fatalf("manual run should preserve schedule, got %#v", loaded)
	}
}

func TestRuntimeRunsDueCronJobsAndAdvancesSchedule(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	due := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "report",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeEvery,
		ScheduleExpr: "30m",
		Timezone:     "UTC",
		Prompt:       "check status",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryNone,
		Creator: model.SessionBindingKey{
			AssistantID:      "alpha",
			Platform:         model.PlatformFeishu,
			AccountID:        "main",
			PrivateChannelID: "chat-a",
			PlatformUserID:   "owner-a",
		},
		NextRunAt: due,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := rt.RunDueCronJobs(ctx, due, 10); err != nil {
		t.Fatalf("run due jobs: %v", err)
	}
	if len(h.prompts) != 1 || h.prompts[0].Text != "check status" {
		t.Fatalf("due job was not prompted: %#v", h.prompts)
	}
	loaded, err := db.CronJob(ctx, "alpha", job.ID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if !loaded.Enabled || loaded.Running || !loaded.NextRunAt.Equal(due.Add(30*time.Minute)) {
		t.Fatalf("recurring job should advance and unlock, got %#v", loaded)
	}
}

func TestRuntimeCronDeliverySuppressesSilentSuccessAndReportsFailure(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{finalText: "[SILENT] no changes"}
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Sender:      s,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	now := time.Date(2026, 5, 23, 8, 0, 0, 0, time.UTC)
	creator := model.SessionBindingKey{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "owner-a",
	}
	job, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "silent",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeEvery,
		ScheduleExpr: "1h",
		Timezone:     "UTC",
		Prompt:       "quiet check",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryOrigin,
		Creator:      creator,
		NextRunAt:    now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create silent job: %v", err)
	}
	run, err := db.CreateManualCronRun(ctx, "alpha", job.ID, now)
	if err != nil {
		t.Fatalf("manual silent run: %v", err)
	}
	if err := rt.ExecuteCronRun(ctx, run); err != nil {
		t.Fatalf("execute silent run: %v", err)
	}
	if len(s.messages) != 0 {
		t.Fatalf("silent success should not deliver, got %#v", s.messages)
	}

	h.finalText = ""
	h.promptErr = errors.New("boom")
	failJob, err := db.CreateCronJob(ctx, model.CronJob{
		AssistantID:  "alpha",
		Name:         "failure",
		Enabled:      true,
		ScheduleType: model.CronScheduleTypeEvery,
		ScheduleExpr: "1h",
		Timezone:     "UTC",
		Prompt:       "fail check",
		Target:       model.CronTargetIsolated,
		DeliveryMode: model.CronDeliveryOrigin,
		Creator:      creator,
		NextRunAt:    now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create failure job: %v", err)
	}
	failRun, err := db.CreateManualCronRun(ctx, "alpha", failJob.ID, now)
	if err != nil {
		t.Fatalf("manual failure run: %v", err)
	}
	if err := rt.ExecuteCronRun(ctx, failRun); err != nil {
		t.Fatalf("execute failure run: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "Cron job failure failed: boom") {
		t.Fatalf("failure should deliver error, got %#v", s.messages)
	}
	completed, err := db.CronRun(ctx, failRun.ID)
	if err != nil {
		t.Fatalf("load failed run: %v", err)
	}
	if completed.Status != model.CronRunStatusFailed || completed.Error != "boom" {
		t.Fatalf("failure should be recorded, got %#v", completed)
	}
}

func TestRuntimeSkillsCommandsReportSources(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	acpaHome := filepath.Join(root, "acpa")
	configspace := filepath.Join(root, "config")
	writeTestSkill(t, filepath.Join(acpaHome, "global", "skills", "global-one"), "global-one", "Global skill")
	writeTestSkill(t, filepath.Join(configspace, "skills", "assistant-one"), "assistant-one", "Assistant skill")
	s := &fakeSender{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID:     "alpha",
		Provider:        model.ProviderCodex,
		Store:           db,
		Sender:          s,
		ACPAHome:        acpaHome,
		ConfigspacePath: configspace,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
			Users: map[string]model.Policy{
				"feishu/main/owner-a": {Admin: true},
			},
		},
	})
	user := model.InboundMessage{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		MessageID:        "m1",
		Text:             "/skills",
	}
	if err := rt.HandleInbound(ctx, user); err != nil {
		t.Fatalf("skills: %v", err)
	}
	if len(s.messages) != 1 || !strings.Contains(s.messages[0].Text, "global-one: Global skill") || !strings.Contains(s.messages[0].Text, "assistant-one: Assistant skill") || strings.Contains(s.messages[0].Text, "source:") {
		t.Fatalf("skills output should list names and descriptions only, got %#v", s.messages)
	}
	verboseDenied := user
	verboseDenied.MessageID = "m2"
	verboseDenied.Text = "/skills verbose"
	if err := rt.HandleInbound(ctx, verboseDenied); err != nil {
		t.Fatalf("skills verbose denied: %v", err)
	}
	if len(s.messages) != 2 || !strings.Contains(s.messages[1].Text, "Owner permission is required") {
		t.Fatalf("verbose skills should require owner/admin, got %#v", s.messages)
	}
	owner := user
	owner.MessageID = "m3"
	owner.PlatformUserID = "owner-a"
	owner.Text = "/skills verbose"
	if err := rt.HandleInbound(ctx, owner); err != nil {
		t.Fatalf("skills verbose: %v", err)
	}
	if len(s.messages) != 3 || !strings.Contains(s.messages[2].Text, "global:") || !strings.Contains(s.messages[2].Text, "assistant:") || !strings.Contains(s.messages[2].Text, filepath.Join(configspace, "skills", "assistant-one")) || !strings.Contains(s.messages[2].Text, filepath.Join(configspace, "harness", "codex-home", "skills")) {
		t.Fatalf("verbose skills should include source layers and paths, got %#v", s.messages)
	}
}

func writeTestSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\nBody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func TestRuntimeHandlesPermissionDecisionsFromCards(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	h := &fakeHarness{}
	rt := assistant.NewRuntime(assistant.RuntimeConfig{
		AssistantID: "alpha",
		Provider:    model.ProviderCodex,
		Store:       db,
		Harness:     h,
		Policy: model.PolicySet{
			Assistant: model.Policy{AllowedModes: []model.PermissionMode{model.PermissionManual}, DefaultMode: model.PermissionManual},
		},
	})
	owner := model.SessionBindingKey{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
	}
	session, err := db.CreateSession(ctx, owner, model.PermissionManual, "manual")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	perm, err := rt.RecordPermissionRequest(ctx, assistant.PermissionRequest{
		LocalSessionID:    session.ID,
		ACPRequestID:      "req-1",
		Options:           []string{"approved", "abort"},
		TimeoutResolution: "abort",
	})
	if err != nil {
		t.Fatalf("record permission: %v", err)
	}

	if err := rt.HandlePermissionDecision(ctx, model.PermissionDecision{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-b",
		EventID:          "card-1",
		ShortApprovalID:  perm.ShortApprovalID,
		Option:           "approve",
	}); err == nil {
		t.Fatal("non-owner card approval should be rejected")
	}
	if len(h.resolvedPermissions) != 0 {
		t.Fatalf("non-owner should not resolve permission: %#v", h.resolvedPermissions)
	}

	if err := rt.HandlePermissionDecision(ctx, model.PermissionDecision{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		EventID:          "card-2",
		ShortApprovalID:  perm.ShortApprovalID,
		Option:           "approve",
	}); err != nil {
		t.Fatalf("owner card approval: %v", err)
	}
	if len(h.resolvedPermissions) != 1 || h.resolvedPermissions[0] != perm.ShortApprovalID+":approved" {
		t.Fatalf("approval was not forwarded: %#v", h.resolvedPermissions)
	}
	if err := rt.HandlePermissionDecision(ctx, model.PermissionDecision{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		EventID:          "card-2",
		ShortApprovalID:  perm.ShortApprovalID,
		Option:           "approve",
	}); err != nil {
		t.Fatalf("duplicate card callback should be ignored: %v", err)
	}
	if err := rt.HandlePermissionDecision(ctx, model.PermissionDecision{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		EventID:          "card-3",
		ShortApprovalID:  perm.ShortApprovalID,
		Option:           "reject",
	}); err != nil {
		t.Fatalf("stale card callback should be ignored: %v", err)
	}
	if len(h.resolvedPermissions) != 1 {
		t.Fatalf("stale callback should not resolve again: %#v", h.resolvedPermissions)
	}
	if err := rt.HandlePermissionDecision(ctx, model.PermissionDecision{
		AssistantID:      "alpha",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "chat-a",
		PlatformUserID:   "user-a",
		EventID:          "card-4",
		Option:           "approve",
	}); err == nil {
		t.Fatal("missing approval id should be rejected")
	}
}
