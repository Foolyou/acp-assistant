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

func TestRuntimeNewSessionAppliesConfiguredEffort(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "effort_config")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeNewSessionAppliesConfiguredEffort")
	rt := acp.NewRuntime(acp.Config{
		Command:     cmd.Path,
		Args:        cmd.Args[1:],
		Workspace:   t.TempDir(),
		EffortLevel: "high",
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()

	if _, err := rt.NewSession(ctx); err != nil {
		t.Fatalf("new session: %v", err)
	}
}

func TestRuntimeStartMergesConfiguredEnvironment(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "env_check")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeStartMergesConfiguredEnvironment")
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Workspace: t.TempDir(),
		Env:       map[string]string{"ACPA_RUNTIME_CUSTOM_ENV": "from-runtime"},
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()
}

func TestRuntimeStartUsesWorkspaceAsProcessDirectory(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	workspace := t.TempDir()
	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "cwd_check")
	t.Setenv("ACPA_EXPECTED_CWD", workspace)
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeStartUsesWorkspaceAsProcessDirectory")
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Env:       map[string]string{"ACPA_RUNTIME_CUSTOM_ENV": "from-runtime"},
		Workspace: workspace,
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()
}

func TestRuntimeStartUsesConfiguredProcessDirectory(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	workspace := t.TempDir()
	processDir := t.TempDir()
	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "cwd_check")
	t.Setenv("ACPA_EXPECTED_CWD", processDir)
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeStartUsesConfiguredProcessDirectory")
	rt := acp.NewRuntime(acp.Config{
		Command:    cmd.Path,
		Args:       cmd.Args[1:],
		Env:        map[string]string{"ACPA_RUNTIME_CUSTOM_ENV": "from-runtime"},
		Workspace:  workspace,
		ProcessDir: processDir,
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()
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

func TestRuntimeNewSessionIncludesManagedInstructionsMetadata(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "managed_instructions_meta")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeNewSessionIncludesManagedInstructionsMetadata")
	rt := acp.NewRuntime(acp.Config{
		Command:             cmd.Path,
		Args:                cmd.Args[1:],
		Workspace:           t.TempDir(),
		ManagedInstructions: "managed session instructions",
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()
	if _, err := rt.NewSession(ctx); err != nil {
		t.Fatalf("new session: %v", err)
	}
}

func TestRuntimePromptDoesNotSendManagedInstructionsAsPromptPrefix(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "no_prompt_prefix")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimePromptDoesNotSendManagedInstructionsAsPromptPrefix")
	rt := acp.NewRuntime(acp.Config{
		Command:             cmd.Path,
		Args:                cmd.Args[1:],
		Workspace:           t.TempDir(),
		ManagedInstructions: "managed instructions",
	})
	ctx := context.Background()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	defer rt.Stop()
	if _, err := rt.Prompt(ctx, "session-1", "hello"); err != nil {
		t.Fatalf("prompt: %v", err)
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

func TestRuntimeFlushesCollectedTextBeforePermissionRequest(t *testing.T) {
	if os.Getenv("ACPA_ACP_HELPER") == "1" {
		runACPHelperProcess()
		return
	}

	t.Setenv("ACPA_ACP_HELPER", "1")
	t.Setenv("ACPA_ACP_HELPER_SCENARIO", "permission_flush")
	cmd := exec.Command(os.Args[0], "-test.run=TestRuntimeFlushesCollectedTextBeforePermissionRequest")
	var flushed []string
	rt := acp.NewRuntime(acp.Config{
		Command:   cmd.Path,
		Args:      cmd.Args[1:],
		Workspace: t.TempDir(),
		OnPromptText: func(sessionID, text string) {
			if sessionID != "session-1" {
				t.Fatalf("unexpected flushed session id: %s", sessionID)
			}
			flushed = append(flushed, text)
		},
		OnRequest: func(req acp.Request) bool {
			if req.Method != "session/request_permission" {
				return false
			}
			_ = req.Respond(map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "approved"}})
			return true
		},
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
	if len(flushed) != 1 || flushed[0] != "BEFORE" {
		t.Fatalf("expected text before permission to flush once, got %#v", flushed)
	}
	if finalText != "AFTER" {
		t.Fatalf("expected final text to exclude flushed text, got %q", finalText)
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
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") == "cwd_check" {
				cwd, _ := os.Getwd()
				expected := os.Getenv("ACPA_EXPECTED_CWD")
				if cwd != expected || os.Getenv("PWD") != expected {
					_ = encoder.Encode(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"error":   map[string]any{"code": -32603, "message": "unexpected cwd or PWD: " + cwd + " / " + os.Getenv("PWD")},
					})
					continue
				}
			}
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") == "env_check" && os.Getenv("ACPA_RUNTIME_CUSTOM_ENV") != "from-runtime" {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error":   map[string]any{"code": -32603, "message": "missing configured env"},
				})
				continue
			}
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
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") == "managed_instructions_meta" {
				var payload struct {
					Meta struct {
						SystemPrompt struct {
							Type   string `json:"type"`
							Preset string `json:"preset"`
							Append string `json:"append"`
						} `json:"systemPrompt"`
					} `json:"_meta"`
				}
				_ = json.Unmarshal(req.Params, &payload)
				if payload.Meta.SystemPrompt.Type != "preset" || payload.Meta.SystemPrompt.Preset != "claude_code" || payload.Meta.SystemPrompt.Append != "managed session instructions" {
					_ = encoder.Encode(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"error":   map[string]any{"code": -32602, "message": "missing managed system prompt metadata"},
					})
					continue
				}
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"sessionId": "session-1"},
			})
		case "session/set_config_option":
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") != "effort_config" {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error":   map[string]any{"code": -32601, "message": "unexpected set_config_option"},
				})
				continue
			}
			var payload struct {
				SessionID string `json:"sessionId"`
				ConfigID  string `json:"configId"`
				Value     string `json:"value"`
			}
			_ = json.Unmarshal(req.Params, &payload)
			if payload.SessionID != "session-1" || payload.ConfigID != "effort" || payload.Value != "high" {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error":   map[string]any{"code": -32602, "message": "unexpected effort config"},
				})
				continue
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"configOptions": []any{}},
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
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") == "no_prompt_prefix" {
				var payload struct {
					Prompt []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"prompt"`
				}
				_ = json.Unmarshal(req.Params, &payload)
				if len(payload.Prompt) != 1 || payload.Prompt[0].Text != "hello" {
					_ = encoder.Encode(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"error":   map[string]any{"code": -32602, "message": "managed instructions leaked into prompt"},
					})
					continue
				}
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result":  map[string]any{"stopReason": "end_turn"},
				})
				continue
			}
			if os.Getenv("ACPA_ACP_HELPER_SCENARIO") == "permission_flush" {
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"method":  "session/update",
					"params": map[string]any{
						"sessionId": "session-1",
						"update": map[string]any{
							"sessionUpdate": "agent_message_chunk",
							"content":       map[string]any{"type": "text", "text": "BEFORE"},
						},
					},
				})
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      900,
					"method":  "session/request_permission",
					"params": map[string]any{
						"sessionId": "session-1",
						"options":   []map[string]any{{"id": "approved"}, {"id": "abort"}},
					},
				})
				var response map[string]json.RawMessage
				_ = decoder.Decode(&response)
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"method":  "session/update",
					"params": map[string]any{
						"sessionId": "session-1",
						"update": map[string]any{
							"sessionUpdate": "agent_message_chunk",
							"content":       map[string]any{"type": "text", "text": "AFTER"},
						},
					},
				})
				_ = encoder.Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result":  map[string]any{"stopReason": "end_turn"},
				})
				continue
			}
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
