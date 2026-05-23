package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/gorilla/websocket"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	channeltypes "github.com/larksuite/oapi-sdk-go/v3/channel/types"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

type Decision string

const (
	DecisionAccepted           Decision = "accepted"
	DecisionIgnored            Decision = "ignored"
	DecisionRejectedNonPrivate Decision = "rejected_non_private"
)

type Account interface {
	Start(context.Context) error
	Stop(context.Context) error
	Send(context.Context, model.OutboundMessage) error
	Status() model.ConnectorStatus
	Logs() []string
	RefreshToken(context.Context) error
}

type InboundHandler func(context.Context, model.InboundMessage) error
type PermissionDecisionHandler func(context.Context, model.PermissionDecision) error
type StatusRecorder func(context.Context, model.ConnectorStatus) error

type AccountConfig struct {
	AssistantID          string
	Channel              model.ChannelConfig
	Secrets              map[string]string
	HTTPClient           *http.Client
	OnInbound            InboundHandler
	OnPermissionDecision PermissionDecisionHandler
	OnStatus             StatusRecorder
}

type feishuLongConnection interface {
	Start(context.Context) error
	Stop(context.Context) error
	Send(context.Context, *channeltypes.SendInput) (*channeltypes.SendResult, error)
	Stream(context.Context, *channeltypes.SendInput) (channeltypes.StreamController, error)
	OnMessage(func(context.Context, *channeltypes.NormalizedMessage) error)
	OnCardAction(func(context.Context, *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error))
	OnReject(func(context.Context, *channeltypes.RejectEvent) error)
	OnReady(func())
	OnError(func(error))
	OnReconnecting(func())
	OnReconnected(func())
	OnDisconnected(func())
}

type feishuLongConnectionConfig struct {
	AppID        string
	AppSecret    string
	Domain       string
	OpenBaseURL  string
	OAuthBaseURL string
	HTTPClient   *http.Client
}

var newFeishuLongConnection = func(cfg feishuLongConnectionConfig) (feishuLongConnection, error) {
	return newFeishuSDKLongConnection(cfg), nil
}

type BaseAccount struct {
	cfg    AccountConfig
	mu     sync.Mutex
	status model.ConnectorStatus
	logs   []string
	cancel context.CancelFunc
}

func (a *BaseAccount) Status() model.ConnectorStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

func (a *BaseAccount) Logs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string{}, a.logs...)
}

func (a *BaseAccount) setStatus(ctx context.Context, state model.ConnectorState, message, lastErr string) error {
	status := model.ConnectorStatus{
		AssistantID: a.cfg.AssistantID,
		Platform:    a.cfg.Channel.Platform,
		AccountID:   a.cfg.Channel.AccountID,
		State:       state,
		Message:     message,
		LastError:   lastErr,
		UpdatedAt:   time.Now().UTC(),
	}
	a.mu.Lock()
	a.status = status
	if message != "" || lastErr != "" {
		line := status.UpdatedAt.Format(time.RFC3339) + " " + string(state) + " " + strings.TrimSpace(message+" "+lastErr)
		a.logs = append(a.logs, line)
		if len(a.logs) > 500 {
			a.logs = a.logs[len(a.logs)-500:]
		}
	}
	a.mu.Unlock()
	if a.cfg.OnStatus != nil {
		return a.cfg.OnStatus(ctx, status)
	}
	return nil
}

type FeishuAccount struct {
	BaseAccount
	connMu  sync.Mutex
	conn    feishuLongConnection
	tokenMu sync.Mutex
	token   string
}

func NewFeishuAccount(cfg AccountConfig) *FeishuAccount {
	cfg.Channel.Platform = model.PlatformFeishu
	return &FeishuAccount{BaseAccount: BaseAccount{cfg: cfg}}
}

