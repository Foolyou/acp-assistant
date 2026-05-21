package configspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
	"gopkg.in/yaml.v3"
)

const (
	AssistantFile    = "assistant.yaml"
	InstructionsFile = "instructions.md"
	PoliciesFile     = "policies.yaml"
	EventsDBFile     = "events.db"
)

func Initialize(ctx context.Context, cfg model.AssistantConfig) error {
	if err := validateAssistant(cfg); err != nil {
		return err
	}
	if cfg.Memory.Files == nil {
		cfg.Memory = model.DefaultMemoryConfig()
	}
	if cfg.EventDBPath == "" {
		cfg.EventDBPath = filepath.Join(cfg.ConfigspacePath, EventsDBFile)
	}
	for _, dir := range []string{
		cfg.ConfigspacePath,
		filepath.Join(cfg.ConfigspacePath, "channels"),
		filepath.Join(cfg.ConfigspacePath, "secrets"),
		cfg.WorkspacePath,
		filepath.Join(cfg.WorkspacePath, "artifacts"),
		filepath.Join(cfg.WorkspacePath, "inbox"),
		filepath.Join(cfg.WorkspacePath, "memory", ".revisions"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, rel := range cfg.Memory.Files {
		path, err := workspacePath(cfg.WorkspacePath, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, []byte(memorySkeleton(rel)), 0o644); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	if err := EnsureAssistantSources(cfg); err != nil {
		return err
	}
	if err := SaveAssistant(cfg.ConfigspacePath, cfg); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(cfg.ConfigspacePath, PoliciesFile)); errors.Is(err, os.ErrNotExist) {
		if err := SavePolicies(cfg.ConfigspacePath, model.DefaultPolicySet()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	db, err := store.Open(cfg.EventDBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Migrate(ctx)
}

func InitializeGlobal(home string) error {
	if strings.TrimSpace(home) == "" {
		return fmt.Errorf("ACPA home path is required")
	}
	globalDir := filepath.Join(home, "global")
	if err := os.MkdirAll(filepath.Join(globalDir, "skills"), 0o755); err != nil {
		return err
	}
	return ensureFile(filepath.Join(globalDir, InstructionsFile), "# Global ACPA Instructions\n\n")
}

func EnsureAssistantSources(cfg model.AssistantConfig) error {
	if err := os.MkdirAll(filepath.Join(cfg.ConfigspacePath, "skills"), 0o755); err != nil {
		return err
	}
	return ensureFile(filepath.Join(cfg.ConfigspacePath, InstructionsFile), assistantInstructionsSkeleton(cfg))
}

func SaveAssistant(configDir string, cfg model.AssistantConfig) error {
	if cfg.EventDBPath == "" {
		cfg.EventDBPath = filepath.Join(configDir, EventsDBFile)
	}
	if cfg.ConfigspacePath == "" {
		cfg.ConfigspacePath = configDir
	}
	if err := validateAssistant(cfg); err != nil {
		return err
	}
	return writeYAMLAtomic(filepath.Join(configDir, AssistantFile), cfg)
}

func LoadAssistant(configDir string) (model.AssistantConfig, error) {
	var cfg model.AssistantConfig
	path := filepath.Join(configDir, AssistantFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return model.AssistantConfig{}, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return model.AssistantConfig{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return model.AssistantConfig{}, err
	}
	if _, ok := raw["autostart"]; !ok {
		cfg.Autostart = true
	}
	if cfg.Memory.Files == nil {
		cfg.Memory = model.DefaultMemoryConfig()
	}
	if cfg.EventDBPath == "" {
		cfg.EventDBPath = filepath.Join(configDir, EventsDBFile)
	}
	return cfg, validateAssistant(cfg)
}

func SavePolicies(configDir string, policies model.PolicySet) error {
	return writeYAMLAtomic(filepath.Join(configDir, PoliciesFile), policies)
}

func LoadPolicies(configDir string) (model.PolicySet, error) {
	var policies model.PolicySet
	path := filepath.Join(configDir, PoliciesFile)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return model.DefaultPolicySet(), nil
	}
	if err := readYAML(path, &policies); err != nil {
		return model.PolicySet{}, err
	}
	if policies.Assistant.AllowedModes == nil {
		policies.Assistant = model.DefaultPolicySet().Assistant
	}
	if policies.Accounts == nil {
		policies.Accounts = map[string]model.Policy{}
	}
	if policies.Users == nil {
		policies.Users = map[string]model.Policy{}
	}
	return policies, nil
}

func SaveChannel(configDir string, channel model.ChannelConfig) error {
	if strings.TrimSpace(channel.ID) == "" {
		return fmt.Errorf("channel id is required")
	}
	if channel.Platform != model.PlatformFeishu && channel.Platform != model.PlatformQQBot {
		return fmt.Errorf("unsupported platform %q", channel.Platform)
	}
	if strings.TrimSpace(channel.AccountID) == "" {
		return fmt.Errorf("channel account id is required")
	}
	if channel.TextChunkLimit == 0 {
		channel.TextChunkLimit = 3500
	}
	if !channel.RejectNonPrivate {
		channel.RejectNonPrivate = true
	}
	dir := filepath.Join(configDir, "channels")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeYAMLAtomic(filepath.Join(dir, channel.ID+".yaml"), channel)
}

func LoadChannels(configDir string) ([]model.ChannelConfig, error) {
	dir := filepath.Join(configDir, "channels")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var channels []model.ChannelConfig
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		var channel model.ChannelConfig
		if err := readYAML(filepath.Join(dir, entry.Name()), &channel); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func ResolveSecrets(refs map[string]model.SecretRef) (map[string]string, error) {
	resolved := map[string]string{}
	for key, ref := range refs {
		switch ref.Type {
		case model.SecretEnv:
			if strings.TrimSpace(ref.Name) == "" {
				return nil, fmt.Errorf("secret %s env name is required", key)
			}
			value, ok := os.LookupEnv(ref.Name)
			if !ok {
				return nil, fmt.Errorf("environment variable %s is not set", ref.Name)
			}
			resolved[key] = value
		case model.SecretFile:
			if strings.TrimSpace(ref.Path) == "" {
				return nil, fmt.Errorf("secret %s file path is required", key)
			}
			data, err := os.ReadFile(ref.Path)
			if err != nil {
				return nil, fmt.Errorf("read secret file %s: %w", ref.Path, err)
			}
			resolved[key] = strings.TrimRight(string(data), "\r\n")
		default:
			return nil, fmt.Errorf("unsupported secret reference type %q for %s", ref.Type, key)
		}
	}
	return resolved, nil
}

func EventDBPath(configDir string) string {
	cfg, err := LoadAssistant(configDir)
	if err == nil && cfg.EventDBPath != "" {
		return cfg.EventDBPath
	}
	return filepath.Join(configDir, EventsDBFile)
}

func validateAssistant(cfg model.AssistantConfig) error {
	if strings.TrimSpace(cfg.ID) == "" {
		return fmt.Errorf("assistant id is required")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("assistant name is required")
	}
	if strings.TrimSpace(cfg.WorkspacePath) == "" {
		return fmt.Errorf("workspace path is required")
	}
	if strings.TrimSpace(cfg.ConfigspacePath) == "" {
		return fmt.Errorf("configspace path is required")
	}
	if cfg.Harness.Provider != model.ProviderCodex && cfg.Harness.Provider != model.ProviderClaude {
		return fmt.Errorf("unsupported harness provider %q", cfg.Harness.Provider)
	}
	return nil
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func writeYAMLAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func ensureFile(path, content string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), 0o644)
	} else if err != nil {
		return err
	}
	return nil
}

func workspacePath(root, rel string) (string, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("invalid workspace relative path %q", rel)
	}
	full := filepath.Join(root, clean)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", rel)
	}
	return fullAbs, nil
}

func memorySkeleton(rel string) string {
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	return "# " + strings.Title(strings.ReplaceAll(name, "-", " ")) + "\n\n"
}

func assistantInstructionsSkeleton(cfg model.AssistantConfig) string {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = strings.TrimSpace(cfg.ID)
	}
	if name == "" {
		name = "Assistant"
	}
	return fmt.Sprintf("# %s Instructions\n\n", name)
}
