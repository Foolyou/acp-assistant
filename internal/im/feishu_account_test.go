package im

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	channeltypes "github.com/larksuite/oapi-sdk-go/v3/channel/types"
)

func TestFeishuAccountStartUsesSDKLongConnectionWithoutWebsocketURL(t *testing.T) {
	originalFactory := newFeishuLongConnection
	defer func() { newFeishuLongConnection = originalFactory }()

	fake := &fakeFeishuLongConnection{started: make(chan context.Context, 1)}
	newFeishuLongConnection = func(cfg feishuLongConnectionConfig) (feishuLongConnection, error) {
		if cfg.AppID != "cli_test" || cfg.AppSecret != "secret_test" {
			t.Fatalf("unexpected SDK credentials: %#v", cfg)
		}
		if cfg.Domain != "feishu" {
			t.Fatalf("unexpected domain: %#v", cfg)
		}
		if cfg.HTTPClient != nil {
			t.Fatalf("nil HTTP client should not be wrapped into SDK config: %#v", cfg.HTTPClient)
		}
		return fake, nil
	}

	var inbound []model.InboundMessage
	var decisions []model.PermissionDecision
	var statuses []model.ConnectorStatus
	account := NewFeishuAccount(AccountConfig{
		AssistantID: "assistant-1",
		Channel: model.ChannelConfig{
			Platform:  model.PlatformFeishu,
			AccountID: "main",
			Enabled:   true,
			Options:   map[string]string{"domain": "feishu"},
		},
		Secrets: map[string]string{
			"app_id":     "cli_test",
			"app_secret": "secret_test",
		},
		OnInbound: func(ctx context.Context, msg model.InboundMessage) error {
			inbound = append(inbound, msg)
			return nil
		},
		OnPermissionDecision: func(ctx context.Context, decision model.PermissionDecision) error {
			decisions = append(decisions, decision)
			return nil
		},
		OnStatus: func(ctx context.Context, status model.ConnectorStatus) error {
			statuses = append(statuses, status)
			return nil
		},
	})

	if err := account.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("expected SDK long connection to start")
	}

	fake.ready()
	fake.message(t, &channeltypes.NormalizedMessage{
		EventID:      "event-1",
		MessageID:    "message-1",
		ChatID:       "oc_private",
		ChatType:     "p2p",
		UserID:       "ou_user",
		Content:      "hello",
		CreateTimeMs: 1710000000000,
	})
	fake.message(t, &channeltypes.NormalizedMessage{
		EventID:   "event-2",
		MessageID: "message-2",
		ChatID:    "oc_group",
		ChatType:  "group",
		UserID:    "ou_user",
		Content:   "ignored",
	})

	if len(inbound) != 1 {
		t.Fatalf("expected one accepted p2p message, got %#v", inbound)
	}
	if inbound[0].PrivateChannelID != "oc_private" || inbound[0].PlatformUserID != "ou_user" || inbound[0].Text != "hello" {
		t.Fatalf("unexpected inbound message: %#v", inbound[0])
	}
	if !hasConnectorState(statuses, model.ConnectorStateConnected) {
		t.Fatalf("expected connected status, got %#v", statuses)
	}
	if !hasStatusMessage(statuses, "ignored non-private inbound event") {
		t.Fatalf("expected rejected non-private status, got %#v", statuses)
	}
	fake.cardAction(t, &channeltypes.CardActionEvent{
		EventID:   "card-event-1",
		MessageID: "om_card",
		ChatID:    "oc_private",
		Operator:  channeltypes.CardActionOperator{OpenID: "ou_user"},
		Action: channeltypes.CardActionPayload{Value: map[string]interface{}{
			"short_approval_id": "ABC123",
			"option":            "approve",
		}},
	})
	if len(decisions) != 1 {
		t.Fatalf("expected one permission decision, got %#v", decisions)
	}
	if decisions[0].EventID != "card-event-1" || decisions[0].ShortApprovalID != "ABC123" || decisions[0].Option != "approve" || decisions[0].PlatformUserID != "ou_user" {
		t.Fatalf("unexpected permission decision: %#v", decisions[0])
	}

	if err := account.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !fake.stopped {
		t.Fatal("expected SDK long connection to stop")
	}
}

