package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Foolyou/acp-assistant/internal/configspace"
	"github.com/Foolyou/acp-assistant/internal/model"
)

type Supervisor struct {
	home       string
	executable string

	mu      sync.Mutex
	workers map[string]*worker
}

type worker struct {
	cfg           model.AssistantConfig
	cmd           *exec.Cmd
	running       bool
	stopping      bool
	pid           int
	lastStartedAt time.Time
	lastStoppedAt time.Time
	lastError     string
}

func NewSupervisor(home, executable string) *Supervisor {
	return &Supervisor{home: home, executable: executable, workers: map[string]*worker{}}
}

func (s *Supervisor) StartAutostart(ctx context.Context) {
	reg, err := LoadRegistry(s.home)
	if err != nil {
		return
	}
	for _, entry := range reg.Assistants {
		cfg, err := configspace.LoadAssistant(entry.ConfigspacePath)
		if err != nil || !cfg.Autostart {
			continue
		}
		_, _ = s.Start(ctx, cfg.ConfigspacePath)
	}
}

func (s *Supervisor) List(ctx context.Context) ([]AssistantState, error) {
	reg, err := LoadRegistry(s.home)
	if err != nil {
		return nil, err
	}
	states := make([]AssistantState, 0, len(reg.Assistants))
	for _, entry := range reg.Assistants {
		state := AssistantState{ID: entry.ID, Name: entry.Name, ConfigspacePath: entry.ConfigspacePath, WorkspacePath: entry.WorkspacePath}
		if cfg, err := configspace.LoadAssistant(entry.ConfigspacePath); err == nil {
			state.ID = cfg.ID
			state.Name = cfg.Name
			state.Harness = string(cfg.Harness.Provider)
			state.WorkspacePath = cfg.WorkspacePath
			state.ConfigspacePath = cfg.ConfigspacePath
			state.ChannelCount = channelCount(cfg.ConfigspacePath)
			state.Autostart = cfg.Autostart
		}
		s.mu.Lock()
		if w := s.workers[state.ID]; w != nil {
			state.Running = w.running && w.cmd != nil && w.cmd.Process != nil
			state.PID = w.pid
			state.LastStartedAt = w.lastStartedAt
			state.LastStoppedAt = w.lastStoppedAt
			state.LastError = w.lastError
		}
		s.mu.Unlock()
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ID < states[j].ID })
	_ = ctx
	return states, nil
}

func (s *Supervisor) Status(ctx context.Context, configDir string) (AssistantState, error) {
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return AssistantState{}, err
	}
	states, err := s.List(ctx)
	if err != nil {
		return AssistantState{}, err
	}
	for _, state := range states {
		if state.ID == cfg.ID {
			return state, nil
		}
	}
	return stateFromConfig(cfg), nil
}

