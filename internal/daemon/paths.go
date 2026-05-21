package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"gopkg.in/yaml.v3"
)

func MetadataPath(home string) string {
	return filepath.Join(home, MetadataFile)
}

func RegistryPath(home string) string {
	return filepath.Join(home, "assistants.yaml")
}

func LoadMetadata(home string) (Metadata, error) {
	var meta Metadata
	data, err := os.ReadFile(MetadataPath(home))
	if err != nil {
		return Metadata{}, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func SaveMetadata(home string, meta Metadata) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetadataPath(home), append(data, '\n'), 0o644)
}

func RemoveMetadata(home string) error {
	err := os.Remove(MetadataPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func LoadRegistry(home string) (Registry, error) {
	data, err := os.ReadFile(RegistryPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return Registry{}, nil
	}
	if err != nil {
		return Registry{}, err
	}
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return Registry{}, err
	}
	return reg, nil
}

func SaveRegistry(home string, reg Registry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	path := RegistryPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func RegisterAssistant(home string, cfg model.AssistantConfig) error {
	reg, err := LoadRegistry(home)
	if err != nil {
		return err
	}
	entry := RegistryEntry{
		ID:              cfg.ID,
		Name:            cfg.Name,
		ConfigspacePath: cfg.ConfigspacePath,
		WorkspacePath:   cfg.WorkspacePath,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	replaced := false
	for i := range reg.Assistants {
		if reg.Assistants[i].ID == cfg.ID {
			reg.Assistants[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		reg.Assistants = append(reg.Assistants, entry)
	}
	return SaveRegistry(home, reg)
}

func ResolveConfigspace(home string, idOrPath string) (string, error) {
	value := strings.TrimSpace(idOrPath)
	if value == "" {
		return "", fmt.Errorf("assistant id or configspace path is required")
	}
	if _, err := os.Stat(filepath.Join(value, "assistant.yaml")); err == nil {
		return absPath(value), nil
	}
	reg, err := LoadRegistry(home)
	if err != nil {
		return "", err
	}
	for _, entry := range reg.Assistants {
		if entry.ID == value || entry.Name == value {
			return entry.ConfigspacePath, nil
		}
	}
	return "", fmt.Errorf("assistant %q not found", value)
}

func absPath(path string) string {
	if path == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