func TestFeishuAccountSendsPermissionPromptCardWithTextFallback(t *testing.T) {
	originalFactory := newFeishuLongConnection
	defer func() { newFeishuLongConnection = originalFactory }()

	fake := &fakeFeishuLongConnection{started: make(chan context.Context, 1)}
	newFeishuLongConnection = func(cfg feishuLongConnectionConfig) (feishuLongConnection, error) {
		return fake, nil
	}
	account := NewFeishuAccount(AccountConfig{
		AssistantID: "assistant-1",
		Channel: model.ChannelConfig{
			Platform:  model.PlatformFeishu,
			AccountID: "main",
			Enabled:   true,
			Options:   map[string]string{"domain": "feishu"},
		},
		Secrets: map[string]string{
			"app_id":     "cli_test",
			"app_secret": "secret_test",
		},
	})
	if err := account.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("expected SDK long connection to start")
	}
	err := account.Send(context.Background(), model.OutboundMessage{
		AssistantID:      "assistant-1",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "oc_private",
		PlatformUserID:   "ou_user",
		Text:             "approve ABC123 or reject ABC123",
		PermissionPrompt: &model.PermissionPrompt{
			ShortApprovalID: "ABC123",
			Options:         []string{"approve", "reject"},
			Text:            "approve ABC123 or reject ABC123",
		},
	})
	if err != nil {
		t.Fatalf("send prompt card: %v", err)
	}
	if len(fake.sent) != 1 {
		t.Fatalf("expected one send, got %#v", fake.sent)
	}
	if fake.sent[0].MsgType != "interactive" || fake.sent[0].Card == "" || fake.sent[0].Text != "" {
		t.Fatalf("expected interactive card send, got %#v", fake.sent[0])
	}
	if !strings.Contains(fake.sent[0].Card, "ABC123") || !strings.Contains(fake.sent[0].Card, "approve") || !strings.Contains(fake.sent[0].Card, "reject") {
		t.Fatalf("card should include approval id and actions: %s", fake.sent[0].Card)
	}

	fake.failSend = true
	if err := account.Send(context.Background(), model.OutboundMessage{
		AssistantID:      "assistant-1",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "oc_private",
		PlatformUserID:   "ou_user",
		Text:             "approve DEF456 or reject DEF456",
		PermissionPrompt: &model.PermissionPrompt{ShortApprovalID: "DEF456", Text: "approve DEF456 or reject DEF456"},
	}); err != nil {
		t.Fatalf("fallback send should succeed: %v", err)
	}
	if len(fake.sent) != 3 {
		t.Fatalf("expected failed card attempt and text fallback, got %#v", fake.sent)
	}
	if fake.sent[2].MsgType != "text" || fake.sent[2].Text != "approve DEF456 or reject DEF456" {
		t.Fatalf("expected text fallback, got %#v", fake.sent[2])
	}
}

func TestFeishuWSClientInstallsEventDispatcher(t *testing.T) {
	wsClient := newFeishuWSClient(feishuLongConnectionConfig{
		AppID:     "cli_test",
		AppSecret: "secret_test",
		Domain:    "feishu",
	}, "https://open.feishu.cn")
	if wsClient.EventHandler() == nil {
		t.Fatal("expected Feishu WebSocket client to have an event dispatcher")
	}
}

func hasConnectorState(statuses []model.ConnectorStatus, state model.ConnectorState) bool {
	for _, status := range statuses {
		if status.State == state {
			return true
		}
	}
	return false
}

func hasStatusMessage(statuses []model.ConnectorStatus, message string) bool {
	for _, status := range statuses {
		if status.Message == message {
			return true
		}
	}
	return false
}

type fakeFeishuLongConnection struct {
	started        chan context.Context
	stopped        bool
	sent           []*channeltypes.SendInput
	failSend       bool
	onMessage      func(context.Context, *channeltypes.NormalizedMessage) error
	onCardAction   func(context.Context, *channeltypes.CardActionEvent) error
	onReady        func()
	onReject       func(context.Context, *channeltypes.RejectEvent) error
	onError        func(error)
	onReconnecting func()
	onReconnected  func()
	onDisconnected func()
}

func (f *fakeFeishuLongConnection) Start(ctx context.Context) error {
	f.started <- ctx
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeFeishuLongConnection) Stop(ctx context.Context) error {
	f.stopped = true
	return nil
}

func (f *fakeFeishuLongConnection) Send(ctx context.Context, input *channeltypes.SendInput) (*channeltypes.SendResult, error) {
	f.sent = append(f.sent, input)
	if f.failSend && input.MsgType == "interactive" {
		return nil, errors.New("card send failed")
	}
	return &channeltypes.SendResult{MessageID: "om_sent"}, nil
}

func (f *fakeFeishuLongConnection) OnMessage(handler func(context.Context, *channeltypes.NormalizedMessage) error) {
	f.onMessage = handler
}

func (f *fakeFeishuLongConnection) OnCardAction(handler func(context.Context, *channeltypes.CardActionEvent) error) {
	f.onCardAction = handler
}

func (f *fakeFeishuLongConnection) OnReject(handler func(context.Context, *channeltypes.RejectEvent) error) {
	f.onReject = handler
}

func (f *fakeFeishuLongConnection) OnReady(handler func()) {
	f.onReady = handler
}

func (f *fakeFeishuLongConnection) OnError(handler func(error)) {
	f.onError = handler
}

func (f *fakeFeishuLongConnection) OnReconnecting(handler func()) {
	f.onReconnecting = handler
}

func (f *fakeFeishuLongConnection) OnReconnected(handler func()) {
	f.onReconnected = handler
}

func (f *fakeFeishuLongConnection) OnDisconnected(handler func()) {
	f.onDisconnected = handler
}

func (f *fakeFeishuLongConnection) ready() {
	if f.onReady != nil {
		f.onReady()
	}
}

func (f *fakeFeishuLongConnection) message(t *testing.T, msg *channeltypes.NormalizedMessage) {
	t.Helper()
	if f.onMessage == nil {
		t.Fatal("message handler was not registered")
	}
	if err := f.onMessage(context.Background(), msg); err != nil {
		t.Fatalf("message handler: %v", err)
	}
}

func (f *fakeFeishuLongConnection) cardAction(t *testing.T, event *channeltypes.CardActionEvent) {
	t.Helper()
	if f.onCardAction == nil {
		t.Fatal("card action handler was not registered")
	}
	if err := f.onCardAction(context.Background(), event); err != nil {
		t.Fatalf("card action handler: %v", err)
	}
}
