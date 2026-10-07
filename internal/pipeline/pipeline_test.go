package pipeline

import (
	"slices"
	"testing"
)

// 15-API-이벤트-명세 §6: defaults for omitted fields.
func TestParseDefaults(t *testing.T) {
	p, e := ParsePipeline([]byte("version: 1\nstages:\n- {id: plan, agent: planner}\n- {id: build, agent: backend, depends_on: [plan], approvals: []}\n"))
	if e != nil {
		t.Fatal(e)
	}
	if p.Policy.MaxRetries != 3 || p.Policy.MaxIterations != 40 || p.Policy.TokenBudget != nil {
		t.Fatalf("policy defaults: %+v", p.Policy)
	}
	if !slices.Equal(p.Stages[0].Approvals, []string{"plan", "deliverable"}) || p.Stages[0].DependsOn == nil || len(p.Stages[0].DependsOn) != 0 {
		t.Fatalf("stage defaults: %+v", p.Stages[0])
	}
	if p.Stages[1].Approvals == nil || len(p.Stages[1].Approvals) != 0 {
		t.Fatalf("explicit empty approvals must disable gates: %+v", p.Stages[1])
	}
}

func TestParseExplicitPolicy(t *testing.T) {
	p, e := ParsePipeline([]byte("version: 1\nstages: [{id: a, agent: a, approvals: [deliverable], tools: [read_file], model: openai/gpt-4o-mini}]\npolicy: {max_retries_per_approval: 5, execute_max_iterations: 7, token_budget: 1000, default_model: fake, pr: {mode: single, base_branch: main}}\n"))
	if e != nil {
		t.Fatal(e)
	}
	if p.Policy.MaxRetries != 5 || p.Policy.MaxIterations != 7 || p.Policy.TokenBudget == nil || *p.Policy.TokenBudget != 1000 || p.Policy.PR.Mode != "single" {
		t.Fatalf("policy: %+v", p.Policy)
	}
	if s := p.Stages[0]; !slices.Equal(s.Approvals, []string{"deliverable"}) || !slices.Equal(s.Tools, []string{"read_file"}) || s.Model != "openai/gpt-4o-mini" {
		t.Fatalf("stage: %+v", s)
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	for name, text := range map[string]string{
		"missing version":    "stages: [{id: a, agent: a}]",
		"unknown field":      "version: 1\nstages: [{id: a, agent: a, extra: 1}]",
		"bad id":             "version: 1\nstages: [{id: A-1, agent: a}]",
		"reserved id":        "version: 1\nstages: [{id: finalize, agent: a}]",
		"duplicate id":       "version: 1\nstages: [{id: a, agent: a}, {id: a, agent: b}]",
		"self dependency":    "version: 1\nstages: [{id: a, agent: a, depends_on: [a]}]",
		"repeat dependency":  "version: 1\nstages: [{id: a, agent: a}, {id: b, agent: b, depends_on: [a, a]}]",
		"three stage cycle":  "version: 1\nstages: [{id: a, agent: a, depends_on: [c]}, {id: b, agent: b, depends_on: [a]}, {id: c, agent: c, depends_on: [b]}]",
		"unknown approval":   "version: 1\nstages: [{id: a, agent: a, approvals: [review]}]",
		"zero retries":       "version: 1\nstages: [{id: a, agent: a}]\npolicy: {max_retries_per_approval: 0}",
		"too many retries":   "version: 1\nstages: [{id: a, agent: a}]\npolicy: {max_retries_per_approval: 11}",
		"zero iterations":    "version: 1\nstages: [{id: a, agent: a}]\npolicy: {execute_max_iterations: 0}",
		"zero token budget":  "version: 1\nstages: [{id: a, agent: a}]\npolicy: {token_budget: 0}",
		"unknown pr mode":    "version: 1\nstages: [{id: a, agent: a}]\npolicy: {pr: {mode: monorepo}}",
		"not yaml structure": "version: 1\nstages: a",
	} {
		if _, e := ParsePipeline([]byte(text)); e == nil {
			t.Errorf("%s: accepted %q", name, text)
		}
	}
}

// Levels group stages whose dependencies are complete; each level is sorted for stable UI output.
func TestLevels(t *testing.T) {
	p, e := ParsePipeline([]byte("version: 1\nstages:\n- {id: review, agent: r, depends_on: [frontend, backend]}\n- {id: frontend, agent: f, depends_on: [design]}\n- {id: planning, agent: p}\n- {id: backend, agent: b, depends_on: [design]}\n- {id: design, agent: d, depends_on: [planning]}\n"))
	if e != nil {
		t.Fatal(e)
	}
	levels, e := p.Levels()
	if e != nil {
		t.Fatal(e)
	}
	want := [][]string{{"planning"}, {"design"}, {"backend", "frontend"}, {"review"}}
	if !slices.EqualFunc(levels, want, slices.Equal) {
		t.Fatalf("levels=%v want %v", levels, want)
	}
}
