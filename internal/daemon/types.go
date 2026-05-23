package daemon

import (
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
)

const (
	DefaultBindAddress = "127.0.0.1:43791"
	MetadataFile       = "daemon.json"
)

type Metadata struct {
	PID      int       `json:"pid"`
	Endpoint string    `json:"endpoint"`
	Started  time.Time `json:"started"`
}

type Registry struct {
	Assistants []RegistryEntry `yaml:"assistants"`
}

type RegistryEntry struct {
	ID              string `yaml:"id" json:"id"`
	Name            string `yaml:"name" json:"name"`
	HomePath        string `yaml:"home_path,omitempty" json:"home_path,omitempty"`
	ConfigspacePath string `yaml:"configspace_path" json:"configspace_path"`
	WorkspacePath   string `yaml:"workspace_path" json:"workspace_path"`
	CreatedAt       string `yaml:"created_at" json:"created_at"`
}

type AssistantState struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Harness         string    `json:"harness,omitempty"`
	HomePath        string    `json:"home_path,omitempty"`
	ConfigspacePath string    `json:"configspace_path"`
	WorkspacePath   string    `json:"workspace_path"`
	ChannelCount    int       `json:"channel_count"`
	Autostart       bool      `json:"autostart"`
	Running         bool      `json:"running"`
	PID             int       `json:"pid,omitempty"`
	LastStartedAt   time.Time `json:"last_started_at,omitempty"`
	LastStoppedAt   time.Time `json:"last_stopped_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type Status struct {
	Reachable       bool             `json:"reachable"`
	Endpoint        string           `json:"endpoint"`
	PID             int              `json:"pid,omitempty"`
	AssistantCount  int              `json:"assistant_count"`
	RunningCount    int              `json:"running_count"`
	Assistants      []AssistantState `json:"assistants,omitempty"`
	ShutdownPolicy  string           `json:"shutdown_policy"`
	BackgroundStart bool             `json:"background_start,omitempty"`
}

type CreateAssistantRequest struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	HomePath        string                 `json:"home_path"`
	RootPath        string                 `json:"root_path"`
	WorkspacePath   string                 `json:"workspace_path"`
	ConfigspacePath string                 `json:"configspace_path"`
	Harness         model.HarnessProvider  `json:"harness"`
	Command         string                 `json:"command"`
	Args            []string               `json:"args"`
	Autostart       *bool                  `json:"autostart"`
	Memory          model.MemoryConfig     `json:"memory"`
	Options         map[string]interface{} `json:"options,omitempty"`
}

type FeishuManualRequest struct {
	AssistantID     string `json:"assistant_id"`
	ConfigspacePath string `json:"configspace_path"`
	ChannelID       string `json:"channel_id"`
	AccountID       string `json:"account_id"`
	DisplayName     string `json:"display_name"`
	Domain          string `json:"domain"`
	AppID           string `json:"app_id"`
	AppSecret       string `json:"app_secret"`
	OpenBaseURL     string `json:"open_base_url"`
	Enabled         *bool  `json:"enabled"`
}

type FeishuQRBeginRequest struct {
	AssistantID          string `json:"assistant_id"`
	ConfigspacePath      string `json:"configspace_path"`
	ChannelID            string `json:"channel_id"`
	AccountID            string `json:"account_id"`
	DisplayName          string `json:"display_name"`
	Domain               string `json:"domain"`
	RegistrationBaseURL  string `json:"registration_base_url"`
	OpenBaseURL          string `json:"open_base_url"`
	OnboardingTimeoutSec int    `json:"onboarding_timeout_sec"`
}
