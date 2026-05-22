package harness

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/model"
)

type builtInSkill struct {
	OverlayName string
	Content     string
}

var builtInSkills = []builtInSkill{
	{
		OverlayName: "acpa-built-in-cron",
		Content: "---\n" +
			"name: acpa-cron\n" +
			"description: Create, delete, and list ACPA scheduled reminders and recurring assistant work.\n" +
			"---\n\n" +
			"# ACPA Cron\n\n" +
			"Use this skill when the user asks to create a reminder, schedule one-time work, schedule recurring assistant work, delete a scheduled job, or list scheduled jobs.\n\n" +
			"ACPA executes cron management only when your final response contains one fenced block whose opening line is exactly three backticks followed by `acpa-cron`. Put only JSON inside the block and no user-facing prose outside it.\n\n" +
			"Create schema:\n\n" +
			"```acpa-cron\n" +
			"{\"action\":\"create\",\"name\":\"short name\",\"schedule_type\":\"at\",\"schedule_expr\":\"2099-01-02T15:04:05+08:00\",\"timezone\":\"Asia/Shanghai\",\"message\":\"self-contained prompt to run later\",\"target\":\"isolated\",\"delivery\":\"origin\"}\n" +
			"```\n\n" +
			"Delete schema:\n\n" +
			"```acpa-cron\n" +
			"{\"action\":\"delete\",\"job_id\":\"cron_xxx\"}\n" +
			"```\n\n" +
			"List schema:\n\n" +
			"```acpa-cron\n" +
			"{\"action\":\"list\"}\n" +
			"```\n\n" +
			"Rules:\n\n" +
			"- For one-time reminders, use `schedule_type` `at` and an RFC3339 `schedule_expr` with timezone offset.\n" +
			"- For fixed intervals, use `schedule_type` `every` and a Go duration such as `10m`, `2h`, or `24h`.\n" +
			"- For calendar schedules, use `schedule_type` `cron` and a five-field cron expression.\n" +
			"- Default `timezone` to `Asia/Shanghai` when the user does not specify one.\n" +
			"- Default `target` to `isolated` and `delivery` to `origin`.\n" +
			"- Make `message` self-contained because it will run in a scheduled harness session.\n" +
			"- Do not claim a reminder or schedule was set unless ACPA executes the block and returns a confirmation.\n",
	},
}

func ListSkills(acpaHome, configspacePath string, provider model.HarnessProvider) ([]model.SkillInfo, error) {
	var out []model.SkillInfo
	out = append(out, builtInSkillInfos(configspacePath, provider)...)
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

func writeBuiltInSkills(targetSkillsDir string) error {
	if err := os.MkdirAll(targetSkillsDir, 0o755); err != nil {
		return err
	}
	for _, skill := range builtInSkills {
		target := filepath.Join(targetSkillsDir, skill.OverlayName)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(skill.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func builtInSkillInfos(configspacePath string, provider model.HarnessProvider) []model.SkillInfo {
	out := make([]model.SkillInfo, 0, len(builtInSkills))
	for _, skill := range builtInSkills {
		metadata := parseSkillFrontMatter(skill.Content)
		name := metadata["name"]
		if name == "" {
			name = skill.OverlayName
		}
		out = append(out, model.SkillInfo{
			Name:        name,
			Description: metadata["description"],
			Layer:       "built-in",
			OverlayPath: skillOverlayPath(configspacePath, provider, skill.OverlayName),
		})
	}
	return out
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
