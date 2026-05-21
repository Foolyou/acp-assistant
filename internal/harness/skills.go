package harness

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/model"
)

func ListSkills(acpaHome, configspacePath string, provider model.HarnessProvider) ([]model.SkillInfo, error) {
	var out []model.SkillInfo
	sources := []struct {
		layer  string
		dir    string
		prefix string
	}{
		{layer: "global", dir: filepath.Join(acpaHome, "global", "skills"), prefix: "acpa-global"},
		{layer: "assistant", dir: filepath.Join(configspacePath, "skills"), prefix: "acpa-assistant"},
	}
	for _, source := range sources {
		skills, err := listSkillsInSource(source.layer, source.dir, source.prefix, configspacePath, provider)
		if err != nil {
			return nil, err
		}
		out = append(out, skills...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Layer == out[j].Layer {
			return out[i].Name < out[j].Name
		}
		return out[i].Layer < out[j].Layer
	})
	return out, nil
}

func listSkillsInSource(layer, dir, prefix, configspacePath string, provider model.HarnessProvider) ([]model.SkillInfo, error) {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.IsDir() {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return []model.SkillInfo{readSkillInfo(layer, dir, prefix, configspacePath, provider)}, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []model.SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(dir, entry.Name())
		if _, err := os.Stat(filepath.Join(sourcePath, "SKILL.md")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		out = append(out, readSkillInfo(layer, sourcePath, prefix+"-"+safeName(entry.Name()), configspacePath, provider))
	}
	return out, nil
}

func readSkillInfo(layer, sourcePath, overlayName, configspacePath string, provider model.HarnessProvider) model.SkillInfo {
	name := filepath.Base(sourcePath)
	description := ""
	data, err := os.ReadFile(filepath.Join(sourcePath, "SKILL.md"))
	if err == nil {
		metadata := parseSkillFrontMatter(string(data))
		if metadata["name"] != "" {
			name = metadata["name"]
		}
		description = metadata["description"]
	}
	return model.SkillInfo{
		Name:        name,
		Description: description,
		Layer:       layer,
		SourcePath:  sourcePath,
		OverlayPath: skillOverlayPath(configspacePath, provider, overlayName),
	}
}

func parseSkillFrontMatter(text string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return out
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out
}

func skillOverlayPath(configspacePath string, provider model.HarnessProvider, name string) string {
	switch provider {
	case model.ProviderCodex:
		return filepath.Join(configspacePath, "harness", "codex-home", "skills", name)
	case model.ProviderClaude:
		return filepath.Join(configspacePath, "harness", "claude-plugin", "skills", name)
	default:
		return ""
	}
}
