package im

import (
	"context"
	"errors"
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

	if err := account.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !fake.stopped {
		t.Fatal("expected SDK long connection to stop")
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
	onMessage      func(context.Context, *channeltypes.NormalizedMessage) error
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
	return nil, errors.New("send not used")
}

func (f *fakeFeishuLongConnection) OnMessage(handler func(context.Context, *channeltypes.NormalizedMessage) error) {
	f.onMessage = handler
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