func (a *FeishuAccount) Start(ctx context.Context) error {
	if !a.cfg.Channel.Enabled {
		return a.setStatus(ctx, model.ConnectorStateDisabled, "channel disabled", "")
	}
	appID := strings.TrimSpace(a.cfg.Secrets["app_id"])
	appSecret := strings.TrimSpace(a.cfg.Secrets["app_secret"])
	if appID == "" || appSecret == "" {
		err := fmt.Errorf("Feishu app_id and app_secret are required")
		_ = a.setStatus(ctx, model.ConnectorStateFailed, "missing Feishu credentials", err.Error())
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	if err := a.setStatus(ctx, model.ConnectorStateConnecting, "starting Feishu long connection", ""); err != nil {
		return err
	}
	conn, err := newFeishuLongConnection(feishuLongConnectionConfig{
		AppID:        appID,
		AppSecret:    appSecret,
		Domain:       a.cfg.Channel.Options["domain"],
		OpenBaseURL:  a.cfg.Channel.Options["open_base_url"],
		OAuthBaseURL: a.cfg.Channel.Options["oauth_base_url"],
		HTTPClient:   a.cfg.HTTPClient,
	})
	if err != nil {
		cancel()
		_ = a.setStatus(ctx, model.ConnectorStateFailed, "failed to create Feishu long connection", err.Error())
		return err
	}
	conn.OnReady(func() {
		_ = a.setStatus(context.Background(), model.ConnectorStateConnected, "Feishu long connection ready", "")
	})
	conn.OnError(func(err error) {
		lastErr := ""
		if err != nil {
			lastErr = err.Error()
		}
		_ = a.setStatus(context.Background(), model.ConnectorStateFailed, "Feishu long connection error", lastErr)
	})
	conn.OnReconnecting(func() {
		_ = a.setStatus(context.Background(), model.ConnectorStateConnecting, "Feishu long connection reconnecting", "")
	})
	conn.OnReconnected(func() {
		_ = a.setStatus(context.Background(), model.ConnectorStateConnected, "Feishu long connection reconnected", "")
	})
	conn.OnDisconnected(func() {
		_ = a.setStatus(context.Background(), model.ConnectorStateDisconnected, "Feishu long connection disconnected", "")
	})
	conn.OnReject(func(ctx context.Context, event *channeltypes.RejectEvent) error {
		reason := ""
		if event != nil {
			reason = event.Reason
		}
		return a.setStatus(ctx, model.ConnectorStateConnected, "rejected inbound event", reason)
	})
	conn.OnMessage(a.handleFeishuSDKMessage)
	conn.OnCardAction(a.handleFeishuCardAction)
	a.connMu.Lock()
	a.conn = conn
	a.connMu.Unlock()
	go func() {
		if err := conn.Start(runCtx); err != nil && runCtx.Err() == nil {
			_ = a.setStatus(context.Background(), model.ConnectorStateFailed, "Feishu long connection stopped", err.Error())
		}
	}()
	return nil
}

func (a *FeishuAccount) Stop(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	a.connMu.Lock()
	conn := a.conn
	a.conn = nil
	a.connMu.Unlock()
	if conn != nil {
		_ = conn.Stop(ctx)
	}
	return a.setStatus(ctx, model.ConnectorStateDisconnected, "stopped", "")
}

func (a *FeishuAccount) RefreshToken(ctx context.Context) error {
	appID := a.cfg.Secrets["app_id"]
	appSecret := a.cfg.Secrets["app_secret"]
	if appID == "" || appSecret == "" {
		return fmt.Errorf("Feishu app_id and app_secret are required")
	}
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	tokenURL := defaulted(a.cfg.Secrets["token_url"], feishuOpenBaseURL(a.cfg.Channel.Options["domain"], a.cfg.Channel.Options["open_base_url"])+"/open-apis/auth/v3/tenant_access_token/internal")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(a.cfg.HTTPClient).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Feishu token refresh failed: %s", strings.TrimSpace(string(data)))
	}
	var parsed struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	if parsed.Code != 0 || parsed.TenantAccessToken == "" {
		return fmt.Errorf("Feishu token refresh failed: %s", parsed.Msg)
	}
	a.tokenMu.Lock()
	a.token = parsed.TenantAccessToken
	a.tokenMu.Unlock()
	return a.writeTokenCache(parsed.TenantAccessToken)
}