func (s *Supervisor) Start(ctx context.Context, configDir string) (AssistantState, error) {
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return AssistantState{}, err
	}
	select {
	case <-ctx.Done():
		return AssistantState{}, ctx.Err()
	default:
	}
	s.mu.Lock()
	if w := s.workers[cfg.ID]; w != nil && w.running {
		state := w.state()
		s.mu.Unlock()
		return state, nil
	}
	s.mu.Unlock()

	outLog, err := os.OpenFile(filepath.Join(cfg.ConfigspacePath, "acpa.out.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return AssistantState{}, err
	}
	errLog, err := os.OpenFile(filepath.Join(cfg.ConfigspacePath, "acpa.err.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = outLog.Close()
		return AssistantState{}, err
	}
	cmd := exec.Command(s.executable, "assistant", "serve", "--configspace", cfg.ConfigspacePath)
	cmd.Stdout = outLog
	cmd.Stderr = errLog
	if err := cmd.Start(); err != nil {
		_ = outLog.Close()
		_ = errLog.Close()
		return AssistantState{}, err
	}
	started := time.Now().UTC()
	w := &worker{cfg: cfg, cmd: cmd, running: true, pid: cmd.Process.Pid, lastStartedAt: started}
	s.mu.Lock()
	s.workers[cfg.ID] = w
	s.mu.Unlock()
	_ = writeAssistantPID(cfg.ConfigspacePath, cmd.Process.Pid)
	go func() {
		err := cmd.Wait()
		_ = outLog.Close()
		_ = errLog.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		if current := s.workers[cfg.ID]; current == w {
			wasRunning := w.running
			w.running = false
			w.lastStoppedAt = time.Now().UTC()
			if err != nil && wasRunning && !w.stopping {
				w.lastError = err.Error()
			}
			_ = removeAssistantPIDForPID(cfg.ConfigspacePath, w.pid)
		}
	}()
	return w.state(), nil
}

func (s *Supervisor) Stop(ctx context.Context, configDir string) (AssistantState, error) {
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return AssistantState{}, err
	}
	s.mu.Lock()
	w := s.workers[cfg.ID]
	s.mu.Unlock()
	if w == nil || !w.running || w.cmd == nil || w.cmd.Process == nil {
		state := stateFromConfig(cfg)
		state.LastStoppedAt = time.Now().UTC()
		return state, nil
	}
	s.mu.Lock()
	w.stopping = true
	s.mu.Unlock()
	if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return w.state(), err
	}
	select {
	case <-ctx.Done():
		return w.state(), ctx.Err()
	case <-time.After(750 * time.Millisecond):
	}
	s.mu.Lock()
	w.running = false
	w.lastStoppedAt = time.Now().UTC()
	state := w.state()
	s.mu.Unlock()
	_ = removeAssistantPIDForPID(cfg.ConfigspacePath, w.pid)
	return state, nil
}

func (s *Supervisor) Restart(ctx context.Context, configDir string) (AssistantState, error) {
	if _, err := s.Stop(ctx, configDir); err != nil {
		return AssistantState{}, err
	}
	return s.Start(ctx, configDir)
}

func (s *Supervisor) SetAutostart(configDir string, enabled bool) (AssistantState, error) {
	cfg, err := configspace.LoadAssistant(configDir)
	if err != nil {
		return AssistantState{}, err
	}
	cfg.Autostart = enabled
	if err := configspace.SaveAssistant(cfg.ConfigspacePath, cfg); err != nil {
		return AssistantState{}, err
	}
	return s.Status(context.Background(), cfg.ConfigspacePath)
}

func (s *Supervisor) Shutdown(ctx context.Context) {
	states, _ := s.List(ctx)
	for _, state := range states {
		if state.Running {
			_, _ = s.Stop(ctx, state.ConfigspacePath)
		}
	}
}

func (w *worker) state() AssistantState {
	state := stateFromConfig(w.cfg)
	state.Running = w.running
	state.PID = w.pid
	state.LastStartedAt = w.lastStartedAt
	state.LastStoppedAt = w.lastStoppedAt
	state.LastError = w.lastError
	return state
}

func stateFromConfig(cfg model.AssistantConfig) AssistantState {
	return AssistantState{
		ID:              cfg.ID,
		Name:            cfg.Name,
		Harness:         string(cfg.Harness.Provider),
		ConfigspacePath: cfg.ConfigspacePath,
		WorkspacePath:   cfg.WorkspacePath,
		ChannelCount:    channelCount(cfg.ConfigspacePath),
		Autostart:       cfg.Autostart,
	}
}

func channelCount(configDir string) int {
	channels, err := configspace.LoadChannels(configDir)
	if err != nil {
		return 0
	}
	return len(channels)
}

func assistantPIDPath(configDir string) string {
	return filepath.Join(configDir, "assistant.pid")
}

func writeAssistantPID(configDir string, pid int) error {
	return os.WriteFile(assistantPIDPath(configDir), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func removeAssistantPIDForPID(configDir string, pid int) error {
	data, err := os.ReadFile(assistantPIDPath(configDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	current, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || current != pid {
		return nil
	}
	return os.Remove(assistantPIDPath(configDir))
}

func RunningCount(states []AssistantState) int {
	count := 0
	for _, state := range states {
		if state.Running {
			count++
		}
	}
	return count
}

func ValidateExecutable(path string) error {
	if path == "" {
		return fmt.Errorf("daemon executable is required")
	}
	return nil
}
