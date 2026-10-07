package agentdef

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type tree map[string]string

func workspace(t *testing.T, files tree) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return dir
}
func persona(front string) string { return "---\n" + front + "\n---\n# 역할\nTest member." }
func member(name, front string) tree {
	return tree{".devsquad/agents/" + name + "/persona.md": persona(front), ".devsquad/agents/" + name + "/conventions.md": "# 1. 입력\n"}
}
func with(trees ...tree) tree {
	out := tree{}
	for _, t := range trees {
		for k, v := range t {
			out[k] = v
		}
	}
	return out
}
func onePipeline(agent string) tree {
	return tree{".devsquad/pipeline.yaml": "version: 1\nstages: [{id: s, agent: " + agent + "}]\n"}
}

// 17-Agent-정의-가이드 §2: persona frontmatter + body, conventions, knowledge and spec.md are loaded.
func TestLoadDefinitionReadsMemberFiles(t *testing.T) {
	dir := workspace(t, with(onePipeline("planner"), member("planner", "name: planner\ndisplay_name: 기획자\ntool_profile: docs-writer\nwrite_paths: [\"docs/spec/**\"]\nmodel: openai/gpt-4o-mini\ncolor: \"#8B7CF6\""), tree{
		".devsquad/spec.md":                               "project context",
		".devsquad/agents/planner/knowledge/README.md":    "index",
		".devsquad/agents/planner/knowledge/db/schema.md": "tables",
		".devsquad/agents/planner/knowledge/.env":         "SECRET=1",
	}))
	d, e := LoadDefinition(dir, ".devsquad")
	if e != nil {
		t.Fatal(e)
	}
	a := d.Agents["planner"]
	if a.DisplayName != "기획자" || a.Model != "openai/gpt-4o-mini" || a.Color != "#8B7CF6" || !slices.Equal(a.WritePaths, []string{"docs/spec/**"}) {
		t.Fatalf("frontmatter: %+v", a)
	}
	if strings.Contains(a.Persona, "tool_profile") || !strings.Contains(a.Persona, "Test member.") || a.Conventions != "# 1. 입력\n" || d.ProjectSpec != "project context" {
		t.Fatalf("bodies: %+v %q", a, d.ProjectSpec)
	}
	if !strings.Contains(a.Knowledge, "### .devsquad/agents/planner/knowledge/db/schema.md\ntables") || !strings.Contains(a.Knowledge, "index") {
		t.Fatalf("knowledge: %q", a.Knowledge)
	}
	if strings.Contains(a.Knowledge, "SECRET") {
		t.Fatal("secret knowledge file injected into the prompt")
	}
}

func TestLoadDefinitionSpecIsOptionalAndPublisherIsBuiltIn(t *testing.T) {
	dir := workspace(t, onePipeline("publisher"))
	d, e := LoadDefinition(dir, ".devsquad")
	if e != nil {
		t.Fatal(e)
	}
	if a := d.Agents["publisher"]; a.ToolProfile != "publisher" || len(a.WritePaths) != 0 || d.ProjectSpec != "" {
		t.Fatalf("publisher: %+v", a)
	}
}

// §1.3: docs-writer writes only write_paths, "기본 docs/**".
func TestDocsWriterDefaultsToDocs(t *testing.T) {
	dir := workspace(t, with(onePipeline("writer"), member("writer", "name: writer\ntool_profile: docs-writer")))
	d, e := LoadDefinition(dir, ".devsquad")
	if e != nil {
		t.Fatal(e)
	}
	if got := d.Agents["writer"].WritePaths; !slices.Equal(got, []string{"docs/**"}) {
		t.Fatalf("write_paths=%v", got)
	}
}