func (a *FeishuAccount) Send(ctx context.Context, msg model.OutboundMessage) error {
	a.connMu.Lock()
	conn := a.conn
	a.connMu.Unlock()
	if conn != nil {
		if msg.PermissionPrompt != nil {
			cardInput := &channeltypes.SendInput{
				ReceiveID: msg.PlatformUserID,
				ChatID:    msg.PrivateChannelID,
				MsgType:   "interactive",
				Card:      renderFeishuPermissionCard(*msg.PermissionPrompt),
			}
			if _, err := conn.Send(ctx, cardInput); err == nil {
				return nil
			}
		}
		input := &channeltypes.SendInput{
			ReceiveID: msg.PlatformUserID,
			ChatID:    msg.PrivateChannelID,
			MsgType:   "text",
			Text:      msg.Text,
		}
		_, err := conn.Send(ctx, input)
		return err
	}
	token := a.currentToken()
	if token == "" {
		if err := a.RefreshToken(ctx); err != nil {
			return err
		}
		token = a.currentToken()
	}
	payload, _ := json.Marshal(map[string]any{
		"receive_id": msg.PlatformUserID,
		"msg_type":   "text",
		"content":    mustJSONString(map[string]string{"text": msg.Text}),
	})
	url := defaulted(a.cfg.Secrets["send_url"], feishuOpenBaseURL(a.cfg.Channel.Options["domain"], a.cfg.Channel.Options["open_base_url"])+"/open-apis/im/v1/messages?receive_id_type=open_id")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(a.cfg.HTTPClient).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		return fmt.Errorf("Feishu send failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func (a *FeishuAccount) StartStream(ctx context.Context, msg model.OutboundMessage) (model.OutboundStream, error) {
	a.connMu.Lock()
	conn := a.conn
	a.connMu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("Feishu long connection is not available")
	}
	if msg.Stream == nil {
		return nil, fmt.Errorf("stream options are required")
	}
	controller, err := conn.Stream(ctx, &channeltypes.SendInput{
		ReceiveID: msg.PlatformUserID,
		ChatID:    msg.PrivateChannelID,
		MsgType:   "interactive",
		Card:      renderFeishuStreamCard(*msg.Stream, ""),
	})
	if err != nil {
		return nil, err
	}
	return &feishuOutboundStream{controller: controller, opts: *msg.Stream}, nil
}

type feishuOutboundStream struct {
	controller channeltypes.StreamController
	opts       model.OutboundStreamOptions
	text       strings.Builder
}

func (s *feishuOutboundStream) Append(ctx context.Context, text string) error {
	s.text.WriteString(text)
	return s.controller.UpdateCard(ctx, renderFeishuStreamCard(s.opts, s.text.String()))
}

func (s *feishuOutboundStream) Finish(ctx context.Context) error {
	return s.controller.Close(ctx)
}

func (s *feishuOutboundStream) Fail(ctx context.Context, err error) error {
	message := "Stream failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message += ": " + err.Error()
	}
	_ = s.controller.UpdateCard(ctx, renderFeishuStreamCard(s.opts, message))
	return s.controller.Close(ctx)
}

func renderFeishuStreamCard(opts model.OutboundStreamOptions, text string) string {
	content := strings.TrimSpace(text)
	if content == "" {
		content = "生成中..."
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]string{"tag": "lark_md", "content": content},
			},
		},
	}
	if opts.Kind == model.OutboundStreamCron {
		title := strings.TrimSpace(opts.CronTitle)
		if title == "" {
			title = "Cron"
		}
		card["header"] = map[string]any{
			"title": map[string]string{"tag": "plain_text", "content": title},
		}
		card["elements"] = []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]string{"tag": "lark_md", "content": content},
			},
			map[string]any{"tag": "hr"},
			map[string]any{
				"tag": "note",
				"elements": []any{
					map[string]string{"tag": "plain_text", "content": "Cron reply · " + strings.TrimSpace(opts.CronID)},
				},
			},
		}
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func renderFeishuPermissionCard(prompt model.PermissionPrompt) string {
	text := strings.TrimSpace(prompt.Text)
	if text == "" {
		text = "Permission requested. Reply approve " + prompt.ShortApprovalID + " to allow or reject " + prompt.ShortApprovalID + " to deny."
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]string{"tag": "plain_text", "content": "Agent action approval"},
		},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]string{"tag": "lark_md", "content": text},
			},
			map[string]any{
				"tag": "action",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"type": "primary",
						"text": map[string]string{"tag": "plain_text", "content": "Allow"},
						"value": map[string]string{
							"short_approval_id": prompt.ShortApprovalID,
							"option":            "approve",
						},
					},
					map[string]any{
						"tag":  "button",
						"type": "danger",
						"text": map[string]string{"tag": "plain_text", "content": "Reject"},
						"value": map[string]string{
							"short_approval_id": prompt.ShortApprovalID,
							"option":            "reject",
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func renderFeishuPermissionStatusCard(shortApprovalID, option string) map[string]any {
	status := "已处理"
	if option == "approve" {
		status = "已授权"
	} else if option == "reject" {
		status = "已拒绝"
	}
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]string{"tag": "plain_text", "content": "Agent action approval"},
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]string{
					"tag":     "lark_md",
					"content": "**" + status + "**\nApproval ID: " + shortApprovalID,
				},
			},
			map[string]any{
				"tag": "hr",
			},
			map[string]any{
				"tag": "note",
				"elements": []any{
					map[string]string{"tag": "plain_text", "content": status},
				},
			},
		},
	}
}

