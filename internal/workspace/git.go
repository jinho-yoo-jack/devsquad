package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
)

// Git's index is shared by all stages. Serialize only Git operations; runners
// still write their disjoint paths concurrently.
type Manager struct{ mu sync.Mutex }

func git(ctx context.Context, dir string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "user.name=DevSquad", "-c", "user.email=devsquad@localhost", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_LITERAL_PATHSPECS=1", "GIT_TERMINAL_PROMPT=0")
	b, e := c.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("workspace git %s: %w: %s", args[0], e, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
func initGit(ctx context.Context, dir string) error {
	for _, args := range [][]string{{"init", "--quiet"}, {"add", "--force", "--", "."}, {"commit", "--quiet", "--allow-empty", "-m", "Workspace baseline"}} {
		if _, e := git(ctx, dir, args...); e != nil {
			return e
		}
	}
	return nil
}
func SaveDefinition(dir string, d agentdef.Definition) error {
	b, e := json.Marshal(d)
	if e != nil {
		return e
	}
	path := filepath.Join(dir, ".devsquad-runtime")
	if e = os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(path, "definition.json"), b, 0600)
}
func LoadDefinition(dir string) (agentdef.Definition, error) {
	var d agentdef.Definition
	b, e := os.ReadFile(filepath.Join(dir, ".devsquad-runtime", "definition.json"))
	if e != nil {
		return d, e
	}
	e = json.Unmarshal(b, &d)
	d.Workspace = dir
	return d, e
}
func owned(name string, patterns []string) bool {
	if pathguard.Protected(name) {
		return false
	}
	for _, p := range patterns {
		if pathguard.GlobMatch(name, p) {
			return true
		}
	}
	return false
}
func files(ctx context.Context, dir string, patterns []string) ([]string, map[string]bool, error) {
	tracked, e := git(ctx, dir, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if e != nil {
		return nil, nil, e
	}
	base := map[string]bool{}
	all := map[string]bool{}
	for _, p := range strings.Split(tracked, "\x00") {
		if p != "" && owned(p, patterns) {
			base[p] = true
			all[p] = true
		}
	}
	e = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if pathguard.Protected(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if owned(rel, patterns) {
			all[rel] = true
		}
		return nil
	})
	names := make([]string, 0, len(all))
	for p := range all {
		names = append(names, p)
	}
	slices.Sort(names)
	return names, base, e
}
func (m *Manager) Reset(ctx context.Context, dir string, patterns []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	names, base, e := files(ctx, dir, patterns)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return e
	}
	defer root.Close()
	for _, p := range names {
		if e = pathguard.NoSymlinks(root, filepath.ToSlash(filepath.Dir(p))); e != nil {
			return e
		}
		if base[p] {
			if _, e = git(ctx, dir, "restore", "--source=HEAD", "--staged", "--worktree", "--", p); e != nil {
				return e
			}
		} else {
			if e = root.Remove(p); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}
func (m *Manager) Accept(ctx context.Context, dir, key string, patterns []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ref := "refs/devsquad/approved/" + key
	if _, e := git(ctx, dir, "rev-parse", "--verify", ref); e == nil {
		return nil
	}
	names, _, e := files(ctx, dir, patterns)
	if e != nil {
		return e
	}
	if len(names) > 0 {
		if _, e = git(ctx, dir, append([]string{"add", "-A", "--force", "--"}, names...)...); e != nil {
			return e
		}
	}
	if _, e = git(ctx, dir, "commit", "--quiet", "--allow-empty", "-m", "Approve "+key); e != nil {
		return e
	}
	_, e = git(ctx, dir, "update-ref", ref, "HEAD")
	return e
}

// Fresh preparations are spared so a concurrent Create cannot lose its files.
func (m *Manager) Cleanup(root string, known map[string]bool, now time.Time) error {
	entries, e := os.ReadDir(root)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, d := range entries {
		if !d.IsDir() || known[d.Name()] {
			continue
		}
		if _, e = uuid.Parse(d.Name()); e != nil && !strings.HasPrefix(d.Name(), ".preparing-") {
			continue
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if now.Sub(info.ModTime()) < time.Hour {
			continue
		}
		if e = os.RemoveAll(filepath.Join(root, d.Name())); e != nil {
			return e
		}
	}
	return nil
}
