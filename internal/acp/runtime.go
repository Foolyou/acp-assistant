package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxSessionListPages = 100

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type SessionCapabilities struct {
	LoadSession   bool `json:"loadSession"`
	ListSessions  bool `json:"listSessions"`
	ResumeSession bool `json:"resumeSession"`
	CloseSession  bool `json:"closeSession"`
}

type Capabilities struct {
	AgentInfo any                 `json:"agentInfo"`
	Prompt    PromptCapabilities  `json:"prompt"`
	Session   SessionCapabilities `json:"session"`
}

type Config struct {
	Command   string
	Args      []string
	Workspace string
	OnEvent   func(Event)
	OnRequest func(Request) bool
}

type Event struct {
	Method string
	Params json.RawMessage
}

type Request struct {
	ID      string
	Method  string
	Params  json.RawMessage
	Respond func(any) error
	Reject  func(int, string) error
}

type Runtime struct {
	cfg      Config
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	pending  map[string]chan rpcResponse
	nextID   atomic.Int64
	caps     Capabilities
	mu       sync.Mutex
	readDone chan struct{}
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewRuntime(cfg Config) *Runtime {
	return &Runtime{cfg: cfg, pending: map[string]chan rpcResponse{}, readDone: make(chan struct{})}
}

func (r *Runtime) Start(ctx context.Context) error {
	if strings.TrimSpace(r.cfg.Command) == "" {
		return fmt.Errorf("ACP command is required")
	}
	r.mu.Lock()
	if r.stdin != nil {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	cmd := exec.CommandContext(ctx, r.cfg.Command, r.cfg.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	r.mu.Lock()
	r.cmd = cmd
	r.stdin = stdin
	r.mu.Unlock()
	go r.readLoop(stdout)
	go func() {
		_ = cmd.Wait()
		r.disconnect(errors.New("ACP process exited"))
	}()
	var raw json.RawMessage
	if err := r.request(ctx, "initialize", initializeParams(), &raw); err != nil {
		r.Stop()
		return err
	}
	caps, err := ParseInitializeCapabilities(raw)
	if err != nil {
		r.Stop()
		return err
	}
	r.mu.Lock()
	r.caps = caps
	r.mu.Unlock()
	return nil
}

func (r *Runtime) Stop() {
	r.mu.Lock()
	stdin := r.stdin
	cmd := r.cmd
	r.stdin = nil
	r.cmd = nil
	r.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (r *Runtime) Capabilities() Capabilities {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.caps
}

func (r *Runtime) NewSession(ctx context.Context) (string, error) {
	var result struct {
		SessionID string `json:"sessionId"`
	}
	if err := r.request(ctx, "session/new", map[string]any{"cwd": r.cfg.Workspace}, &result); err != nil {
		return "", err
	}
	return result.SessionID, nil
}

type SessionListItem struct {
	SessionID string  `json:"sessionId"`
	CWD       string  `json:"cwd"`
	Title     *string `json:"title,omitempty"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

func (r *Runtime) ListSessions(ctx context.Context) ([]SessionListItem, error) {
	if !r.Capabilities().Session.ListSessions {
		return nil, fmt.Errorf("session/list is not supported")
	}
	var sessions []SessionListItem
	var cursor *string
	seen := map[string]struct{}{}
	for page := 0; page < maxSessionListPages; page++ {
		var result struct {
			Sessions   []SessionListItem `json:"sessions"`
			NextCursor *string           `json:"nextCursor,omitempty"`
		}
		if err := r.request(ctx, "session/list", map[string]any{"cwd": r.cfg.Workspace, "cursor": cursor}, &result); err != nil {
			return nil, err
		}
		sessions = append(sessions, result.Sessions...)
		if result.NextCursor == nil || strings.TrimSpace(*result.NextCursor) == "" {
			return sessions, nil
		}
		if _, ok := seen[*result.NextCursor]; ok {
			return nil, fmt.Errorf("session/list cursor loop detected")
		}
		seen[*result.NextCursor] = struct{}{}
		cursor = result.NextCursor
	}
	return nil, fmt.Errorf("session/list exceeded %d pages", maxSessionListPages)
}

func (r *Runtime) LoadSession(ctx context.Context, sessionID string) (string, error) {
	if !r.Capabilities().Session.LoadSession {
		return "", fmt.Errorf("session/load is not supported")
	}
	var result struct {
		SessionID string `json:"sessionId"`
	}
	if err := r.request(ctx, "session/load", map[string]any{"sessionId": sessionID, "cwd": r.cfg.Workspace}, &result); err != nil {
		return "", err
	}
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	return result.SessionID, nil
}

func (r *Runtime) Prompt(ctx context.Context, sessionID, text string) error {
	var result map[string]any
	return r.request(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	}, &result)
}

func (r *Runtime) Cancel(ctx context.Context, sessionID string) error {
	var result map[string]any
	return r.request(ctx, "session/cancel", map[string]any{"sessionId": sessionID}, &result)
}

func (r *Runtime) request(ctx context.Context, method string, params any, target any) error {
	id := r.nextID.Add(1)
	key := fmt.Sprintf("%d", id)
	ch := make(chan rpcResponse, 1)
	r.mu.Lock()
	stdin := r.stdin
	r.pending[key] = ch
	r.mu.Unlock()
	if stdin == nil {
		r.removePending(key)
		return fmt.Errorf("ACP runtime is not connected")
	}
	message := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	data, _ := json.Marshal(message)
	if _, err := stdin.Write(append(data, '\n')); err != nil {
		r.removePending(key)
		return err
	}
	select {
	case response := <-ch:
		if response.Error != nil {
			return errors.New(response.Error.Message)
		}
		if target != nil {
			return json.Unmarshal(response.Result, target)
		}
		return nil
	case <-ctx.Done():
		r.removePending(key)
		return ctx.Err()
	case <-time.After(5 * time.Minute):
		r.removePending(key)
		return fmt.Errorf("%s timed out", method)
	}
}

func (r *Runtime) readLoop(stdout io.Reader) {
	defer close(r.readDone)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var message map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if rawID, ok := message["id"]; ok {
			if _, hasMethod := message["method"]; !hasMethod {
				key := idKey(rawID)
				var response rpcResponse
				if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
					continue
				}
				r.mu.Lock()
				ch := r.pending[key]
				delete(r.pending, key)
				r.mu.Unlock()
				if ch != nil {
					ch <- response
				}
				continue
			}
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			continue
		}
		r.handleIncoming(envelope.ID, envelope.Method, envelope.Params)
	}
}

func (r *Runtime) handleIncoming(id json.RawMessage, method string, params json.RawMessage) {
	switch method {
	case "fs/read_text_file":
		var req struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			_ = r.sendRawError(id, -32602, err.Error())
			return
		}
		content, err := ReadTextFileConfined(context.Background(), r.cfg.Workspace, req.Path, 2*1024*1024)
		if err != nil {
			_ = r.sendRawError(id, -32000, err.Error())
			return
		}
		_ = r.sendRawResult(id, map[string]any{"content": content})
	default:
		if r.cfg.OnRequest != nil && len(id) > 0 && r.cfg.OnRequest(Request{
			ID:      idKey(id),
			Method:  method,
			Params:  params,
			Respond: func(result any) error { return r.sendRawResult(id, result) },
			Reject:  func(code int, message string) error { return r.sendRawError(id, code, message) },
		}) {
			return
		}
		if r.cfg.OnEvent != nil {
			r.cfg.OnEvent(Event{Method: method, Params: params})
		}
		if len(id) > 0 {
			_ = r.sendRawResult(id, map[string]any{})
		}
	}
}

func (r *Runtime) sendRawResult(id json.RawMessage, result any) error {
	return r.sendRawMessage(id, "result", result)
}

func (r *Runtime) sendRawError(id json.RawMessage, code int, message string) error {
	return r.sendRawMessage(id, "error", map[string]any{"code": code, "message": message})
}

func (r *Runtime) sendRawMessage(id json.RawMessage, field string, value any) error {
	r.mu.Lock()
	stdin := r.stdin
	r.mu.Unlock()
	if stdin == nil {
		return fmt.Errorf("ACP runtime is not connected")
	}
	message := map[string]any{"jsonrpc": "2.0", field: value}
	if len(id) > 0 {
		var decoded any
		if err := json.Unmarshal(id, &decoded); err == nil {
			message["id"] = decoded
		}
	}
	data, _ := json.Marshal(message)
	_, err := stdin.Write(append(data, '\n'))
	return err
}

func (r *Runtime) removePending(key string) {
	r.mu.Lock()
	delete(r.pending, key)
	r.mu.Unlock()
}

func (r *Runtime) disconnect(err error) {
	r.mu.Lock()
	pending := r.pending
	r.pending = map[string]chan rpcResponse{}
	r.stdin = nil
	r.cmd = nil
	r.mu.Unlock()
	for _, ch := range pending {
		ch <- rpcResponse{Error: &rpcError{Code: -32000, Message: err.Error()}}
	}
}

func ParseInitializeCapabilities(raw []byte) (Capabilities, error) {
	var result struct {
		AgentInfo         any `json:"agentInfo"`
		AgentCapabilities struct {
			LoadSession         bool `json:"loadSession"`
			SessionCapabilities struct {
				List   any `json:"list"`
				Resume any `json:"resume"`
				Close  any `json:"close"`
			} `json:"sessionCapabilities"`
			PromptCapabilities PromptCapabilities `json:"promptCapabilities"`
		} `json:"agentCapabilities"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return Capabilities{}, err
	}
	return Capabilities{
		AgentInfo: result.AgentInfo,
		Prompt:    result.AgentCapabilities.PromptCapabilities,
		Session: SessionCapabilities{
			LoadSession:   result.AgentCapabilities.LoadSession,
			ListSessions:  capabilityAdvertised(result.AgentCapabilities.SessionCapabilities.List),
			ResumeSession: capabilityAdvertised(result.AgentCapabilities.SessionCapabilities.Resume),
			CloseSession:  capabilityAdvertised(result.AgentCapabilities.SessionCapabilities.Close),
		},
	}, nil
}

func ReadTextFileConfined(ctx context.Context, workspaceRoot, requested string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", err
	}
	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	full, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %s escapes workspace", requested)
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", requested)
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("%s exceeds read limit", requested)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return string(data), nil
	}
}

func initializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": "0.4.0",
		"clientInfo": map[string]any{
			"name":    "acp-assistant",
			"version": "0.1.0",
		},
		"clientCapabilities": map[string]any{
			"fs": map[string]any{"readTextFile": true},
		},
	}
}

func capabilityAdvertised(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	default:
		return true
	}
}

func idKey(raw json.RawMessage) string {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%d", n)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
