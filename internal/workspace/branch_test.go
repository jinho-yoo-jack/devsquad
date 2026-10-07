package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// remote creates a bare repository whose main branch holds the given files.
func remote(t *testing.T, files map[string]string) string {
	t.Helper()
	ctx := context.Background()
	src := t.TempDir()
	for name, body := range files {
		put(t, src, name, body)
	}
	bare := filepath.Join(t.TempDir(), "o", "r.git")
	for _, step := range [][]string{{"init", "--quiet", "--initial-branch=main"}, {"add", "--all"}, {"commit", "--quiet", "-m", "initial"}} {
		if _, e := git(ctx, src, step...); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := git(ctx, src, "clone", "--quiet", "--bare", ".", bare); e != nil {
		t.Fatal(e)
	}
	return bare
}
func tree(t *testing.T, dir, rev string) []string {
	t.Helper()
	out, e := git(context.Background(), dir, "ls-tree", "-r", "--name-only", "-z", rev)
	if e != nil {
		t.Fatal(e)
	}
	names := strings.Split(strings.TrimRight(out, "\x00"), "\x00")
	slices.Sort(names)
	return names
}

func TestCloneKeepsHistoryWithoutStoringCredentials(t *testing.T) {
	bare := remote(t, map[string]string{"README.md": "hello", ".devsquad/pipeline.yaml": "version: 1"})
	root := t.TempDir()
	ctx := context.Background()
	dir, e := Clone(ctx, root, "task-1", "file://"+bare, "main", "Authorization: Basic c2VjcmV0LXRva2Vu")
	if e != nil {
		t.Fatal(e)
	}
	head, _ := git(ctx, dir, "rev-parse", "HEAD")
	upstream, _ := git(ctx, bare, "rev-parse", "main")
	base, _ := git(ctx, dir, "rev-parse", baseRef)
	if head != upstream || base != head || read(t, dir, "README.md") != "hello" {
		t.Fatalf("head=%s upstream=%s base=%s", head, upstream, base)
	}
	if config := read(t, dir, ".git/config"); strings.Contains(config, "c2VjcmV0LXRva2Vu") || strings.Contains(config, "extraHeader") {
		t.Fatalf("credentials stored: %s", config)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("temporary clone left behind: %v", entries)
	}
	for name, args := range map[string][3]string{"existing": {"task-1", "file://" + bare, "main"}, "traversal": {"../x", "file://" + bare, "main"}, "missing branch": {"task-2", "file://" + bare, "release"}, "missing repo": {"task-3", "file://" + bare + "-missing", "main"}} {
		if _, e = Clone(ctx, root, args[0], args[1], args[2], ""); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Each role branch is the base plus only that stage's approved files (16-구현-로드맵 Phase 3, per-role).
func TestBranchCommitHoldsOnlyOwnedChanges(t *testing.T) {
	bare := remote(t, map[string]string{"README.md": "hello", "server/old.go": "old"})
	ctx := context.Background()
	dir, e := Clone(ctx, t.TempDir(), "task-1", "file://"+bare, "main", "")
	if e != nil {
		t.Fatal(e)
	}
	m := &Manager{}
	put(t, dir, "docs/spec/a.md", "spec")
	if e = m.Accept(ctx, dir, "planning", []string{"docs/spec/**"}); e != nil {
		t.Fatal(e)
	}
	put(t, dir, "server/main.go", "package main")
	if e = os.Remove(filepath.Join(dir, "server/old.go")); e != nil {
		t.Fatal(e)
	}
	if e = m.Accept(ctx, dir, "backend", []string{"server/**"}); e != nil {
		t.Fatal(e)
	}
	sha, changed, e := m.BranchCommit(ctx, dir, "refs/devsquad/approved/backend", []string{"server/**"}, "backend")
	if e != nil || !changed {
		t.Fatalf("%v %v", changed, e)
	}
	if got := tree(t, dir, sha); !slices.Equal(got, []string{"README.md", "server/main.go"}) {
		t.Fatalf("backend branch tree: %v", got)
	}
	parent, _ := git(ctx, dir, "rev-parse", sha+"^")
	base, _ := git(ctx, dir, "rev-parse", baseRef)
	if parent != base {
		t.Fatal("role branch must start at the cloned base")
	}
	if again, _, _ := m.BranchCommit(ctx, dir, "refs/devsquad/approved/backend", []string{"server/**"}, "backend"); again != sha {
		t.Fatalf("not deterministic: %s != %s", again, sha)
	}
	planning, _, _ := m.BranchCommit(ctx, dir, "refs/devsquad/approved/planning", []string{"docs/spec/**"}, "planning")
	if got := tree(t, dir, planning); !slices.Equal(got, []string{"README.md", "docs/spec/a.md", "server/old.go"}) {
		t.Fatalf("planning branch tree: %v", got)
	}
	all, _, _ := m.BranchCommit(ctx, dir, "HEAD", nil, "all")
	if got := tree(t, dir, all); !slices.Equal(got, []string{"README.md", "docs/spec/a.md", "server/main.go"}) {
		t.Fatalf("single branch tree: %v", got)
	}
	if _, changed, e = m.BranchCommit(ctx, dir, "refs/devsquad/approved/backend", []string{"web/**"}, "frontend"); e != nil || changed {
		t.Fatalf("unchanged role: %v %v", changed, e)
	}
}

func TestPushPublishesBranchIdempotently(t *testing.T) {
	bare := remote(t, map[string]string{"README.md": "hello"})
	ctx := context.Background()
	dir, e := Clone(ctx, t.TempDir(), "task-1", "file://"+bare, "main", "")
	if e != nil {
		t.Fatal(e)
	}
	m := &Manager{}
	put(t, dir, "docs/a.md", "a")
	if e = m.Accept(ctx, dir, "a", []string{"docs/**"}); e != nil {
		t.Fatal(e)
	}
	sha, _, e := m.BranchCommit(ctx, dir, "refs/devsquad/approved/a", []string{"docs/**"}, "a")
	if e != nil {
		t.Fatal(e)
	}
	for range 2 {
		if e = m.Push(ctx, dir, sha, "devsquad/task-1/a", ""); e != nil {
			t.Fatal(e)
		}
	}
	if pushed, _ := git(ctx, bare, "rev-parse", "refs/heads/devsquad/task-1/a"); pushed != sha {
		t.Fatalf("remote branch=%s want %s", pushed, sha)
	}
	src := t.TempDir()
	put(t, src, "README.md", "local")
	local, e := Prepare(ctx, t.TempDir(), "task-2", src)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = git(ctx, local, "rev-parse", baseRef); e != nil {
		t.Fatal("copied workspace has no base ref")
	}
	if e = m.Push(ctx, local, "HEAD", "devsquad/task-2/a", ""); !errors.Is(e, ErrNoRemote) {
		t.Fatalf("copied workspace push: %v", e)
	}
}
