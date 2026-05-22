package model

import "time"

type HarnessProvider string

const (
	ProviderCodex  HarnessProvider = "codex"
	ProviderClaude HarnessProvider = "claude"
)

type PermissionMode string

const (
	PermissionManual   PermissionMode = "manual"
	PermissionFullAuto PermissionMode = "full_auto"
	PermissionYolo     PermissionMode = "yolo"
)

type Platform string

const (
	PlatformFeishu Platform = "feishu"
	PlatformQQBot  Platform = "qqbot"
)

type SecretType string

const (
	SecretEnv  SecretType = "env"
	SecretFile SecretType = "file"
)

type MemoryOrigin string

const (
	MemoryOriginUser    MemoryOrigin = "user"
	MemoryOriginHarness MemoryOrigin = "harness"
	MemoryOriginSystem  MemoryOrigin = "system"
)

type ConnectorState string

const (
	ConnectorStateDisabled     ConnectorState = "disabled"
	ConnectorStateDisconnected ConnectorState = "disconnected"
	ConnectorStateConnecting   ConnectorState = "connecting"
	ConnectorStateConnected    ConnectorState = "connected"
	ConnectorStateFailed       ConnectorState = "failed"
)

type EventType string

const (
	EventLifecycle       EventType = "lifecycle"
	EventConnector       EventType = "connector"
	EventSession         EventType = "session"
	EventPrompt          EventType = "prompt"
	EventACP             EventType = "acp"
	EventPermission      EventType = "permission"
	EventMemory          EventType = "memory"
	EventError           EventType = "error"
	EventInboundDecision EventType = "inbound_decision"
	EventCron            EventType = "cron"
)

type CronScheduleType string

const (
	CronScheduleTypeAt    CronScheduleType = "at"
	CronScheduleTypeEvery CronScheduleType = "every"
	CronScheduleTypeCron  CronScheduleType = "cron"
)

type CronTarget string

const (
	CronTargetIsolated CronTarget = "isolated"
	CronTargetMain     CronTarget = "main"
)

type CronDeliveryMode string

const (
	CronDeliveryOrigin CronDeliveryMode = "origin"
	CronDeliveryNone   CronDeliveryMode = "none"
)

type CronRunStatus string

const (
	CronRunStatusRunning   CronRunStatus = "running"
	CronRunStatusSucceeded CronRunStatus = "succeeded"
	CronRunStatusFailed    CronRunStatus = "failed"
	CronRunStatusTimedOut  CronRunStatus = "timed_out"
)

type HarnessBinding struct {
	Provider HarnessProvider `yaml:"provider" json:"provider"`
	Command  string          `yaml:"command,omitempty" json:"command,omitempty"`
	Args     []string        `yaml:"args,omitempty" json:"args,omitempty"`
}

type AssistantConfig struct {
	ID              string         `yaml:"id" json:"id"`
	Name            string         `yaml:"name" json:"name"`
	WorkspacePath   string         `yaml:"workspace_path" json:"workspace_path"`
	ConfigspacePath string         `yaml:"configspace_path" json:"configspace_path"`
	Harness         HarnessBinding `yaml:"harness" json:"harness"`
	Memory          MemoryConfig   `yaml:"memory" json:"memory"`
	EventDBPath     string         `yaml:"event_db_path" json:"event_db_path"`
	Autostart       bool           `yaml:"autostart" json:"autostart"`
}

type MemoryConfig struct {
	Files []string `yaml:"files" json:"files"`
}

func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{Files: []string{
		"memory/identity.md",
		"memory/preferences.md",
		"memory/facts.md",
		"memory/project.md",
	}}
}

type SecretRef struct {
	Type SecretType `yaml:"type" json:"type"`
	Name string     `yaml:"name,omitempty" json:"name,omitempty"`
	Path string     `yaml:"path,omitempty" json:"path,omitempty"`
}

type ChannelConfig struct {
	ID               string               `yaml:"id" json:"id"`
	Platform         Platform             `yaml:"platform" json:"platform"`
	AccountID        string               `yaml:"account_id" json:"account_id"`
	DisplayName      string               `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Enabled          bool                 `yaml:"enabled" json:"enabled"`
	Credentials      map[string]SecretRef `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	SetupURL         string               `yaml:"setup_url,omitempty" json:"setup_url,omitempty"`
	Options          map[string]string    `yaml:"options,omitempty" json:"options,omitempty"`
	TokenCachePath   string               `yaml:"token_cache_path,omitempty" json:"token_cache_path,omitempty"`
	RejectNonPrivate bool                 `yaml:"reject_non_private" json:"reject_non_private"`
	TextChunkLimit   int                  `yaml:"text_chunk_limit,omitempty" json:"text_chunk_limit,omitempty"`
}

