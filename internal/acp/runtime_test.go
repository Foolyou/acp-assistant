package acp_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Foolyou/acp-assistant/internal/acp"
)

func TestReadTextFileIsWorkspaceConfined(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "notes.md")
	if err := os.WriteFile(inside, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := acp.ReadTextFileConfined(context.Background(), root, "notes.md", 1024)
	if err != nil {
		t.Fatalf("read inside workspace: %v", err)
	}
	if content != "hello" {
		t.Fatalf("unexpected content: %q", content)
	}
	if _, err := acp.ReadTextFileConfined(context.Background(), root, outside, 1024); err == nil {
		t.Fatal("expected outside workspace read to be rejected")
	}
}

func TestRuntimeNewSessionIncludesMCPServers(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeNewSessionIncludesMCPServers")
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Workspace: t.TempDir(),
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()

	sessionID, err := rt.NewSession(ctx)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if sessionID != "session-1" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
}

func TestRuntimeLoadSessionIncludesMCPServers(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeLoadSessionIncludesMCPServers")
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Workspace: t.TempDir(),
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()

	sessionID, err := rt.LoadSession(ctx, "external-session")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if sessionID != "loaded-external-session" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
}

func TestRuntimePromptReturnsAgentTextChunks(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimePromptReturnsAgentTextChunks")
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Workspace: t.TempDir(),
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()

	finalText, err := rt.Prompt(ctx, "session-1", "hello")
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if finalText != "PONG" {
		t.Fatalf("unexpected final text: %q", finalText)
	}
}

func TestParseInitializeCapabilities(t *testing.T) {
	caps, err := acp.ParseInitializeCapabilities([]byte(`{
		"agentInfo": {"name": "fake"},
		"agentCapabilities": {
			"loadSession": true,
			"sessionCapabilities": {"list": {}, "resume": true, "close": false},
			"promptCapabilities": {"image": true, "audio": false, "embeddedContext": true}
		}
	}`))
	if err != nil {
		t.Fatalf("parse capabilities: %v", err)
	}
	if !caps.Session.LoadSession || !caps.Session.ListSessions || !caps.Session.ResumeSession || caps.Session.CloseSession {
		t.Fatalf("unexpected session caps: %#v", caps.Session)
	}
	if !caps.Prompt.Image || caps.Prompt.Audio || !caps.Prompt.EmbeddedContext {
		t.Fatalf("unexpected prompt caps: %#v", caps.Prompt)
	}
}

func runACPHelperProcess() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := decoder.Decode(&req); err != nil {
			return
		}
		switch req.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"agentCapabilities": map[string]any{"loadSession": true},
				},
			})
		case "session/new":
			var params map[string]json.RawMessage
			_ = json.Unmarshal(req.Params, &params)
			if _, ok := params["mcpServers"]; !ok {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error": map[string]any{
						"code":    -32602,
						"message": "missing field `mcpServers`",
					},
				})
				continue
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"sessionId": "session-1"},
			})
		case "session/load":
			var params map[string]json.RawMessage
			_ = json.Unmarshal(req.Params, &params)
			if _, ok := params["mcpServers"]; !ok {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error": map[string]any{
						"code":    -32602,
						"message": "missing field `mcpServers`",
					},
				})
				continue
			}
			var payload struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(req.Params, &payload)
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"sessionId": "loaded-" + payload.SessionID},
			})
		case "session/prompt":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"method":  "session/update",
				"params": map[string]any{
					"sessionId": "session-1",
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]any{"type": "text", "text": "PO"},
					},
				},
			})
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"method":  "session/update",
				"params": map[string]any{
					"sessionId": "session-1",
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]any{"type": "text", "text": "NG"},
					},
				},
			})
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"stopReason": "end_turn"},
			})
		}
	}
}
