package pipeline

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/goccy/go-yaml"
)

type StageSpec struct {
	ID        string   `json:"id" yaml:"id"`
	Agent     string   `json:"agent" yaml:"agent"`
	DependsOn []string `json:"depends_on" yaml:"depends_on"`
	Approvals []string `json:"approvals" yaml:"approvals"`
	Model     string   `json:"model,omitempty" yaml:"model"`
	Tools     []string `json:"tools" yaml:"tools"`
}
type Policy struct {
	MaxRetries    int    `json:"max_retries_per_approval" yaml:"max_retries_per_approval"`
	MaxIterations int    `json:"execute_max_iterations" yaml:"execute_max_iterations"`
	DefaultModel  string `json:"default_model" yaml:"default_model"`
	TokenBudget   *int64 `json:"token_budget,omitempty" yaml:"token_budget"`
	PR            struct {
		Mode       string `json:"mode" yaml:"mode"`
		BaseBranch string `json:"base_branch" yaml:"base_branch"`
	} `json:"pr" yaml:"pr"`
}
type Pipeline struct {
	Version int         `json:"version" yaml:"version"`
	Stages  []StageSpec `json:"stages" yaml:"stages"`
	Policy  Policy      `json:"policy" yaml:"policy"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func ParsePipeline(data []byte) (Pipeline, error) {
	p := Pipeline{Policy: Policy{MaxRetries: 3, MaxIterations: 40}}
	if err := yaml.UnmarshalWithOptions(data, &p, yaml.Strict()); err != nil {
		return p, err
	}
	if p.Version != 1 || len(p.Stages) == 0 {
		return p, fmt.Errorf("pipeline requires version: 1 and at least one stage")
	}
	if p.Policy.MaxRetries < 1 || p.Policy.MaxRetries > 10 || p.Policy.MaxIterations < 1 || p.Policy.MaxIterations > 500 {
		return p, fmt.Errorf("invalid retry or iteration limit")
	}
	if p.Policy.TokenBudget != nil && *p.Policy.TokenBudget < 1 {
		return p, fmt.Errorf("invalid token_budget")
	}
	if mode := p.Policy.PR.Mode; mode != "" && mode != "per-role" && mode != "single" {
		return p, fmt.Errorf("invalid pr mode")
	}
	seen := map[string]bool{}
	for i, s := range p.Stages {
		if !identifier.MatchString(s.ID) || s.ID == "finalize" || !identifier.MatchString(s.Agent) || seen[s.ID] {
			return p, fmt.Errorf("invalid or duplicate stage/agent: %s", s.ID)
		}
		seen[s.ID] = true
		if s.Approvals == nil {
			p.Stages[i].Approvals = []string{"plan", "deliverable"}
		}
		if s.DependsOn == nil {
			p.Stages[i].DependsOn = []string{}
		}
		approvals := map[string]bool{}
		for _, kind := range s.Approvals {
			if (kind != "plan" && kind != "deliverable") || approvals[kind] {
				return p, fmt.Errorf("invalid approvals for %s", s.ID)
			}
			approvals[kind] = true
		}
	}
	for _, s := range p.Stages {
		deps := map[string]bool{}
		for _, dep := range s.DependsOn {
			if !seen[dep] || dep == s.ID || deps[dep] {
				return p, fmt.Errorf("invalid dependency %s -> %s", s.ID, dep)
			}
			deps[dep] = true
		}
	}
	_, err := p.Levels()
	return p, err
}
func (p Pipeline) Levels() ([][]string, error) {
	done := map[string]bool{}
	out := [][]string{}
	for len(done) < len(p.Stages) {
		level := []string{}
		for _, s := range p.Stages {
			if done[s.ID] {
				continue
			}
			ready := true
			for _, dep := range s.DependsOn {
				if !done[dep] {
					ready = false
				}
			}
			if ready {
				level = append(level, s.ID)
			}
		}
		if len(level) == 0 {
			return nil, fmt.Errorf("pipeline contains a cycle")
		}
		slices.Sort(level)
		out = append(out, level)
		for _, id := range level {
			done[id] = true
		}
	}
	return out, nil
}