func TestLoadDefinitionRejectsInvalidMembers(t *testing.T) {
	for name, files := range map[string]tree{
		"missing member folder": onePipeline("ghost"),
		"missing conventions":   with(onePipeline("a"), tree{".devsquad/agents/a/persona.md": persona("name: a\ntool_profile: docs-writer")}),
		"missing frontmatter":   with(onePipeline("a"), tree{".devsquad/agents/a/persona.md": "# 역할", ".devsquad/agents/a/conventions.md": "x"}),
		"name mismatch":         with(onePipeline("a"), member("a", "name: b\ntool_profile: docs-writer")),
		"unknown profile":       with(onePipeline("a"), member("a", "name: a\ntool_profile: admin")),
		"missing profile":       with(onePipeline("a"), member("a", "name: a")),
		"unknown frontmatter":   with(onePipeline("a"), member("a", "name: a\ntool_profile: docs-writer\nsudo: true")),
		"code writer no paths":  with(onePipeline("a"), member("a", "name: a\ntool_profile: code-writer")),
		"reader writes code":    with(onePipeline("a"), member("a", "name: a\ntool_profile: reader\nwrite_paths: [server/**]")),
		"traversal write path":  with(onePipeline("a"), member("a", "name: a\ntool_profile: code-writer\nwrite_paths: [../outside/**]")),
		"secret write path":     with(onePipeline("a"), member("a", "name: a\ntool_profile: code-writer\nwrite_paths: [secrets/**]")),
		"overlapping writers": with(tree{".devsquad/pipeline.yaml": "version: 1\nstages: [{id: a, agent: a}, {id: b, agent: b}]\n"},
			member("a", "name: a\ntool_profile: code-writer\nwrite_paths: [web/**]"), member("b", "name: b\ntool_profile: code-writer\nwrite_paths: [web/src/**]")),
		"writer used twice": with(tree{".devsquad/pipeline.yaml": "version: 1\nstages: [{id: a, agent: a}, {id: b, agent: a}]\n"},
			member("a", "name: a\ntool_profile: code-writer\nwrite_paths: [web/**]")),
	} {
		dir := workspace(t, files)
		if _, e := LoadDefinition(dir, ".devsquad"); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLoadDefinitionAcceptsDisjointWriters(t *testing.T) {
	dir := workspace(t, with(tree{".devsquad/pipeline.yaml": "version: 1\nstages: [{id: fe, agent: fe}, {id: be, agent: be}, {id: review, agent: review, depends_on: [fe, be]}]\n"},
		member("fe", "name: fe\ntool_profile: code-writer\nwrite_paths: [web/**, docs/api-usage/**]\ntest_command: npm test"),
		member("be", "name: be\ntool_profile: code-writer\nwrite_paths: [server/**, docs/api/**]"),
		member("review", "name: review\ntool_profile: reader\nwrite_paths: [docs/review/**]")))
	d, e := LoadDefinition(dir, ".devsquad")
	if e != nil {
		t.Fatal(e)
	}
	if d.Agents["fe"].TestCommand != "npm test" || len(d.Agents) != 3 {
		t.Fatalf("%+v", d.Agents)
	}
}

func TestLoadDefinitionRejectsSymlinks(t *testing.T) {
	outside := workspace(t, tree{"persona.md": persona("name: a\ntool_profile: docs-writer"), "notes.md": "outside"})
	for name, link := range map[string]string{"persona": ".devsquad/agents/a/persona.md", "knowledge": ".devsquad/agents/a/knowledge/notes.md"} {
		files := with(onePipeline("a"), tree{".devsquad/agents/a/conventions.md": "x"})
		if name == "knowledge" {
			files[".devsquad/agents/a/persona.md"] = persona("name: a\ntool_profile: docs-writer")
		}
		dir := workspace(t, files)
		if e := os.MkdirAll(filepath.Dir(filepath.Join(dir, link)), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.Symlink(filepath.Join(outside, filepath.Base(link)), filepath.Join(dir, link)); e != nil {
			t.Fatal(e)
		}
		if _, e := LoadDefinition(dir, ".devsquad"); e == nil {
			t.Errorf("%s symlink accepted", name)
		}
	}
	if _, e := LoadDefinition(t.TempDir(), "../outside"); e == nil {
		t.Fatal("context_path traversal accepted")
	}
}
