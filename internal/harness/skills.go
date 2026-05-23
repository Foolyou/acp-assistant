package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Foolyou/acp-assistant/internal/model"
)

const managedAssetVersion = "1"

type builtInSkill struct {
	Name    string
	Content string
}

type ManagedSkillMarker struct {
	ManagedBy   string `json:"managed_by"`
	Provider    string `json:"provider"`
	Asset       string `json:"asset"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	ContentHash string `json:"content_hash"`
}

const builtInCronProtocol = `ACPA built-in cron protocol:

When the user asks to create a reminder, schedule one-time work, schedule recurring work, delete a scheduled job, or list scheduled jobs, you MUST use the host cron protocol instead of merely saying it is done.

Return exactly one fenced JSON block using ` + "```acpa-cron" + ` and no user-facing prose. ACPA will execute the block, then ACPA will send the confirmation or error.

Create:
` + "```acpa-cron" + `
{"action":"create","name":"short name","schedule_type":"at","schedule_expr":"2099-01-02T15:04:05+08:00","timezone":"Asia/Shanghai","message":"self-contained reminder or task prompt","target":"isolated","delivery":"origin"}
` + "```" + `

Delete:
` + "```acpa-cron" + `
{"action":"delete","job_id":"cron_xxx"}
` + "```" + `

List:
` + "```acpa-cron" + `
{"action":"list"}
` + "```" + `

Use only these schedule_type values: at, every, cron. Do not use aliases such as interval. Use RFC3339 with an explicit offset for one-time reminders; if the user gives a relative one-time reminder such as "in 10 minutes", calculate the absolute RFC3339 time. Use schedule_type every with Go durations such as 10m, 2h, or 24h for fixed intervals. Use schedule_type cron with five-field cron expressions for calendar schedules. Default timezone to Asia/Shanghai and delivery to origin. Use target isolated unless the user explicitly asks the scheduled task to continue the current conversation, in which case use target main. Always make message a self-contained prompt describing exactly what the harness should say or do when the schedule fires. Do not tell the user a reminder or schedule has been created unless you returned an acpa-cron block.`

func builtInSkillsFor(provider model.HarnessProvider) []builtInSkill {
	_ = provider
	description := "Create, delete, and list ACPA scheduled reminders and recurring assistant work."
	return []builtInSkill{
		{
			Name: "acpa-cron",
			Content: "---\n" +
				"name: acpa-cron\n" +
				"description: " + description + "\n" +
				"---\n\n" +
				"# ACPA Cron\n\n" +
				builtInCronProtocol + "\n",
		},
	}
}

func MaterializeBuiltInSkills(workspacePath string, provider model.HarnessProvider) error {
	root := ManagedSkillRoot(workspacePath, provider)
	if root == "" {
		return fmt.Errorf("unsupported harness provider %q", provider)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := ensureManagedSkillGitignore(workspacePath); err != nil {
		return err
	}
	for _, skill := range builtInSkillsFor(provider) {
		if err := materializeBuiltInSkill(root, provider, skill); err != nil {
			return err
		}
	}
	return nil
}

func ManagedSkillRoot(workspacePath string, provider model.HarnessProvider) string {
	switch provider {
	case model.ProviderCodex:
		return filepath.Join(workspacePath, ".agents", "skills")
	case model.ProviderClaude:
		return filepath.Join(workspacePath, ".claude", "skills")
	default:
		return ""
	}
}

func ExpectedManagedSkillDirs(workspacePath string, provider model.HarnessProvider) []string {
	root := ManagedSkillRoot(workspacePath, provider)
	if root == "" {
		return nil
	}
	skills := builtInSkillsFor(provider)
	out := make([]string, 0, len(skills))
	for _, skill := range skills {
		out = append(out, filepath.Join(root, skill.Name))
	}
	return out
}

func ReadManagedSkillMarker(skillDir string) (ManagedSkillMarker, error) {
	var marker ManagedSkillMarker
	data, err := os.ReadFile(filepath.Join(skillDir, ".acpa-managed.json"))
	if err != nil {
		return ManagedSkillMarker{}, err
	}
	if err := json.Unmarshal(data, &marker); err != nil {
		return ManagedSkillMarker{}, err
	}
	if marker.ManagedBy != "acpa" || marker.Asset != "skill" || strings.TrimSpace(marker.Name) == "" || strings.TrimSpace(marker.Provider) == "" {
		return ManagedSkillMarker{}, fmt.Errorf("invalid ACPA managed skill marker")
	}
	return marker, nil
}

func ListSkills(workspacePath string, provider model.HarnessProvider) ([]model.SkillInfo, error) {
	root := ManagedSkillRoot(workspacePath, provider)
	if root == "" {
		return nil, fmt.Errorf("unsupported harness provider %q", provider)
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []model.SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(sourcePath, "SKILL.md")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		out = append(out, readNativeSkillInfo(sourcePath, provider))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Layer == out[j].Layer {
			return out[i].Name < out[j].Name
		}
		return out[i].Layer < out[j].Layer
	})
	return out, nil
}

func materializeBuiltInSkill(root string, provider model.HarnessProvider, skill builtInSkill) error {
	target := filepath.Join(root, skill.Name)
	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("unowned ACPA managed skill collision at %s: path is not a directory", target)
		}
		marker, err := ReadManagedSkillMarker(target)
		if err != nil {
			return fmt.Errorf("unowned ACPA managed skill collision at %s: %w", target, err)
		}
		if marker.Name != skill.Name || marker.Provider != string(provider) {
			return fmt.Errorf("unowned ACPA managed skill collision at %s: marker belongs to %s/%s", target, marker.Provider, marker.Name)
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(skill.Content), 0o644); err != nil {
		return err
	}
	marker := ManagedSkillMarker{
		ManagedBy:   "acpa",
		Provider:    string(provider),
		Asset:       "skill",
		Name:        skill.Name,
		Version:     managedAssetVersion,
		ContentHash: contentHash(skill.Content),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, ".acpa-managed.json"), append(data, '\n'), 0o644)
}

func ensureManagedSkillGitignore(workspacePath string) error {
	if strings.TrimSpace(workspacePath) == "" {
		return fmt.Errorf("workspace path is required")
	}
	path := filepath.Join(workspacePath, ".gitignore")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = nil
	} else if err != nil {
		return err
	}
	text := string(data)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	has := map[string]bool{}
	for _, line := range lines {
		has[strings.TrimSpace(line)] = true
	}
	rules := []string{".agents/skills/acpa-*/", ".claude/skills/acpa-*/"}
	var changed bool
	for _, rule := range rules {
		if !has[rule] {
			if text != "" && !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			text += rule + "\n"
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func readNativeSkillInfo(sourcePath string, provider model.HarnessProvider) model.SkillInfo {
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
	layer := "workspace"
	if marker, err := ReadManagedSkillMarker(sourcePath); err == nil && marker.Provider == string(provider) {
		layer = "built-in"
	}
	return model.SkillInfo{
		Name:        name,
		Description: description,
		Layer:       layer,
		SourcePath:  sourcePath,
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

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}
