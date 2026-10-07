package tools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
)

func names(specs []ToolSpec) []string {
	out := []string{}
	for _, s := range specs {
		out = append(out, s.Name)
	}
	return out
}

// 17-Agent-정의-가이드 §1.3: the profile, not the member name, decides the tools.
func TestToolsFollowProfile(t *testing.T) {
	for name, tc := range map[string]struct {
		agent   agentdef.AgentSpec
		allowed []string
		want    []string
	}{
		"docs writer":           {agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}}, nil, []string{"read_file", "list_dir", "search", "write_file"}},
		"code writer":           {agentdef.AgentSpec{ToolProfile: "code-writer", WritePaths: []string{"web/**"}, TestCommand: "npm test"}, nil, []string{"read_file", "list_dir", "search", "write_file", "run_tests"}},
		"code writer no tests":  {agentdef.AgentSpec{ToolProfile: "code-writer", WritePaths: []string{"web/**"}}, nil, []string{"read_file", "list_dir", "search", "write_file"}},
		"reader":                {agentdef.AgentSpec{ToolProfile: "reader"}, nil, []string{"read_file", "list_dir", "search"}},
		"publisher":             {agentdef.AgentSpec{ToolProfile: "publisher", WritePaths: []string{"docs/**"}}, nil, []string{"read_file", "list_dir", "search"}},
		"docs writer test cmd":  {agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}, TestCommand: "rm -rf /"}, nil, []string{"read_file", "list_dir", "search", "write_file"}},
		"stage tools allowlist": {agentdef.AgentSpec{ToolProfile: "code-writer", WritePaths: []string{"web/**"}, TestCommand: "npm test"}, []string{"read_file", "run_tests"}, []string{"read_file", "run_tests"}},
		"empty stage allowlist": {agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}}, []string{}, []string{}},
	} {
		if got := names(Sandbox{Agent: tc.agent, Allowed: tc.allowed}.Tools()); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v want %v", name, got, tc.want)
		}
	}
}

func sandbox(t *testing.T, agent agentdef.AgentSpec) (Sandbox, string) {
	t.Helper()
	dir := t.TempDir()
	root, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	return Sandbox{Root: root, Agent: agent}, dir
}

// 19-Go-통합-서비스-설계 §9: reads are capped at 200KB and writes at 2MB.
func TestFileSizeLimits(t *testing.T) {
	s, _ := sandbox(t, agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}})
	if e := s.Write("docs/big.md", strings.Repeat("a", 2_000_001)); e == nil {
		t.Fatal("write over 2MB accepted")
	}
	if e := s.Write("docs/big.md", strings.Repeat("a", 2_000_000)); e != nil {
		t.Fatal(e)
	}
	text, e := s.Read("docs/big.md")
	if e != nil || len(text) != 200_000+len("\n[truncated]") || !strings.HasSuffix(text, "\n[truncated]") {
		t.Fatalf("read cap: %d %v", len(text), e)
	}
	if e = s.Write("docs/exact.md", strings.Repeat("b", 200_000)); e != nil {
		t.Fatal(e)
	}
	if text, _ = s.Read("docs/exact.md"); strings.Contains(text, "[truncated]") {
		t.Fatal("file at the limit was marked truncated")
	}
}

func TestListAndSearchHideProtectedFiles(t *testing.T) {
	s, _ := sandbox(t, agentdef.AgentSpec{ToolProfile: "reader"})
	for name, body := range map[string]string{"docs/a.md": "needle", ".env": "needle", "secrets/key.txt": "needle", "docs/.env.local": "needle", ".git/HEAD": "needle"} {
		if e := s.Root.MkdirAll(filepath.Dir(name), 0700); e != nil {
			t.Fatal(e)
		}
		if e := s.Root.WriteFile(name, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx := context.Background()
	listed, e := s.Call(ctx, "list_dir", []byte(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	if lines := strings.Split(listed, "\n"); !slices.Equal(lines, []string{"docs/"}) {
		t.Fatalf("list_dir exposed: %q", listed)
	}
	if listed, e = s.Call(ctx, "list_dir", []byte(`{"path":"docs"}`)); e != nil || listed != "docs/a.md" {
		t.Fatalf("list_dir docs: %q %v", listed, e)
	}
	if _, e = s.Call(ctx, "list_dir", []byte(`{"path":"secrets"}`)); e == nil {
		t.Fatal("protected directory listed")
	}
	found, e := s.Call(ctx, "search", []byte(`{"pattern":"needle"}`))
	if e != nil || found != "docs/a.md:1: needle" {
		t.Fatalf("search exposed: %q %v", found, e)
	}
	if found, _ = s.Call(ctx, "search", []byte(`{"pattern":"needle","glob":"secrets/**"}`)); found != "" {
		t.Fatalf("search glob exposed: %q", found)
	}
}

func TestCallRejectsToolsOutsideProfile(t *testing.T) {
	s, _ := sandbox(t, agentdef.AgentSpec{ToolProfile: "reader"})
	for _, tool := range []string{"write_file", "run_tests", "shell", ""} {
		if _, e := s.Call(context.Background(), tool, []byte(`{"path":"docs/a.md","content":"x"}`)); e == nil {
			t.Errorf("%q allowed for reader", tool)
		}
	}
	s.Agent = agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}}
	s.Allowed = []string{"read_file"}
	if _, e := s.Call(context.Background(), "write_file", []byte(`{"path":"docs/a.md","content":"x"}`)); e == nil {
		t.Fatal("stage allowlist ignored")
	}
	if _, e := s.Call(context.Background(), "read_file", []byte(`not json`)); e == nil {
		t.Fatal("malformed arguments accepted")
	}
}
