package im

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/gorilla/websocket"
	channeltypes "github.com/larksuite/oapi-sdk-go/v3/channel/types"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
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
	decisions := make(chan model.PermissionDecision, 1)
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
			decisions <- decision
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
	select {
	case decision := <-decisions:
		if decision.EventID != "card-event-1" || decision.ShortApprovalID != "ABC123" || decision.Option != "approve" || decision.PlatformUserID != "ou_user" {
			t.Fatalf("unexpected permission decision: %#v", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("expected one permission decision")
	}

	if err := account.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !fake.stopped {
		t.Fatal("expected SDK long connection to stop")
	}
}

func TestFeishuCardActionAcknowledgesBeforePermissionDecisionCompletes(t *testing.T) {
	block := make(chan struct{})
	account := NewFeishuAccount(AccountConfig{
		AssistantID: "assistant-1",
		Channel: model.ChannelConfig{
			Platform:  model.PlatformFeishu,
			AccountID: "main",
		},
		OnPermissionDecision: func(ctx context.Context, decision model.PermissionDecision) error {
			<-block
			return errors.New("late resolution failure")
		},
	})
	resp, err := account.handleFeishuCardAction(context.Background(), &channeltypes.CardActionEvent{
		EventID:   "card-event-1",
		MessageID: "om_card",
		ChatID:    "oc_private",
		Operator:  channeltypes.CardActionOperator{OpenID: "ou_user"},
		Action: channeltypes.CardActionPayload{Value: map[string]interface{}{
			"short_approval_id": "ABC123",
			"option":            "approve",
		}},
	})
	if err != nil {
		t.Fatalf("card action should be acknowledged immediately, got %v", err)
	}
	if resp == nil || resp.Card == nil {
		t.Fatalf("card action should return an updated card response, got %#v", resp)
	}
	close(block)
}

func TestFeishuCardActionReturnsUpdatedStatusCard(t *testing.T) {
	account := NewFeishuAccount(AccountConfig{
		AssistantID: "assistant-1",
		Channel: model.ChannelConfig{
			Platform:  model.PlatformFeishu,
			AccountID: "main",
		},
		OnPermissionDecision: func(ctx context.Context, decision model.PermissionDecision) error {
			return nil
		},
	})
	resp, err := account.handleFeishuCardAction(context.Background(), &channeltypes.CardActionEvent{
		EventID:   "card-event-1",
		MessageID: "om_card",
		ChatID:    "oc_private",
		Operator:  channeltypes.CardActionOperator{OpenID: "ou_user"},
		Action: channeltypes.CardActionPayload{Value: map[string]interface{}{
			"short_approval_id": "ABC123",
			"option":            "approve",
		}},
	})
	if err != nil {
		t.Fatalf("card action: %v", err)
	}
	if resp == nil || resp.Card == nil || resp.Card.Type != "raw" {
		t.Fatalf("expected raw card update response, got %#v", resp)
	}
	cardData, err := json.Marshal(resp.Card.Data)
	if err != nil {
		t.Fatalf("marshal response card: %v", err)
	}
	card := string(cardData)
	if !strings.Contains(card, "已授权") || strings.Contains(card, `"tag":"button"`) {
		t.Fatalf("expected approved status card without buttons, got %s", card)
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

func TestFeishuAccountStreamsOrdinaryCardWithoutTitle(t *testing.T) {
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
		Secrets: map[string]string{"app_id": "cli_test", "app_secret": "secret_test"},
	})
	if err := account.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("expected SDK long connection to start")
	}
	stream, err := account.StartStream(context.Background(), model.OutboundMessage{
		AssistantID:      "assistant-1",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "oc_private",
		PlatformUserID:   "ou_user",
		Stream:           &model.OutboundStreamOptions{Kind: model.OutboundStreamNormal},
	})
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	if err := stream.Append(context.Background(), "hello"); err != nil {
		t.Fatalf("append stream: %v", err)
	}
	if len(fake.sent) != 1 || len(fake.streams) != 1 {
		t.Fatalf("expected one stream card, sent=%#v streams=%#v", fake.sent, fake.streams)
	}
	if fake.sent[0].MsgType != "interactive" || fake.sent[0].Card == "" {
		t.Fatalf("expected initial interactive card, got %#v", fake.sent[0])
	}
	updated := fake.streams[0].cards[len(fake.streams[0].cards)-1]
	if !strings.Contains(updated, "hello") {
		t.Fatalf("updated card should contain streamed text: %s", updated)
	}
	if strings.Contains(updated, `"header"`) {
		t.Fatalf("ordinary stream card should not contain header/title: %s", updated)
	}
}

