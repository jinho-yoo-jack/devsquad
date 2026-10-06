package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/tools"
	"github.com/jinho-yoo-jack/devsquad/internal/workspace"
)

func TestPipelineValidation(t *testing.T) {
	cases := []string{"version: 2\nstages: [{id: a, agent: a}]", "version: 1\nstages: []", "version: 1\nstages: [{id: a, agent: a, depends_on: [b]}, {id: b, agent: b, depends_on: [a]}]", "version: 1\nstages: [{id: a, agent: a, depends_on: [missing]}]", "version: 1\nstages: [{id: a, agent: '../escape'}]", "version: 1\nstages: [{id: a, agent: a, approvals: [plan, plan]}]"}
	for _, text := range cases {
		if _, err := pipeline.ParsePipeline([]byte(text)); err == nil {
			t.Errorf("accepted invalid pipeline: %s", text)
		}
	}
	p, err := pipeline.ParsePipeline([]byte("version: 1\nstages: [{id: a, agent: a}, {id: b, agent: b, depends_on: [a]}, {id: c, agent: c, depends_on: [a]}, {id: d, agent: d, depends_on: [b,c], approvals: []}]"))
	if err != nil {
		t.Fatal(err)
	}
	levels, err := p.Levels()
	if err != nil || len(levels) != 3 || len(levels[1]) != 2 {
		t.Fatalf("levels: %v %v", levels, err)
	}
	if len(p.Stages[0].Approvals) != 2 || len(p.Stages[3].Approvals) != 0 {
		t.Fatal("default and empty approvals were conflated")
	}
}
func TestSandboxBoundaries(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	s := tools.Sandbox{Root: root, Agent: agentdef.AgentSpec{ToolProfile: "docs-writer", WritePaths: []string{"docs/spec/**"}}}
	for _, name := range []string{"../escape", "/etc/passwd", ".env", "nested/.env.prod", ".git/config", "linked/secret", ".devsquad-runtime/cache.json"} {
		if _, err = s.Read(name); err == nil {
			t.Fatalf("read allowed: %s", name)
		}
	}
	if err = s.Write("docs/spec/nested/result.md", "ok"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read("docs/spec/nested/result.md"); err != nil {
		t.Fatal(err)
	}
	if err = s.Write("server/main.go", "denied"); err == nil {
		t.Fatal("write escaped allowlist")
	}
	if _, err = s.Call(context.Background(), "run_tests", []byte(`{}`)); err == nil {
		t.Fatal("docs writer could execute command")
	}
	s.Agent.ToolProfile = "publisher"
	if err = s.Write("docs/spec/file.md", "denied"); err == nil {
		t.Fatal("publisher could write")
	}
}
func TestExampleDefinitionsAndWorkspace(t *testing.T) {
	source, err := filepath.Abs("../../examples/devsquad-templates")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ws, err := workspace.Prepare(context.Background(), root, "task-1", source)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := agentdef.LoadDefinition(ws, ".devsquad")
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Pipeline.Stages) != 6 {
		t.Fatal("example pipeline not preserved")
	}
	if _, err = workspace.Prepare(context.Background(), root, "../../outside", source); err == nil {
		t.Fatal("task traversal allowed")
	}
	if _, err = agentdef.LoadDefinition(ws, "../../outside"); err == nil {
		t.Fatal("context traversal allowed")
	}
}
