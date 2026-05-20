package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Foolyou/acp-assistant/internal/model"
	"github.com/Foolyou/acp-assistant/internal/store"
)

type MemoryManager struct {
	assistantID string
	root        string
	config      model.MemoryConfig
	store       *store.Store
	allowed     map[string]struct{}
}

func NewMemoryManager(assistantID, workspaceRoot string, config model.MemoryConfig, db *store.Store) *MemoryManager {
	if config.Files == nil {
		config = model.DefaultMemoryConfig()
	}
	allowed := map[string]struct{}{}
	for _, target := range config.Files {
		allowed[filepath.ToSlash(filepath.Clean(target))] = struct{}{}
	}
	return &MemoryManager{assistantID: assistantID, root: workspaceRoot, config: config, store: db, allowed: allowed}
}

func (m *MemoryManager) InitSkeletons() error {
	for _, rel := range m.config.Files {
		path, err := m.targetPath(rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(defaultSkeleton(rel)), 0o644); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	for _, dir := range []string{filepath.Join(m.root, "memory", ".revisions"), filepath.Join(m.root, "artifacts"), filepath.Join(m.root, "inbox")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryManager) Update(ctx context.Context, update model.MemoryUpdate) (model.MemoryRevision, error) {
	target := normalizeTarget(update.Target)
	if err := m.validateTarget(target); err != nil {
		return model.MemoryRevision{}, err
	}
	if update.Origin == "" {
		update.Origin = model.MemoryOriginUser
	}
	path, err := m.targetPath(target)
	if err != nil {
		return model.MemoryRevision{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return model.MemoryRevision{}, err
	}
	if err := writeFileAtomic(path, []byte(update.Content), 0o644); err != nil {
		return model.MemoryRevision{}, err
	}
	next, err := m.store.NextMemoryRevision(ctx, m.assistantID, target)
	if err != nil {
		return model.MemoryRevision{}, err
	}
	contentPath := m.revisionContentPath(target, next)
	if err := os.MkdirAll(filepath.Dir(contentPath), 0o755); err != nil {
		return model.MemoryRevision{}, err
	}
	if err := writeFileAtomic(contentPath, []byte(update.Content), 0o644); err != nil {
		return model.MemoryRevision{}, err
	}
	return m.store.RecordMemoryRevision(ctx, model.MemoryRevision{
		AssistantID: m.assistantID,
		Target:      target,
		Revision:    next,
		Origin:      update.Origin,
		ActorID:     update.ActorID,
		ContentPath: contentPath,
		CreatedAt:   time.Now().UTC(),
	})
}

func (m *MemoryManager) Rollback(ctx context.Context, target string, revision int64, actorID string) error {
	target = normalizeTarget(target)
	if err := m.validateTarget(target); err != nil {
		return err
	}
	prior, err := m.store.MemoryRevision(ctx, m.assistantID, target, revision)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(prior.ContentPath)
	if err != nil {
		return err
	}
	_, err = m.Update(ctx, model.MemoryUpdate{Target: target, Content: string(content), Origin: model.MemoryOriginUser, ActorID: actorID})
	return err
}

func (m *MemoryManager) Read(target string) (string, error) {
	target = normalizeTarget(target)
	if err := m.validateTarget(target); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(m.root, filepath.FromSlash(target)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *MemoryManager) AllowedTargets() []string {
	targets := make([]string, 0, len(m.allowed))
	for target := range m.allowed {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func (m *MemoryManager) validateTarget(target string) error {
	if _, ok := m.allowed[target]; !ok {
		return fmt.Errorf("memory target %q is not configured", target)
	}
	_, err := m.targetPath(target)
	return err
}

func (m *MemoryManager) targetPath(target string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(target))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid memory target %q", target)
	}
	rootAbs, err := filepath.Abs(m.root)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		return "", err
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("memory target %q escapes workspace", target)
	}
	return fullAbs, nil
}

func (m *MemoryManager) revisionContentPath(target string, revision int64) string {
	name := strings.ReplaceAll(filepath.ToSlash(target), "/", "__")
	return filepath.Join(m.root, "memory", ".revisions", name, fmt.Sprintf("%06d.md", revision))
}

func normalizeTarget(target string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(target))))
}

func defaultSkeleton(rel string) string {
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	title := strings.ReplaceAll(name, "-", " ")
	if title == "" {
		title = "memory"
	}
	return "# " + strings.ToUpper(title[:1]) + title[1:] + "\n\n"
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
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
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