func TestFeishuAccountStartsNewCardForNewStreamSegment(t *testing.T) {
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
		Secrets: map[string]string{"app_id": "cli_test", "app_secret": "secret_test"},
	})
	if err := account.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("expected SDK long connection to start")
	}
	msg := model.OutboundMessage{
		AssistantID:      "assistant-1",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "oc_private",
		PlatformUserID:   "ou_user",
		Stream:           &model.OutboundStreamOptions{Kind: model.OutboundStreamNormal},
	}
	first, err := account.StartStream(context.Background(), msg)
	if err != nil {
		t.Fatalf("first stream: %v", err)
	}
	if err := first.Append(context.Background(), "before"); err != nil {
		t.Fatalf("first append: %v", err)
	}
	second, err := account.StartStream(context.Background(), msg)
	if err != nil {
		t.Fatalf("second stream: %v", err)
	}
	if err := second.Append(context.Background(), "after"); err != nil {
		t.Fatalf("second append: %v", err)
	}
	if len(fake.sent) != 2 || len(fake.streams) != 2 {
		t.Fatalf("expected two independent stream cards, sent=%#v streams=%#v", fake.sent, fake.streams)
	}
	if !strings.Contains(fake.streams[0].cards[len(fake.streams[0].cards)-1], "before") {
		t.Fatalf("first stream did not keep first segment: %#v", fake.streams[0].cards)
	}
	if !strings.Contains(fake.streams[1].cards[len(fake.streams[1].cards)-1], "after") {
		t.Fatalf("second stream did not keep second segment: %#v", fake.streams[1].cards)
	}
}

func TestFeishuAccountStreamsCronCardWithTitleAndID(t *testing.T) {
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
		Secrets: map[string]string{"app_id": "cli_test", "app_secret": "secret_test"},
	})
	if err := account.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("expected SDK long connection to start")
	}
	stream, err := account.StartStream(context.Background(), model.OutboundMessage{
		AssistantID:      "assistant-1",
		Platform:         model.PlatformFeishu,
		AccountID:        "main",
		PrivateChannelID: "oc_private",
		PlatformUserID:   "ou_user",
		Stream:           &model.OutboundStreamOptions{Kind: model.OutboundStreamCron, CronID: "cron_123", CronTitle: "Daily Summary"},
	})
	if err != nil {
		t.Fatalf("start cron stream: %v", err)
	}
	if !strings.Contains(fake.sent[0].Card, "Daily Summary") || !strings.Contains(fake.sent[0].Card, "Cron reply") || !strings.Contains(fake.sent[0].Card, "cron_123") {
		t.Fatalf("initial cron card should include title and cron id: %s", fake.sent[0].Card)
	}
	if err := stream.Append(context.Background(), "cron body"); err != nil {
		t.Fatalf("append cron stream: %v", err)
	}
	updated := fake.streams[0].cards[len(fake.streams[0].cards)-1]
	if !strings.Contains(updated, "Daily Summary") || !strings.Contains(updated, "Cron reply") || !strings.Contains(updated, "cron_123") || !strings.Contains(updated, "cron body") {
		t.Fatalf("updated cron card should include title, cron id, and body: %s", updated)
	}
}