type Policy struct {
	AllowedModes      []PermissionMode `yaml:"allowed_modes" json:"allowed_modes"`
	DefaultMode       PermissionMode   `yaml:"default_mode" json:"default_mode"`
	CanSetDefaultMode bool             `yaml:"can_set_default_mode" json:"can_set_default_mode"`
	Admin             bool             `yaml:"admin,omitempty" json:"admin,omitempty"`
}

type PolicySet struct {
	Assistant Policy            `yaml:"assistant" json:"assistant"`
	Accounts  map[string]Policy `yaml:"accounts,omitempty" json:"accounts,omitempty"`
	Users     map[string]Policy `yaml:"users,omitempty" json:"users,omitempty"`
}

func DefaultPolicySet() PolicySet {
	return PolicySet{
		Assistant: Policy{
			AllowedModes:      []PermissionMode{PermissionManual},
			DefaultMode:       PermissionManual,
			CanSetDefaultMode: false,
		},
		Accounts: map[string]Policy{},
		Users:    map[string]Policy{},
	}
}

type SessionBindingKey struct {
	AssistantID      string   `yaml:"assistant_id" json:"assistant_id"`
	Platform         Platform `yaml:"platform" json:"platform"`
	AccountID        string   `yaml:"account_id" json:"account_id"`
	PrivateChannelID string   `yaml:"private_channel_id" json:"private_channel_id"`
	PlatformUserID   string   `yaml:"platform_user_id" json:"platform_user_id"`
	ConversationKey  string   `yaml:"conversation_key,omitempty" json:"conversation_key,omitempty"`
	ThreadKey        string   `yaml:"thread_key,omitempty" json:"thread_key,omitempty"`
}

func (k SessionBindingKey) UserPolicyKey() string {
	return string(k.Platform) + "/" + k.AccountID + "/" + k.PlatformUserID
}

func (k SessionBindingKey) AccountPolicyKey() string {
	return string(k.Platform) + "/" + k.AccountID
}

type LocalSession struct {
	ID                string            `json:"id"`
	Binding           SessionBindingKey `json:"binding"`
	ACPSessionID      string            `json:"acp_session_id,omitempty"`
	ExternalSessionID string            `json:"external_session_id,omitempty"`
	PermissionMode    PermissionMode    `json:"permission_mode"`
	LaunchProfileKey  string            `json:"launch_profile_key"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type InboundMessage struct {
	AssistantID      string     `json:"assistant_id"`
	Platform         Platform   `json:"platform"`
	AccountID        string     `json:"account_id"`
	PrivateChannelID string     `json:"private_channel_id"`
	PlatformUserID   string     `json:"platform_user_id"`
	MessageID        string     `json:"message_id"`
	Text             string     `json:"text"`
	Timestamp        time.Time  `json:"timestamp"`
	Raw              jsonObject `json:"raw,omitempty"`
}

func (m InboundMessage) BindingKey() SessionBindingKey {
	return SessionBindingKey{
		AssistantID:      m.AssistantID,
		Platform:         m.Platform,
		AccountID:        m.AccountID,
		PrivateChannelID: m.PrivateChannelID,
		PlatformUserID:   m.PlatformUserID,
	}
}

type OutboundMessage struct {
	AssistantID      string            `json:"assistant_id"`
	Platform         Platform          `json:"platform"`
	AccountID        string            `json:"account_id"`
	PrivateChannelID string            `json:"private_channel_id"`
	PlatformUserID   string            `json:"platform_user_id"`
	Text             string            `json:"text"`
	CreatedAt        time.Time         `json:"created_at"`
	PermissionPrompt *PermissionPrompt `json:"permission_prompt,omitempty"`
}

type PermissionPrompt struct {
	ShortApprovalID string   `json:"short_approval_id"`
	Options         []string `json:"options,omitempty"`
	Text            string   `json:"text"`
}

type PermissionDecision struct {
	AssistantID      string   `json:"assistant_id"`
	Platform         Platform `json:"platform"`
	AccountID        string   `json:"account_id"`
	PrivateChannelID string   `json:"private_channel_id"`
	PlatformUserID   string   `json:"platform_user_id"`
	EventID          string   `json:"event_id,omitempty"`
	MessageID        string   `json:"message_id,omitempty"`
	ShortApprovalID  string   `json:"short_approval_id"`
	Option           string   `json:"option"`
}

type ConnectorStatus struct {
	AssistantID string         `json:"assistant_id"`
	Platform    Platform       `json:"platform"`
	AccountID   string         `json:"account_id"`
	State       ConnectorState `json:"state"`
	Message     string         `json:"message,omitempty"`
	LastError   string         `json:"last_error,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Event struct {
	ID          int64          `json:"id"`
	AssistantID string         `json:"assistant_id"`
	Type        EventType      `json:"type"`
	Scope       string         `json:"scope"`
	Message     string         `json:"message"`
	Data        map[string]any `json:"data,omitempty"`
	At          time.Time      `json:"at"`
}

