package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkchannel "github.com/larksuite/oapi-sdk-go/v3/channel"
	"github.com/larksuite/oapi-sdk-go/v3/channel/normalize"
	channeltypes "github.com/larksuite/oapi-sdk-go/v3/channel/types"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type feishuSDKLongConnection struct {
	cfg        feishuLongConnectionConfig
	client     *lark.Client
	sender     channeltypes.Channel
	dispatcher *dispatcher.EventDispatcher
	httpClient *http.Client

	mu           sync.Mutex
	conn         *websocket.Conn
	serviceID    string
	pingInterval time.Duration

	messageHandlers    []func(context.Context, *channeltypes.NormalizedMessage) error
	cardActionHandlers []func(context.Context, *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error)
	rejectHandlers     []func(context.Context, *channeltypes.RejectEvent) error

	onReady        func()
	onError        func(error)
	onReconnecting func()
	onReconnected  func()
	onDisconnected func()

	messageHandlerRegistered    bool
	cardActionHandlerRegistered bool
	combineCache                map[string][][]byte
	writeMessageHook            func(int, []byte) error
}

func newFeishuSDKLongConnection(cfg feishuLongConnectionConfig) *feishuSDKLongConnection {
	openBaseURL := feishuOpenBaseURL(cfg.Domain, cfg.OpenBaseURL)
	clientOptions := []lark.ClientOptionFunc{
		lark.WithOpenBaseUrl(openBaseURL),
		lark.WithOAuthBaseUrl(feishuOAuthBaseURL(cfg.Domain, cfg.OAuthBaseURL)),
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithLogger(noopLarkLogger{}),
	}
	if cfg.HTTPClient != nil {
		clientOptions = append(clientOptions, lark.WithHttpClient(cfg.HTTPClient))
	}
	client := lark.NewClient(cfg.AppID, cfg.AppSecret, clientOptions...)
	return &feishuSDKLongConnection{
		cfg:          cfg,
		client:       client,
		sender:       larkchannel.NewChannel(client, nil),
		dispatcher:   dispatcher.NewEventDispatcher("", ""),
		httpClient:   httpClient(cfg.HTTPClient),
		pingInterval: 2 * time.Minute,
		combineCache: make(map[string][][]byte),
	}
}

func (c *feishuSDKLongConnection) Start(ctx context.Context) error {
	reconnecting := false
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if reconnecting && c.onReconnecting != nil {
			c.onReconnecting()
		}
		if err := c.connect(ctx); err != nil {
			if c.onError != nil {
				c.onError(err)
			}
			sleepContext(ctx, backoff)
			if ctx.Err() != nil {
				return nil
			}
			backoff = minDuration(backoff*2, 30*time.Second)
			reconnecting = true
			continue
		}
		backoff = time.Second
		if reconnecting {
			if c.onReconnected != nil {
				c.onReconnected()
			}
		} else if c.onReady != nil {
			c.onReady()
		}
		err := c.receiveLoop(ctx)
		c.disconnect()
		if c.onDisconnected != nil {
			c.onDisconnected()
		}
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && c.onError != nil {
			c.onError(err)
		}
		reconnecting = true
		sleepContext(ctx, backoff)
		if ctx.Err() != nil {
			return nil
		}
		backoff = minDuration(backoff*2, 30*time.Second)
	}
}

func (c *feishuSDKLongConnection) Stop(ctx context.Context) error {
	c.disconnect()
	return nil
}

func (c *feishuSDKLongConnection) Send(ctx context.Context, input *channeltypes.SendInput) (*channeltypes.SendResult, error) {
	return c.sender.Send(ctx, input)
}

