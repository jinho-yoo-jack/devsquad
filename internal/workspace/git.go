package workspace

import (
	"context"
	"encoding/json"
	"errors"
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

// baseRef marks the commit a workspace started from; role branches build on it.
const baseRef = "refs/devsquad/base"

var ErrNoRemote = errors.New("workspace has no remote repository")

func git(ctx context.Context, dir string, args ...string) (string, error) {
	return gitEnv(ctx, dir, nil, "", args...)
}
func gitEnv(ctx context.Context, dir string, env []string, stdin string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "user.name=DevSquad", "-c", "user.email=devsquad@localhost", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = dir
	c.Env = append(append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_LITERAL_PATHSPECS=1", "GIT_TERMINAL_PROMPT=0"), env...)
	c.Stdin = strings.NewReader(stdin)
	b, e := c.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("workspace git %s: %w: %s", args[0], e, strings.TrimSpace(string(b)))
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

// auth passes the HTTP header through the environment so the token is neither
// stored in .git/config nor visible in the process arguments.
func auth(header string) []string {
	if header == "" {
		return nil
	}
	return []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=" + header}
}
func initGit(ctx context.Context, dir string) error {
	for _, args := range [][]string{{"init", "--quiet"}, {"add", "--force", "--", "."}, {"commit", "--quiet", "--allow-empty", "-m", "Workspace baseline"}, {"update-ref", baseRef, "HEAD"}} {
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

// BranchCommit builds a commit on the workspace base holding only the changes
// from base to source inside patterns (all changes when patterns is nil). It
// reuses the source commit's dates, so the same input yields the same commit and
// a repeated push is a no-op.
func (m *Manager) BranchCommit(ctx context.Context, dir, source string, patterns []string, message string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	diff, e := git(ctx, dir, "diff", "--name-only", "-z", "--no-renames", baseRef, source)
	if e != nil {
		return "", false, e
	}
	names := []string{}
	for _, p := range strings.Split(diff, "\x00") {
		if p != "" && (patterns == nil || owned(p, patterns)) {
			names = append(names, p)
		}
	}
	if len(names) == 0 {
		return "", false, nil
	}
	listed, e := git(ctx, dir, append([]string{"ls-tree", "-z", source, "--"}, names...)...)
	if e != nil {
		return "", false, e
	}
	entries := map[string]string{}
	for _, line := range strings.Split(listed, "\x00") {
		if meta, path, ok := strings.Cut(line, "\t"); ok {
			entries[path] = meta
		}
	}
	// A zero mode removes the path from the index.
	info := strings.Builder{}
	for _, p := range names {
		if meta, ok := entries[p]; ok {
			f := strings.Fields(meta)
			fmt.Fprintf(&info, "%s %s\t%s\n", f[0], f[2], p)
		} else {
			fmt.Fprintf(&info, "0 0000000000000000000000000000000000000000\t%s\n", p)
		}
	}
	index, e := os.CreateTemp("", "devsquad-index-")
	if e != nil {
		return "", false, e
	}
	index.Close()
	defer os.Remove(index.Name())
	env := []string{"GIT_INDEX_FILE=" + index.Name()}
	if _, e = gitEnv(ctx, dir, env, "", "read-tree", baseRef); e != nil {
		return "", false, e
	}
	if _, e = gitEnv(ctx, dir, env, info.String(), "update-index", "--index-info"); e != nil {
		return "", false, e
	}
	tree, e := gitEnv(ctx, dir, env, "", "write-tree")
	if e != nil {
		return "", false, e
	}
	date, e := git(ctx, dir, "log", "-1", "--format=%cI", source)
	if e != nil {
		return "", false, e
	}
	sha, e := gitEnv(ctx, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "", "commit-tree", tree, "-p", baseRef, "-m", message)
	return sha, e == nil, e
}

// Push publishes a commit as a branch of the cloned repository.
func (m *Manager) Push(ctx context.Context, dir, sha, branch, header string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, e := git(ctx, dir, "remote", "get-url", "origin"); e != nil {
		return ErrNoRemote
	}
	_, e := gitEnv(ctx, dir, auth(header), "", "push", "--quiet", "--force", "origin", sha+":refs/heads/"+branch)
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