type StatusSnapshot struct {
	AssistantID        string            `json:"assistant_id"`
	LastEvent          *Event            `json:"last_event,omitempty"`
	Connectors         []ConnectorStatus `json:"connectors"`
	ActiveSessions     int               `json:"active_sessions"`
	PendingPermissions int               `json:"pending_permissions"`
	RecentErrors       []Event           `json:"recent_errors"`
	MemoryRevisions    []MemoryRevision  `json:"memory_revisions"`
}

type CronJob struct {
	ID             string            `json:"id"`
	AssistantID    string            `json:"assistant_id"`
	Name           string            `json:"name"`
	Enabled        bool              `json:"enabled"`
	ScheduleType   CronScheduleType  `json:"schedule_type"`
	ScheduleExpr   string            `json:"schedule_expr"`
	Timezone       string            `json:"timezone"`
	Prompt         string            `json:"prompt"`
	Target         CronTarget        `json:"target"`
	DeliveryMode   CronDeliveryMode  `json:"delivery_mode"`
	Creator        SessionBindingKey `json:"creator"`
	PermissionMode PermissionMode    `json:"permission_mode"`
	MaxConcurrency int               `json:"max_concurrency"`
	NextRunAt      time.Time         `json:"next_run_at,omitempty"`
	LastRunAt      time.Time         `json:"last_run_at,omitempty"`
	Running        bool              `json:"running"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type CronRun struct {
	ID                string        `json:"id"`
	JobID             string        `json:"job_id"`
	AssistantID       string        `json:"assistant_id"`
	Status            CronRunStatus `json:"status"`
	Manual            bool          `json:"manual"`
	DueAt             time.Time     `json:"due_at"`
	StartedAt         time.Time     `json:"started_at"`
	FinishedAt        time.Time     `json:"finished_at,omitempty"`
	LocalSessionID    string        `json:"local_session_id,omitempty"`
	ACPSessionID      string        `json:"acp_session_id,omitempty"`
	ExternalSessionID string        `json:"external_session_id,omitempty"`
	FinalText         string        `json:"final_text,omitempty"`
	Error             string        `json:"error,omitempty"`
	Job               CronJob       `json:"job,omitempty"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Layer       string `json:"layer"`
	SourcePath  string `json:"source_path,omitempty"`
	OverlayPath string `json:"overlay_path,omitempty"`
}

type PendingPermission struct {
	ID                string            `json:"id"`
	LocalSessionID    string            `json:"local_session_id"`
	Owner             SessionBindingKey `json:"owner"`
	ACPRequestID      string            `json:"acp_request_id"`
	Options           []string          `json:"options"`
	ShortApprovalID   string            `json:"short_approval_id"`
	ResolvedOption    string            `json:"resolved_option,omitempty"`
	Status            string            `json:"status"`
	CreatedAt         time.Time         `json:"created_at"`
	ExpiresAt         time.Time         `json:"expires_at"`
	ResolvedAt        *time.Time        `json:"resolved_at,omitempty"`
	TimeoutResolution string            `json:"timeout_resolution"`
}

type MemoryUpdate struct {
	Target  string       `json:"target"`
	Content string       `json:"content"`
	Origin  MemoryOrigin `json:"origin"`
	ActorID string       `json:"actor_id"`
}

type MemoryRevision struct {
	ID          string       `json:"id"`
	AssistantID string       `json:"assistant_id"`
	Target      string       `json:"target"`
	Revision    int64        `json:"revision"`
	Origin      MemoryOrigin `json:"origin"`
	ActorID     string       `json:"actor_id"`
	ContentPath string       `json:"content_path"`
	CreatedAt   time.Time    `json:"created_at"`
}

type jsonObject map[string]any
