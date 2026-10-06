package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/openai"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
)

// This test makes billable provider calls only when explicitly enabled.
func TestLiveOpenAIPipeline(t *testing.T) {
	if os.Getenv("DEVSQUAD_REAL_LLM_TEST") != "true" {
		t.Skip("opt-in live provider test")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("OPENAI_API_KEY is required")
	}
	model := os.Getenv("DEVSQUAD_REAL_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	registry := llm.Registry{Providers: map[string]llm.Factory{"openai": func(m string) llm.LLM { return openai.New(m) }}}
	runner := &stage.AgentRunner{Registry: registry}
	h := newHarness(t, fmt.Sprintf("version: 1\nstages: [{id: a, agent: a, approvals: []}]\npolicy: {default_model: openai/%s, execute_max_iterations: 3}\n", model), runner)
	h.tasks.Registry = registry
	p, e := h.projects.CreateProject(context.Background(), app.CreateProjectRequest{Name: "live smoke", GithubOwner: "test", GithubRepo: "test", LocalPath: &h.source})
	if e != nil {
		t.Fatal(e)
	}
	task, e := h.tasks.CreateTask(context.Background(), app.CreateTaskRequest{ProjectID: p.ID, Command: "Plan in one sentence. Then use write_file to create docs/a/smoke.md containing only '# Smoke test'. Do not read or create other files. End after the write succeeds."}, "live-test")
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		current, e := h.db.Task(context.Background(), task.ID)
		if e != nil {
			t.Fatal(e)
		}
		switch current.Status {
		case "failed":
			events, _ := h.db.Events(context.Background(), task.ID, 0, 100)
			for _, ev := range events {
				if ev.Type == "run.failed" {
					t.Fatalf("live provider failed: %s", ev.Payload)
				}
			}
			t.Fatal("live task failed")
		case "completed":
			b, e := os.ReadFile(filepath.Join(current.Workspace, "docs/a/smoke.md"))
			if e != nil || !strings.Contains(string(b), "# Smoke test") {
				t.Fatalf("live output missing: %v", e)
			}
			var calls, tokens int64
			if e = h.db.Pool.QueryRow(context.Background(), "select count(*),sum(input_tokens+output_tokens) from usage_record where task_id=$1", task.ID).Scan(&calls, &tokens); e != nil {
				t.Fatal(e)
			}
			t.Logf("completed model=%s calls=%d tokens=%d", model, calls, tokens)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("live pipeline timeout")
}
