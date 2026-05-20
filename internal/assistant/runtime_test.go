package assistant_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/assistant"
	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

type fakeHarness struct {
	prompts []assistant.PromptRequest
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
}
