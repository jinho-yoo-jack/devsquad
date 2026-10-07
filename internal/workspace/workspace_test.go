package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
)

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func exists(dir, name string) bool { _, e := os.Lstat(filepath.Join(dir, name)); return e == nil }
func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(dir, name))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

// 19-Go-통합-서비스-설계 §9: copy local_path with the IGNORE list into its own Git baseline.
func TestPrepareCopiesSourceWithoutIgnoredFiles(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"README.md", "docs/spec/a.md", ".devsquad/pipeline.yaml", ".env", ".env.local", "secrets/token", "server.pem", ".git/config", "web/node_modules/x.js", "web/.next/cache", "server/build/out", "server/target/out", ".gradle/x", "a/__pycache__/x.pyc", ".venv/bin/python"} {
		put(t, src, name, "x")
	}
	if e := os.Symlink(filepath.Join(src, "README.md"), filepath.Join(src, "link.md")); e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	dir, e := Prepare(context.Background(), root, "task-1", src)
	if e != nil {
		t.Fatal(e)
	}
	if want, _ := filepath.EvalSymlinks(root); filepath.Dir(dir) != want || filepath.Base(dir) != "task-1" {
		t.Fatalf("workspace=%s", dir)
	}
	for _, name := range []string{"README.md", "docs/spec/a.md", ".devsquad/pipeline.yaml"} {
		if !exists(dir, name) {
			t.Errorf("missing %s", name)
		}
	}
	for _, name := range []string{".env", ".env.local", "secrets", "server.pem", "web/node_modules", "web/.next", "server/build", "server/target", ".gradle", "a/__pycache__", ".venv", "link.md"} {
		if exists(dir, name) {
			t.Errorf("copied ignored %s", name)
		}
	}
	if config := read(t, dir, ".git/config"); config == "x" {
		t.Fatal("source .git was copied")
	}
	out, e := git(context.Background(), dir, "status", "--porcelain")
	if e != nil || out != "" {
		t.Fatalf("baseline commit missing: %q %v", out, e)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("temporary preparation left behind: %v", entries)
	}
}

func TestPrepareRejectsUnsafeTargets(t *testing.T) {
	src := t.TempDir()
	put(t, src, "README.md", "x")
	root := t.TempDir()
	if _, e := Prepare(context.Background(), root, "task-1", src); e != nil {
		t.Fatal(e)
	}
	for name, args := range map[string][3]string{
		"existing workspace": {root, "task-1", src},
		"task traversal":     {root, "../escape", src},
		"nested task":        {root, "a/b", src},
		"dot task":           {root, ".", src},
		"missing local_path": {root, "task-2", ""},
		"missing source":     {root, "task-3", filepath.Join(src, "missing")},
		"root inside source": {filepath.Join(src, "workspaces"), "task-4", src},
	} {
		if _, e := Prepare(context.Background(), args[0], args[1], args[2]); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// §6.8: execute restarts restore only the stage's write_paths; approval commits them as the next baseline.
func TestResetAndAcceptOwnOnlyWritePaths(t *testing.T) {
	src := t.TempDir()
	put(t, src, "docs/a/base.md", "base")
	put(t, src, "docs/b/base.md", "other")
	dir, e := Prepare(context.Background(), t.TempDir(), "task-1", src)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	m := &Manager{}
	owned := []string{"docs/a/**"}
	put(t, dir, "docs/a/base.md", "partial edit")
	put(t, dir, "docs/a/new/partial.md", "partial")
	put(t, dir, "docs/a/.env", "secret kept")
	put(t, dir, "docs/b/base.md", "sibling edit")
	if e = m.Reset(ctx, dir, owned); e != nil {
		t.Fatal(e)
	}
	if read(t, dir, "docs/a/base.md") != "base" || exists(dir, "docs/a/new/partial.md") {
		t.Fatal("owned paths were not restored to the baseline")
	}
	if read(t, dir, "docs/b/base.md") != "sibling edit" || read(t, dir, "docs/a/.env") != "secret kept" {
		t.Fatal("reset touched files outside the stage ownership")
	}
	put(t, dir, "docs/a/result.md", "approved")
	if e = m.Accept(ctx, dir, "a", owned); e != nil {
		t.Fatal(e)
	}
	head, e := git(ctx, dir, "rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Accept(ctx, dir, "a", owned); e != nil {
		t.Fatal(e)
	}
	again, _ := git(ctx, dir, "rev-parse", "HEAD")
	ref, _ := git(ctx, dir, "rev-parse", "refs/devsquad/approved/a")
	if head != again || ref != head {
		t.Fatalf("accept is not idempotent: %s %s %s", head, again, ref)
	}
	put(t, dir, "docs/a/result.md", "rejected rewrite")
	if e = m.Reset(ctx, dir, owned); e != nil {
		t.Fatal(e)
	}
	if read(t, dir, "docs/a/result.md") != "approved" {
		t.Fatal("reset did not return to the approved checkpoint")
	}
	if changed, _ := git(ctx, dir, "status", "--porcelain", "--", "docs/b"); !strings.Contains(changed, "docs/b/base.md") {
		t.Fatal("accept committed a sibling stage's files")
	}
}

func TestCleanupRemovesOnlyOldOrphans(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	known, orphan, fresh := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, name := range []string{known, orphan, fresh, ".preparing-old", "not-a-task"} {
		if e := os.MkdirAll(filepath.Join(root, name), 0700); e != nil {
			t.Fatal(e)
		}
		if name != fresh {
			old := now.Add(-2 * time.Hour)
			if e := os.Chtimes(filepath.Join(root, name), old, old); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := (&Manager{}).Cleanup(root, map[string]bool{known: true}, now); e != nil {
		t.Fatal(e)
	}
	for name, want := range map[string]bool{known: true, orphan: false, fresh: true, ".preparing-old": false, "not-a-task": true} {
		if exists(root, name) != want {
			t.Errorf("%s exists=%v want %v", name, !want, want)
		}
	}
	if e := (&Manager{}).Cleanup(filepath.Join(root, "missing"), nil, now); e != nil {
		t.Fatal(e)
	}
}

func TestDefinitionRoundTripUsesCurrentWorkspace(t *testing.T) {
	dir := t.TempDir()
	d := agentdef.Definition{Workspace: "/old/path", ProjectSpec: "spec", Agents: map[string]agentdef.AgentSpec{"a": {Name: "a", WritePaths: []string{"docs/**"}}}}
	if e := SaveDefinition(dir, d); e != nil {
		t.Fatal(e)
	}
	got, e := LoadDefinition(dir)
	if e != nil {
		t.Fatal(e)
	}
	if got.Workspace != dir || got.ProjectSpec != "spec" || got.Agents["a"].WritePaths[0] != "docs/**" {
		t.Fatalf("%+v", got)
	}
	if info, _ := os.Stat(filepath.Join(dir, ".devsquad-runtime", "definition.json")); info.Mode().Perm() != 0600 {
		t.Fatalf("definition is not private: %v", info.Mode())
	}
}
