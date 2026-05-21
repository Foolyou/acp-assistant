package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Client struct {
	Home        string
	Executable  string
	HTTPClient  *http.Client
	StartDaemon func(context.Context, string) error
}

func (c Client) Status(ctx context.Context) (Status, error) {
	var status Status
	err := c.do(ctx, http.MethodGet, "/api/status", nil, &status)
	return status, err
}

func (c Client) StopDaemon(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/daemon/stop", map[string]string{}, nil)
}

func (c Client) Assistants(ctx context.Context) ([]AssistantState, error) {
	var states []AssistantState
	err := c.do(ctx, http.MethodGet, "/api/assistants", nil, &states)
	return states, err
}

func (c Client) StartAssistant(ctx context.Context, id string) (AssistantState, error) {
	return c.assistantAction(ctx, id, "start", nil)
}

func (c Client) StopAssistant(ctx context.Context, id string) (AssistantState, error) {
	return c.assistantAction(ctx, id, "stop", nil)
}

func (c Client) RestartAssistant(ctx context.Context, id string) (AssistantState, error) {
	return c.assistantAction(ctx, id, "restart", nil)
}

func (c Client) AssistantStatus(ctx context.Context, id string) (AssistantState, error) {
	var state AssistantState
	err := c.do(ctx, http.MethodGet, "/api/assistants/"+id+"/status", nil, &state)
	return state, err
}

func (c Client) SetAutostart(ctx context.Context, id string, enabled bool) (AssistantState, error) {
	return c.assistantAction(ctx, id, "autostart", map[string]bool{"enabled": enabled})
}

func (c Client) assistantAction(ctx context.Context, id, action string, body any) (AssistantState, error) {
	var state AssistantState
	err := c.do(ctx, http.MethodPost, "/api/assistants/"+id+"/"+action, body, &state)
	return state, err
}

func (c Client) EnsureRunning(ctx context.Context, bind string) (Status, error) {
	if status, err := c.Status(ctx); err == nil {
		return status, nil
	}
	start := c.StartDaemon
	if start == nil {
		start = func(ctx context.Context, bind string) error {
			cmd := exec.CommandContext(ctx, c.Executable, "daemon", "start")
			if bind != "" {
				cmd.Args = append(cmd.Args, "--bind", bind)
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	if err := start(ctx, bind); err != nil {
		return Status{}, fmt.Errorf("start daemon failed: %w; try: acpa daemon start", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := c.Status(ctx)
		if err == nil {
			status.BackgroundStart = true
			return status, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return Status{}, fmt.Errorf("daemon did not become ready; try: acpa daemon status")
}

func (c Client) do(ctx context.Context, method, path string, body any, target any) error {
	meta, err := LoadMetadata(c.Home)
	if err != nil {
		return err
	}
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, meta.Endpoint+path, &payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&apiErr)
		if apiErr.Error == "" {
			apiErr.Error = res.Status
		}
		return fmt.Errorf("%s", apiErr.Error)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(target)
}
