package agentdef

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
)

type AgentSpec struct {
	Color       string   `json:"color,omitempty" yaml:"color"`
	Name        string   `json:"name" yaml:"name"`
	DisplayName string   `json:"display_name" yaml:"display_name"`
	ToolProfile string   `json:"tool_profile" yaml:"tool_profile"`
	WritePaths  []string `json:"write_paths" yaml:"write_paths"`
	Model       string   `json:"model" yaml:"model"`
	TestCommand string   `json:"test_command" yaml:"test_command"`
	Persona     string   `json:"persona" yaml:"-"`
	Conventions string   `json:"conventions" yaml:"-"`
	Knowledge   string   `json:"knowledge" yaml:"-"`
}
type Definition struct {
	Pipeline    pipeline.Pipeline    `json:"pipeline"`
	Agents      map[string]AgentSpec `json:"agents"`
	ProjectSpec string               `json:"project_spec"`
	Workspace   string               `json:"workspace"`
}

func LoadDefinition(workspace, contextPath string) (Definition, error) {
	out := Definition{Workspace: workspace, Agents: map[string]AgentSpec{}}
	if !pathguard.SafeRelative(contextPath) {
		return out, fmt.Errorf("invalid context_path")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return out, err
	}
	defer root.Close()
	read := func(rel string) (string, error) {
		if err := pathguard.NoSymlinks(root, rel); err != nil {
			return "", err
		}
		data, err := root.ReadFile(rel)
		return string(data), err
	}
	text, err := read(filepath.Join(contextPath, "pipeline.yaml"))
	if err != nil {
		return out, err
	}
	out.Pipeline, err = pipeline.ParsePipeline([]byte(text))
	if err != nil {
		return out, err
	}
	out.ProjectSpec, err = read(filepath.Join(contextPath, "spec.md"))
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	for _, s := range out.Pipeline.Stages {
		if _, ok := out.Agents[s.Agent]; ok {
			continue
		}
		if s.Agent == "publisher" {
			out.Agents[s.Agent] = AgentSpec{Name: "publisher", ToolProfile: "publisher", Persona: "Collect approved deliverables into a PR draft.", Conventions: "Return the PR draft as Markdown. Publishing is not implemented yet."}
			continue
		}
		dir := filepath.Join(contextPath, "agents", s.Agent)
		persona, err := read(filepath.Join(dir, "persona.md"))
		if err != nil {
			return out, err
		}
		parts := strings.SplitN(persona, "---", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
			return out, fmt.Errorf("agent %s requires YAML frontmatter", s.Agent)
		}
		var a AgentSpec
		if err = yaml.UnmarshalWithOptions([]byte(parts[1]), &a, yaml.Strict()); err != nil {
			return out, err
		}
		a.Persona = parts[2]
		if a.Name != s.Agent {
			return out, fmt.Errorf("agent name must match directory: %s", s.Agent)
		}
		switch a.ToolProfile {
		case "docs-writer", "code-writer":
			if len(a.WritePaths) == 0 {
				return out, fmt.Errorf("%s requires write_paths", a.Name)
			}
		case "reader", "publisher":
			for _, wp := range a.WritePaths {
				if !strings.HasPrefix(wp, "docs/") {
					return out, fmt.Errorf("%s may only write docs", a.Name)
				}
			}
		default:
			return out, fmt.Errorf("invalid tool_profile: %s", a.ToolProfile)
		}
		for _, wp := range a.WritePaths {
			if !pathguard.SafeRelative(wp) {
				return out, fmt.Errorf("invalid write_path: %s", wp)
			}
		}
		a.Conventions, err = read(filepath.Join(dir, "conventions.md"))
		if err != nil {
			return out, err
		}
		kdir := filepath.Join(workspace, dir, "knowledge")
		err = filepath.WalkDir(kdir, func(path string, d os.DirEntry, e error) error {
			if os.IsNotExist(e) {
				return nil
			}
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("knowledge symlink is not allowed")
			}
			rel, e := filepath.Rel(workspace, path)
			if e != nil {
				return e
			}
			if pathguard.Protected(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			content, e := read(rel)
			if e != nil {
				return e
			}
			a.Knowledge += "\n### " + filepath.ToSlash(rel) + "\n" + content
			return nil
		})
		if err != nil {
			return out, err
		}
		out.Agents[a.Name] = a
	}
	if err := checkWriteConflicts(out); err != nil {
		return out, err
	}
	return out, nil
}
