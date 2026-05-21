package assistant_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/assistant"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

type fakeHarness struct {
	prompts             []assistant.PromptRequest
	resolvedPermissions []string
}

func (f *fakeHarness) EnsureSession(ctx context.Context, req assistant.EnsureSessionRequest) (assistant.EnsureSessionResult, error) {
	return assistant.EnsureSessionResult{ACPSessionID: "acp-" + req.LocalSessionID, ExternalSessionID: "external-" + req.LocalSessionID}, nil
}

func (f *fakeHarness) Prompt(ctx context.Context, req assistant.PromptRequest) (assistant.PromptResult, error) {
	f.prompts = append(f.prompts, req)
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
	if err := rt.HandleInbound(ctx, attacker); err == nil {
		t.Fatal("non-owner approval should be rejected")
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