func (a *FeishuAccount) handleFeishuSDKMessage(ctx context.Context, sdkMsg *channeltypes.NormalizedMessage) error {
	if sdkMsg == nil {
		return nil
	}
	if sdkMsg.ChatType != "p2p" {
		return a.setStatus(ctx, model.ConnectorStateConnected, "ignored non-private inbound event", string(DecisionRejectedNonPrivate))
	}
	messageID := sdkMsg.MessageID
	if messageID == "" {
		messageID = sdkMsg.EventID
	}
	timestamp := time.Now().UTC()
	if sdkMsg.CreateTimeMs > 0 {
		timestamp = time.UnixMilli(sdkMsg.CreateTimeMs).UTC()
	}
	msg := model.InboundMessage{
		AssistantID:      a.cfg.AssistantID,
		Platform:         model.PlatformFeishu,
		AccountID:        a.cfg.Channel.AccountID,
		PrivateChannelID: sdkMsg.ChatID,
		PlatformUserID:   sdkMsg.UserID,
		MessageID:        messageID,
		Text:             sdkMsg.Content,
		Timestamp:        timestamp,
	}
	if a.cfg.OnInbound != nil {
		return a.cfg.OnInbound(ctx, msg)
	}
	return nil
}

func (a *FeishuAccount) handleFeishuCardAction(ctx context.Context, event *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error) {
	if event == nil {
		return nil, nil
	}
	userID := event.Operator.OpenID
	if userID == "" {
		userID = event.Operator.UserID
	}
	chatID := event.ChatID
	if chatID == "" {
		chatID = event.Context.OpenChatID
	}
	option := stringMapValue(event.Action.Value, "option")
	if option == "" {
		option = event.Action.Name
	}
	shortApprovalID := stringMapValue(event.Action.Value, "short_approval_id")
	decision := model.PermissionDecision{
		AssistantID:      a.cfg.AssistantID,
		Platform:         model.PlatformFeishu,
		AccountID:        a.cfg.Channel.AccountID,
		PrivateChannelID: chatID,
		PlatformUserID:   userID,
		EventID:          event.EventID,
		MessageID:        event.MessageID,
		ShortApprovalID:  shortApprovalID,
		Option:           option,
	}
	if a.cfg.OnPermissionDecision == nil {
		return feishuCardActionResponse(shortApprovalID, option), nil
	}
	go func() {
		if err := a.cfg.OnPermissionDecision(context.Background(), decision); err != nil {
			_ = a.setStatus(context.Background(), model.ConnectorStateConnected, "permission card action failed", err.Error())
		}
	}()
	return feishuCardActionResponse(shortApprovalID, option), nil
}

func feishuCardActionResponse(shortApprovalID, option string) *callback.CardActionTriggerResponse {
	if shortApprovalID == "" {
		return nil
	}
	status := "已处理"
	toastType := "info"
	if option == "approve" {
		status = "已授权"
		toastType = "success"
	} else if option == "reject" {
		status = "已拒绝"
		toastType = "warning"
	}
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    toastType,
			Content: status,
		},
		Card: &callback.Card{
			Type: "raw",
			Data: renderFeishuPermissionStatusCard(shortApprovalID, option),
		},
	}
}

type QQBotAccount struct {
	BaseAccount
	tokenMu sync.Mutex
	token   string
}