func (c *feishuSDKLongConnection) OnMessage(handler func(context.Context, *channeltypes.NormalizedMessage) error) {
	c.messageHandlers = append(c.messageHandlers, handler)
	if c.messageHandlerRegistered {
		return
	}
	c.messageHandlerRegistered = true
	c.dispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		msg := normalize.ParseMessage(event)
		if msg == nil {
			return nil
		}
		for _, h := range c.messageHandlers {
			if err := h(ctx, msg); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *feishuSDKLongConnection) OnCardAction(handler func(context.Context, *channeltypes.CardActionEvent) (*callback.CardActionTriggerResponse, error)) {
	c.cardActionHandlers = append(c.cardActionHandlers, handler)
	if c.cardActionHandlerRegistered {
		return
	}
	c.cardActionHandlerRegistered = true
	c.dispatcher.OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		cardAction := normalize.ParseCardAction(event)
		if cardAction == nil {
			return nil, nil
		}
		var response *callback.CardActionTriggerResponse
		for _, h := range c.cardActionHandlers {
			resp, err := h(ctx, cardAction)
			if err != nil {
				return nil, err
			}
			if resp != nil {
				response = resp
			}
		}
		return response, nil
	})
}

func (c *feishuSDKLongConnection) OnReject(handler func(context.Context, *channeltypes.RejectEvent) error) {
	c.rejectHandlers = append(c.rejectHandlers, handler)
}

func (c *feishuSDKLongConnection) OnReady(handler func()) {
	c.onReady = handler
}

func (c *feishuSDKLongConnection) OnError(handler func(error)) {
	c.onError = handler
}

func (c *feishuSDKLongConnection) OnReconnecting(handler func()) {
	c.onReconnecting = handler
}

func (c *feishuSDKLongConnection) OnReconnected(handler func()) {
	c.onReconnected = handler
}

func (c *feishuSDKLongConnection) OnDisconnected(handler func()) {
	c.onDisconnected = handler
}

func (c *feishuSDKLongConnection) connect(ctx context.Context) error {
	connURL, err := c.getConnURL(ctx)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(connURL)
	if err != nil {
		return err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, connURL, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("Feishu websocket dial failed: status %s: %w", resp.Status, err)
		}
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.serviceID = parsed.Query().Get(larkws.ServiceID)
	c.mu.Unlock()
	go c.pingLoop(ctx)
	return nil
}

func (c *feishuSDKLongConnection) getConnURL(ctx context.Context) (string, error) {
	body, err := json.Marshal(larkws.BootstrapRequest{AppID: c.cfg.AppID, AppSecret: c.cfg.AppSecret})
	if err != nil {
		return "", err
	}
	endpointURL := strings.TrimRight(feishuOpenBaseURL(c.cfg.Domain, c.cfg.OpenBaseURL), "/") + larkws.GenEndpointUri
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("locale", "zh")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Feishu websocket endpoint failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var parsed larkws.EndpointResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != larkws.OK {
		return "", fmt.Errorf("Feishu websocket endpoint failed: code %d: %s", parsed.Code, parsed.Msg)
	}
	if parsed.Data == nil || parsed.Data.Url == "" {
		return "", fmt.Errorf("Feishu websocket endpoint returned empty URL")
	}
	if parsed.Data.ClientConfig != nil {
		if parsed.Data.ClientConfig.PingInterval > 0 {
			c.pingInterval = time.Duration(parsed.Data.ClientConfig.PingInterval) * time.Second
		}
	}
	return parsed.Data.Url, nil
}

func (c *feishuSDKLongConnection) receiveLoop(ctx context.Context) error {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return fmt.Errorf("Feishu websocket connection is closed")
		}
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if mt != websocket.BinaryMessage {
			continue
		}
		go c.handleMessage(ctx, data)
	}
}

func (c *feishuSDKLongConnection) handleMessage(ctx context.Context, data []byte) {
	var frame larkws.Frame
	if err := frame.Unmarshal(data); err != nil {
		if c.onError != nil {
			c.onError(err)
		}
		return
	}
	switch larkws.FrameType(frame.Method) {
	case larkws.FrameTypeControl:
		c.handleControlFrame(ctx, frame)
	case larkws.FrameTypeData:
		c.handleDataFrame(ctx, frame)
	}
}

