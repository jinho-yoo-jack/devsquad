package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
)

func TestGlobSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		want          bool
	}{{"docs/a.md", "docs/**/*.md", true}, {"docs/a/b.md", "docs/**/*.md", true}, {"docs/a/b.md", "docs/*.md", false}, {"a.md", "**/*.md", true}, {"docs/result.md", "docs/**", true}, {"other/a.md", "docs/**", false}} {
		if got := pathguard.GlobMatch(tc.name, tc.pattern); got != tc.want {
			t.Fatalf("%s %s=%v", tc.name, tc.pattern, got)
		}
	}
}
func TestOutputRetainsHeadAndTail(t *testing.T) {
	out := &limitedOutput{}
	input := strings.Repeat("가", 10001) + strings.Repeat("나", 10001)
	data := []byte(input)
	for i := 0; i < len(data); i += 7 {
		_, _ = out.Write(data[i:min(i+7, len(data))])
	}
	want := strings.Repeat("가", 10000) + "\n[truncated]\n" + strings.Repeat("나", 10000)
	if out.String() != want {
		t.Fatalf("unexpected clipped output, %d runes", len([]rune(out.String())))
	}
}
func TestConfiguredCommandsAndTimeout(t *testing.T) {
	root, e := os.OpenRoot(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	s := Sandbox{Root: root, Agent: agentdef.AgentSpec{ToolProfile: "code-writer"}, Timeout: 100 * time.Millisecond}
	for _, command := range []string{`printf '%s' 'hello world'`, `printf hello && printf world`} {
		s.Agent.TestCommand = command
		out, e := s.Call(context.Background(), "run_tests", []byte(`{}`))
		if e != nil || !strings.HasPrefix(out, "exit=0\nhello") {
			t.Fatalf("%s: %s %v", command, out, e)
		}
	}
	s.Agent.TestCommand = `bash -c 'sleep 30 & wait'`
	start := time.Now()
	out, e := s.Call(context.Background(), "run_tests", []byte(`{}`))
	if e != nil || strings.HasPrefix(out, "exit=0") || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout failed: %s %v", out, e)
	}
}

func TestSymlinksCannotEscapeOwnershipOrSecrets(t *testing.T) {
	dir := t.TempDir()
	root, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	for _, p := range []string{"docs/a", "docs/b"} {
		if e = root.MkdirAll(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e = root.WriteFile("docs/a/real.md", []byte("inside"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = root.WriteFile(".env", []byte("secret"), 0600); e != nil {
		t.Fatal(e)
	}
	for link, target := range map[string]string{"docs/a/alias.md": "real.md", "docs/a/secret.md": "../../.env", "docs/a/other": "../b"} {
		if e = os.Symlink(target, dir+"/"+link); e != nil {
			t.Fatal(e)
		}
	}
	s := Sandbox{Root: root, Agent: agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/a/**"}}}
	if text, e := s.Read("docs/a/alias.md"); e != nil || text != "inside" {
		t.Fatalf("internal alias: %s %v", text, e)
	}
	if _, e = s.Read("docs/a/secret.md"); e == nil {
		t.Fatal("symlink exposed a secret")
	}
	if e = s.Write("docs/a/other/file.md", "bad"); e == nil {
		t.Fatal("symlink escaped stage ownership")
	}
}