func NewQQBotAccount(cfg AccountConfig) *QQBotAccount {
	cfg.Channel.Platform = model.PlatformQQBot
	return &QQBotAccount{BaseAccount: BaseAccount{cfg: cfg}}
}

func (a *QQBotAccount) Start(ctx context.Context) error {
	if !a.cfg.Channel.Enabled {
		return a.setStatus(ctx, model.ConnectorStateDisabled, "channel disabled", "")
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	if err := a.setStatus(ctx, model.ConnectorStateConnecting, "starting QQ Bot gateway", ""); err != nil {
		return err
	}
	go a.runWebSocket(runCtx, model.PlatformQQBot)
	return nil
}

func (a *QQBotAccount) Stop(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	return a.setStatus(ctx, model.ConnectorStateDisconnected, "stopped", "")
}

func (a *QQBotAccount) RefreshToken(ctx context.Context) error {
	appID := a.cfg.Secrets["app_id"]
	appSecret := a.cfg.Secrets["app_secret"]
	if appID == "" || appSecret == "" {
		return fmt.Errorf("QQ Bot app_id and app_secret are required")
	}
	token := "Bot " + appID + "." + appSecret
	a.tokenMu.Lock()
	a.token = token
	a.tokenMu.Unlock()
	return a.writeTokenCache(token)
}

func (a *QQBotAccount) Send(ctx context.Context, msg model.OutboundMessage) error {
	token := a.currentToken()
	if token == "" {
		if err := a.RefreshToken(ctx); err != nil {
			return err
		}
		token = a.currentToken()
	}
	url := defaulted(a.cfg.Secrets["send_url"], "https://api.sgroup.qq.com/v2/users/"+msg.PlatformUserID+"/messages")
	payload, _ := json.Marshal(map[string]string{"content": msg.Text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(a.cfg.HTTPClient).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		return fmt.Errorf("QQ Bot send failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func (a *BaseAccount) runWebSocket(ctx context.Context, platform model.Platform) {
	url := a.cfg.Secrets["websocket_url"]
	if strings.TrimSpace(url) == "" {
		_ = a.setStatus(ctx, model.ConnectorStateDisconnected, "websocket_url not configured; connector is ready for manual credentials but not connected", "")
		return
	}
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		header := http.Header{}
		if token := a.cfg.Secrets["gateway_token"]; token != "" {
			header.Set("Authorization", token)
		}
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, header)
		if err != nil {
			_ = a.setStatus(ctx, model.ConnectorStateFailed, "websocket dial failed", err.Error())
			sleepContext(ctx, backoff)
			backoff = minDuration(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		_ = a.setStatus(ctx, model.ConnectorStateConnected, "websocket connected", "")
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				_ = conn.Close()
				_ = a.setStatus(ctx, model.ConnectorStateDisconnected, "websocket disconnected", err.Error())
				break
			}
			var msg model.InboundMessage
			var decision Decision
			if platform == model.PlatformFeishu {
				msg, decision, err = NormalizeFeishuEvent(a.cfg.Channel.AccountID, data)
			} else {
				msg, decision, err = NormalizeQQBotEvent(a.cfg.Channel.AccountID, data)
			}
			if err != nil {
				_ = a.setStatus(ctx, model.ConnectorStateConnected, "ignored malformed inbound event", err.Error())
				continue
			}
			if decision != DecisionAccepted {
				_ = a.setStatus(ctx, model.ConnectorStateConnected, "ignored non-private inbound event", string(decision))
				continue
			}
			msg.AssistantID = a.cfg.AssistantID
			if a.cfg.OnInbound != nil {
				_ = a.cfg.OnInbound(ctx, msg)
			}
		}
		sleepContext(ctx, backoff)
		backoff = minDuration(backoff*2, 30*time.Second)
	}
}

func NormalizeFeishuEvent(accountID string, data []byte) (model.InboundMessage, Decision, error) {
	var envelope struct {
		Header struct {
			EventID string `json:"event_id"`
		} `json:"header"`
		Event struct {
			Message struct {
				MessageID   string `json:"message_id"`
				ChatID      string `json:"chat_id"`
				ChatType    string `json:"chat_type"`
				CreateTime  string `json:"create_time"`
				MessageType string `json:"message_type"`
				Content     string `json:"content"`
			} `json:"message"`
			Sender struct {
				SenderID struct {
					OpenID string `json:"open_id"`
					UserID string `json:"user_id"`
				} `json:"sender_id"`
			} `json:"sender"`
		} `json:"event"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return model.InboundMessage{}, DecisionIgnored, err
	}
	if envelope.Event.Message.ChatType != "p2p" {
		return model.InboundMessage{}, DecisionRejectedNonPrivate, nil
	}
	text := envelope.Event.Message.Content
	if envelope.Event.Message.MessageType == "text" {
		var content struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(envelope.Event.Message.Content), &content); err == nil {
			text = content.Text
		}
	}
	userID := envelope.Event.Sender.SenderID.OpenID
	if userID == "" {
		userID = envelope.Event.Sender.SenderID.UserID
	}
	messageID := envelope.Event.Message.MessageID
	if messageID == "" {
		messageID = envelope.Header.EventID
	}
	return model.InboundMessage{
		Platform:         model.PlatformFeishu,
		AccountID:        accountID,
		PrivateChannelID: envelope.Event.Message.ChatID,
		PlatformUserID:   userID,
		MessageID:        messageID,
		Text:             text,
		Timestamp:        parseFeishuMillis(envelope.Event.Message.CreateTime),
	}, DecisionAccepted, nil
}

func NormalizeQQBotEvent(accountID string, data []byte) (model.InboundMessage, Decision, error) {
	var envelope struct {
		Type string `json:"t"`
		ID   string `json:"id"`
		Data struct {
			ID        string `json:"id"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
			GuildID   string `json:"guild_id"`
			GroupID   string `json:"group_id"`
			ChannelID string `json:"channel_id"`
			Author    struct {
				ID string `json:"id"`
			} `json:"author"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return model.InboundMessage{}, DecisionIgnored, err
	}
	if envelope.Type != "C2C_MESSAGE_CREATE" || envelope.Data.GuildID != "" || envelope.Data.GroupID != "" || envelope.Data.ChannelID != "" {
		return model.InboundMessage{}, DecisionRejectedNonPrivate, nil
	}
	messageID := envelope.Data.ID
	if messageID == "" {
		messageID = envelope.ID
	}
	timestamp, _ := time.Parse(time.RFC3339, envelope.Data.Timestamp)
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return model.InboundMessage{
		Platform:         model.PlatformQQBot,
		AccountID:        accountID,
		PrivateChannelID: envelope.Data.Author.ID,
		PlatformUserID:   envelope.Data.Author.ID,
		MessageID:        messageID,
		Text:             strings.TrimSpace(envelope.Data.Content),
		Timestamp:        timestamp.UTC(),
	}, DecisionAccepted, nil
}

func (a *FeishuAccount) currentToken() string {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	return a.token
}

func (a *QQBotAccount) currentToken() string {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	return a.token
}

func (a *BaseAccount) writeTokenCache(token string) error {
	if a.cfg.Channel.TokenCachePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.cfg.Channel.TokenCachePath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(a.cfg.Channel.TokenCachePath, []byte(token), 0o600)
}

func mustJSONString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func parseFeishuMillis(raw string) time.Time {
	if raw == "" {
		return time.Now().UTC()
	}
	millis, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.UnixMilli(millis).UTC()
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func defaulted(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func stringMapValue(values map[string]interface{}, key string) string {
	value, _ := values[key].(string)
	return value
}

func feishuOpenBaseURL(domain, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimRight(override, "/")
	}
	if domain == "lark" {
		return lark.LarkBaseUrl
	}
	return lark.FeishuBaseUrl
}

func feishuOAuthBaseURL(domain, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimRight(override, "/")
	}
	if domain == "lark" {
		return lark.OAuthBaseUrlLark
	}
	return lark.OAuthBaseUrlFeishu
}

func sleepContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

type noopLarkLogger struct{}

func (noopLarkLogger) Debug(context.Context, ...interface{}) {}
func (noopLarkLogger) Info(context.Context, ...interface{})  {}
func (noopLarkLogger) Warn(context.Context, ...interface{})  {}
func (noopLarkLogger) Error(context.Context, ...interface{}) {}
