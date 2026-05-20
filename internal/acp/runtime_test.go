package acp_test

import (
	"context"
	"os"
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
