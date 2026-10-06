package stage_test

import (
	"context"
	"testing"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/fake"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
)

type observedModel struct {
	t      *testing.T
	latest *string
	calls  int
}

func (m *observedModel) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	m.t.Helper()
	if *m.latest != "agent.thinking" {
		m.t.Fatalf("model is running while UI still reports %q", *m.latest)
	}
	m.calls++
	return (fake.LLM{}).Chat(ctx, req)
}

func TestActivityEmittedBeforeEveryModelCall(t *testing.T) {
	for _, mode := range []string{"plan", "execute"} {
		t.Run(mode, func(t *testing.T) {
			latest := ""
			model := &observedModel{t: t, latest: &latest}
			runner := stage.AgentRunner{Registry: llm.Registry{ForceFake: true, Fake: model}}
			thinking := 0
			in := stage.Input{
				TaskID: "t", Command: "activity", Mode: mode,
				Stage: pipeline.StageSpec{ID: "build", Agent: "writer"},
				Definition: agentdef.Definition{
					Workspace: t.TempDir(),
					Pipeline:  pipeline.Pipeline{Policy: pipeline.Policy{MaxIterations: 3}},
					Agents:    map[string]agentdef.AgentSpec{"writer": {Name: "writer", ToolProfile: "docs-writer", WritePaths: []string{"docs/**"}}},
				},
				Emit: func(_ context.Context, kind string, payload any) error {
					latest = kind
					if kind == "agent.thinking" {
						thinking++
						p := payload.(map[string]any)
						if p["iteration"] != thinking || p["model"] != "fake/echo" || p["summary"] != mode {
							t.Fatalf("incorrect activity: %v", p)
						}
					}
					return nil
				},
			}
			if _, err := runner.Run(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == "execute" {
				want = 2
			}
			if thinking != want || model.calls != want {
				t.Fatalf("thinking=%d calls=%d want=%d", thinking, model.calls, want)
			}
		})
	}
}