func TestFeishuLongConnectionHandlesCardFrames(t *testing.T) {
	conn := newFeishuSDKLongConnection(feishuLongConnectionConfig{
		AppID:     "cli_test",
		AppSecret: "secret_test",
	})
	done := make(chan *channeltypes.CardActionEvent, 1)
	var written []byte
	conn.writeMessageHook = func(messageType int, data []byte) error {
		if messageType != websocket.BinaryMessage {
			t.Fatalf("expected binary ACK frame, got %d", messageType)
		}
		written = append([]byte(nil), data...)
		return nil
	}
	conn.OnCardAction(func(ctx context.Context, event *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error) {
		done <- event
		return &callback.CardActionTriggerResponse{
			Card: &callback.Card{
				Type: "raw",
				Data: map[string]any{
					"elements": []any{map[string]any{"tag": "div", "text": map[string]string{"content": "已授权"}}},
				},
			},
		}, nil
	})
	headers := larkws.Headers{}
	headers.Add(larkws.HeaderType, string(larkws.MessageTypeCard))
	headers.Add(larkws.HeaderMessageID, "msg-card-1")
	headers.Add(larkws.HeaderSum, "1")
	headers.Add(larkws.HeaderSeq, "0")
	conn.handleDataFrame(context.Background(), larkws.Frame{
		Method:  int32(larkws.FrameTypeData),
		Headers: headers,
		Payload: []byte(`{
			"schema": "2.0",
			"header": {
				"event_id": "evt_card_1",
				"event_type": "card.action.trigger",
				"create_time": "1700000000000"
			},
			"event": {
				"operator": {"open_id": "ou_user"},
				"context": {
					"open_message_id": "om_card",
					"open_chat_id": "oc_private"
				},
				"action": {
					"tag": "button",
					"name": "approve",
					"value": {
						"short_approval_id": "ABC123",
						"option": "approve"
					}
				}
			}
		}`),
	})
	select {
	case event := <-done:
		if event.EventID != "evt_card_1" || event.MessageID != "om_card" || event.ChatID != "oc_private" {
			t.Fatalf("unexpected card action event: %#v", event)
		}
		if event.Operator.OpenID != "ou_user" || event.Action.Value["short_approval_id"] != "ABC123" || event.Action.Value["option"] != "approve" {
			t.Fatalf("unexpected card action payload: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected card action frame to reach handler")
	}
	var ackFrame larkws.Frame
	if err := ackFrame.Unmarshal(written); err != nil {
		t.Fatalf("unmarshal ACK frame: %v", err)
	}
	var ack larkws.Response
	if err := json.Unmarshal(ackFrame.Payload, &ack); err != nil {
		t.Fatalf("unmarshal ACK payload: %v", err)
	}
	if ack.StatusCode != 200 {
		t.Fatalf("expected ACK status 200, got %#v", ack)
	}
	var callbackResp callback.CardActionTriggerResponse
	if err := json.Unmarshal(ack.Data, &callbackResp); err != nil {
		t.Fatalf("unmarshal callback response: %v", err)
	}
	if callbackResp.Card == nil || callbackResp.Card.Type != "raw" {
		t.Fatalf("expected card update in ACK, got %#v", callbackResp)
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
	streams        []*fakeFeishuStreamController
	failSend       bool
	failStream     bool
	onMessage      func(context.Context, *channeltypes.NormalizedMessage) error
	onCardAction   func(context.Context, *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error)
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

func (f *fakeFeishuLongConnection) Stream(ctx context.Context, input *channeltypes.SendInput) (channeltypes.StreamController, error) {
	f.sent = append(f.sent, input)
	if f.failStream {
		return nil, errors.New("stream failed")
	}
	stream := &fakeFeishuStreamController{}
	f.streams = append(f.streams, stream)
	return stream, nil
}

type fakeFeishuStreamController struct {
	cards    []string
	finished bool
}

func (f *fakeFeishuStreamController) Append(ctx context.Context, text string) error {
	return errors.New("append is not supported")
}

func (f *fakeFeishuStreamController) UpdateCard(ctx context.Context, card string) error {
	f.cards = append(f.cards, card)
	return nil
}

func (f *fakeFeishuStreamController) Flush(ctx context.Context) error {
	return nil
}

func (f *fakeFeishuStreamController) Close(ctx context.Context) error {
	f.finished = true
	return nil
}

func (f *fakeFeishuLongConnection) OnMessage(handler func(context.Context, *channeltypes.NormalizedMessage) error) {
	f.onMessage = handler
}

func (f *fakeFeishuLongConnection) OnCardAction(handler func(context.Context, *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error)) {
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
	if _, err := f.onCardAction(context.Background(), event); err != nil {
		t.Fatalf("card action handler: %v", err)
	}
}