func (c *feishuSDKLongConnection) handleControlFrame(ctx context.Context, frame larkws.Frame) {
	headers := larkws.Headers(frame.Headers)
	if larkws.MessageType(headers.GetString(larkws.HeaderType)) != larkws.MessageTypePong || len(frame.Payload) == 0 {
		return
	}
	var conf larkws.ClientConfig
	if err := json.Unmarshal(frame.Payload, &conf); err != nil {
		if c.onError != nil {
			c.onError(err)
		}
		return
	}
	if conf.PingInterval > 0 {
		c.pingInterval = time.Duration(conf.PingInterval) * time.Second
	}
}

func (c *feishuSDKLongConnection) handleDataFrame(ctx context.Context, frame larkws.Frame) {
	headers := larkws.Headers(frame.Headers)
	sum := headers.GetInt(larkws.HeaderSum)
	seq := headers.GetInt(larkws.HeaderSeq)
	msgID := headers.GetString(larkws.HeaderMessageID)
	msgType := larkws.MessageType(headers.GetString(larkws.HeaderType))
	payload := frame.Payload
	if sum > 1 {
		payload = c.combinePayload(msgID, sum, seq, payload)
		if payload == nil {
			return
		}
	}

	start := time.Now()
	var rsp interface{}
	var err error
	switch msgType {
	case larkws.MessageTypeEvent, larkws.MessageTypeCard:
		rsp, err = c.dispatcher.Do(ctx, payload)
	default:
		return
	}
	headers.Add(larkws.HeaderBizRt, strconv.FormatInt(time.Since(start).Milliseconds(), 10))
	resp := larkws.NewResponseByCode(http.StatusOK)
	if err != nil {
		resp = larkws.NewResponseByCode(http.StatusInternalServerError)
		if c.onError != nil {
			c.onError(err)
		}
	} else if rsp != nil {
		resp.Data, err = json.Marshal(rsp)
		if err != nil {
			resp = larkws.NewResponseByCode(http.StatusInternalServerError)
			if c.onError != nil {
				c.onError(err)
			}
		}
	}
	frame.Payload, _ = json.Marshal(resp)
	frame.Headers = headers
	data, _ := frame.Marshal()
	if err := c.writeMessage(websocket.BinaryMessage, data); err != nil && c.onError != nil {
		c.onError(err)
	}
}

func (c *feishuSDKLongConnection) combinePayload(msgID string, sum, seq int, payload []byte) []byte {
	if msgID == "" || seq < 0 || seq >= sum {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	buf := c.combineCache[msgID]
	if buf == nil {
		buf = make([][]byte, sum)
		c.combineCache[msgID] = buf
	}
	buf[seq] = payload
	size := 0
	for _, part := range buf {
		if len(part) == 0 {
			return nil
		}
		size += len(part)
	}
	delete(c.combineCache, msgID)
	combined := make([]byte, 0, size)
	for _, part := range buf {
		combined = append(combined, part...)
	}
	return combined
}

func (c *feishuSDKLongConnection) pingLoop(ctx context.Context) {
	for {
		sleepContext(ctx, c.pingInterval)
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		serviceID := c.serviceID
		c.mu.Unlock()
		id, _ := strconv.ParseInt(serviceID, 10, 32)
		frame := larkws.NewPingFrame(int32(id))
		data, _ := frame.Marshal()
		if err := c.writeMessage(websocket.BinaryMessage, data); err != nil {
			return
		}
	}
}

func (c *feishuSDKLongConnection) writeMessage(messageType int, data []byte) error {
	if c.writeMessageHook != nil {
		return c.writeMessageHook(messageType, data)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("Feishu websocket connection is closed")
	}
	return c.conn.WriteMessage(messageType, data)
}

func (c *feishuSDKLongConnection) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}
