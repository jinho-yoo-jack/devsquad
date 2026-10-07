package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/httpapi"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/fake"
	"github.com/jinho-yoo-jack/devsquad/internal/orchestrator"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/scm"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/workspace"
	"github.com/jinho-yoo-jack/devsquad/internal/ws"
)

type harness struct {
	t         *testing.T
	db        *store.Store
	bus       *event.Bus
	emitter   *event.Emitter
	engine    *orchestrator.Orchestrator
	tasks     *app.TaskService
	approvals *app.ApprovalService
	projects  *app.ProjectService
	server    *httptest.Server
	source    string
	runner    stage.Runner
	scm       scm.Publisher
}

func database(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("DEVSQUAD_TEST_DB_URL")
	if url == "" {
		t.Skip("set DEVSQUAD_TEST_DB_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec(ctx, "create schema "+schema); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		pool.Close()
		_, e := admin.Exec(ctx, "drop schema "+schema+" cascade")
		if e != nil {
			t.Error(e)
		}
		admin.Close()
	})
	if e = store.Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if e = store.Migrate(ctx, pool); e != nil {
		t.Fatal("repeat migrations", e)
	}
	return store.New(pool)
}
func newHarness(t *testing.T, p string, runner stage.Runner) *harness {
	t.Helper()
	h := &harness{t: t, db: database(t), source: t.TempDir()}
	h.bus = event.NewBus()
	h.emitter = &event.Emitter{Bus: h.bus}
	definition, e := pipeline.ParsePipeline([]byte(p))
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(h.source, ".devsquad/pipeline.yaml"), p)
	for _, s := range definition.Stages {
		if s.Agent == "publisher" {
			continue
		}
		dir := filepath.Join(h.source, ".devsquad/agents", s.Agent)
		write(t, filepath.Join(dir, "persona.md"), fmt.Sprintf("---\nname: %s\ntool_profile: docs-writer\nwrite_paths: [docs/%s/**]\n---\nTest writer.", s.Agent, s.Agent))
		write(t, filepath.Join(dir, "conventions.md"), "Write a useful result.")
	}
	registry := llm.Registry{ForceFake: true, Fake: fake.LLM{}}
	if runner == nil {
		runner = &stage.AgentRunner{Registry: registry, Budget: &app.BudgetService{Store: h.db, Emitter: h.emitter}}
	}
	h.runner = runner
	h.projects = &app.ProjectService{Store: h.db}
	h.tasks = &app.TaskService{Store: h.db, Emitter: h.emitter, Registry: registry, WorkspaceRoot: t.TempDir()}
	h.approvals = &app.ApprovalService{Store: h.db, Emitter: h.emitter}
	h.restart(runner)
	hub := ws.New(h.db, h.bus)
	h.server = httptest.NewServer(httpapi.Handler(h.projects, h.tasks, h.approvals, hub, nil, nil))
	t.Cleanup(func() { h.engine.Close(); hub.Close(); h.server.Close() })
	return h
}
func (h *harness) restart(runner stage.Runner) {
	if h.engine != nil {
		h.engine.Close()
	}
	h.engine = orchestrator.New(context.Background(), h.db, h.emitter, runner, &workspace.Manager{})
	h.engine.SCM = h.scm
	h.tasks.Coordinator = h.engine
	h.approvals.Coordinator = h.engine
}
func write(t *testing.T, path, body string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func (h *harness) request(method, path string, body any, want int, out any) {
	h.t.Helper()
	raw, _ := json.Marshal(body)
	req, e := http.NewRequest(method, h.server.URL+path, bytes.NewReader(raw))
	if e != nil {
		h.t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		h.t.Fatal(e)
	}
	defer res.Body.Close()
	var data json.RawMessage
	if e = json.NewDecoder(res.Body).Decode(&data); e != nil {
		h.t.Fatal(e)
	}
	if res.StatusCode != want {
		h.t.Fatalf("%s %s: %d want %d: %s", method, path, res.StatusCode, want, data)
	}
	if out != nil {
		if e = json.Unmarshal(data, out); e != nil {
			h.t.Fatal(e)
		}
	}
}
func (h *harness) create() string {
	h.t.Helper()
	var p app.ProjectEntity
	h.request("POST", "/api/v1/projects", map[string]any{"name": "test", "github_owner": "test", "github_repo": "repo", "local_path": h.source}, 201, &p)
	var task app.TaskResponse
	h.request("POST", "/api/v1/tasks", map[string]any{"project_id": p.ID, "command": "Build a feature"}, 201, &task)
	return task.ID
}
func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func (h *harness) pending(id, key, kind string, retry int) domain.ApprovalEntity {
	h.t.Helper()
	var found domain.ApprovalEntity
	eventually(h.t, func() bool {
		task, err := h.db.Task(context.Background(), id)
		if err != nil {
			h.t.Fatal(err)
		}
		if domain.Terminal(task.Status) {
			events, _ := h.db.Events(context.Background(), id, 0, 1000)
			h.t.Fatalf("unexpected terminal task: %+v events=%+v", task, events)
		}
		stages, e := h.db.Stages(context.Background(), id)
		if e != nil {
			h.t.Fatal(e)
		}
		stageID := ""
		for _, s := range stages {
			if s.StageKey == key {
				stageID = s.ID
			}
		}
		list, e := h.db.Approvals(context.Background(), &id, domain.Ptr("pending"))
		if e != nil {
			h.t.Fatal(e)
		}
		for _, a := range list {
			if a.StageID == stageID && a.Kind == kind && a.RetryNo == retry {
				found = a
				return true
			}
		}
		return false
	})
	return found
}
func (h *harness) decide(a domain.ApprovalEntity, decision string, extra map[string]any) {
	h.t.Helper()
	if extra == nil {
		extra = map[string]any{}
	}
	extra["decision"] = decision
	h.request("POST", "/api/v1/approvals/"+a.ID+"/decide", extra, 200, nil)
}
func (h *harness) status(id, status string) {
	h.t.Helper()
	eventually(h.t, func() bool {
		v, e := h.db.Task(context.Background(), id)
		if e != nil {
			h.t.Fatal(e)
		}
		return v.Status == status
	})
}
func TestUnifiedApprovalFlowAndWebSocket(t *testing.T) {
	h := newHarness(t, "version: 1\nstages:\n- {id: a, agent: a}\n- {id: b, agent: b}\n- {id: c, agent: c, depends_on: [a], approvals: []}\n", nil)
	id := h.create()
	a := h.pending(id, "a", "plan", 0)
	b := h.pending(id, "b", "plan", 0)
	h.decide(a, "reject", map[string]any{"feedback": "add details"})
	a = h.pending(id, "a", "plan", 1)
	if !strings.Contains(domain.Text(a.Content), "add details") {
		t.Fatal("feedback missing")
	}
	var success, conflict atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			_, e := h.approvals.UpdateApproval(context.Background(), a.ID, app.DecisionRequest{Decision: "edit", EditedContent: domain.Ptr("Write docs/a/edited.md")}, "tester")
			if e == nil {
				success.Add(1)
			} else {
				var api *domain.Error
				if errors.As(e, &api) && api.Code == "APPROVAL_ALREADY_DECIDED" {
					conflict.Add(1)
				} else {
					t.Error(e)
				}
			}
		})
	}
	wg.Wait()
	if success.Load() != 1 || conflict.Load() != 11 {
		t.Fatalf("decision race: %d/%d", success.Load(), conflict.Load())
	}
	a = h.pending(id, "a", "deliverable", 0)
	h.decide(a, "reject", map[string]any{"feedback": "improve result"})
	a = h.pending(id, "a", "deliverable", 1)
	h.decide(a, "edit", map[string]any{"edited_content": "approved summary"})
	eventually(t, func() bool {
		stages, _ := h.db.Stages(context.Background(), id)
		for _, s := range stages {
			if s.StageKey == "c" {
				return s.Status == "approved"
			}
		}
		return false
	})
	h.status(id, "waiting_approval")
	h.request("POST", "/api/v1/tasks/"+id+"/pause", nil, 200, nil)
	h.decide(b, "approve", nil)
	h.status(id, "paused")
	stages, _ := h.db.Stages(context.Background(), id)
	for _, s := range stages {
		if s.StageKey == "b" && s.Status != "executing" {
			t.Fatal(s)
		}
	}
	h.request("POST", "/api/v1/tasks/"+id+"/resume", nil, 200, nil)
	b = h.pending(id, "b", "deliverable", 0)
	h.restart(h.runner)
	if e := h.engine.Recover(context.Background()); e != nil {
		t.Fatal(e)
	}
	h.decide(b, "approve", nil)
	h.status(id, "completed")
	task, _ := h.db.Task(context.Background(), id)
	content, e := os.ReadFile(filepath.Join(task.Workspace, "docs/a/edited.md"))
	if e != nil || !bytes.Contains(content, []byte("improve result")) {
		t.Fatalf("deliverable: %s %v", content, e)
	}
	var page app.EventPage
	h.request("GET", "/api/v1/tasks/"+id+"/events?limit=1000", nil, 200, &page)
	for i, ev := range page.Items {
		if ev.Seq != int64(i+1) {
			t.Fatalf("event gap at %d: %d", i, ev.Seq)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.server.URL, "http")+"/ws", nil)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	if e = wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "task_id": id, "from_seq": 0}); e != nil {
		t.Fatal(e)
	}
	count := 0
	for {
		var raw json.RawMessage
		if e = wsjson.Read(ctx, conn, &raw); e != nil {
			t.Fatal(e)
		}
		var op struct {
			Op       string `json:"op"`
			Replayed int    `json:"replayed"`
		}
		_ = json.Unmarshal(raw, &op)
		if op.Op == "subscribed" {
			if op.Replayed != len(page.Items) {
				t.Fatal(op)
			}
			break
		}
		var ev domain.Event
		if e = json.Unmarshal(raw, &ev); e != nil {
			t.Fatal(e)
		}
		want, _ := json.Marshal(page.Items[count])
		got, _ := json.Marshal(ev)
		if !bytes.Equal(want, got) {
			t.Fatalf("REST/WS diverged: %s != %s", want, got)
		}
		count++
	}
	if count != len(page.Items) {
		t.Fatal(count)
	}
}

type runnerFunc func(context.Context, stage.Input) (stage.Result, error)

func (f runnerFunc) Run(ctx context.Context, in stage.Input) (stage.Result, error) { return f(ctx, in) }
func TestCrashRecoveryAndDuplicateReconcile(t *testing.T) {
	started := make(chan string, 1)
	var calls atomic.Int32
	crashing := runnerFunc(func(ctx context.Context, in stage.Input) (stage.Result, error) {
		if in.Mode == "plan" {
			return stage.Result{Content: "plan"}, nil
		}
		calls.Add(1)
		dir := filepath.Join(in.Definition.Workspace, "docs", in.Stage.Agent)
		if e := os.MkdirAll(dir, 0700); e != nil {
			return stage.Result{}, e
		}
		if in.Stage.ID == "b" {
			if e := os.WriteFile(filepath.Join(dir, "approved.md"), []byte("keep"), 0600); e != nil {
				return stage.Result{}, e
			}
			return stage.Result{Content: "keep", Ref: domain.Ptr("docs/b/approved.md")}, nil
		}
		if e := os.WriteFile(filepath.Join(dir, "partial.md"), []byte("incomplete"), 0600); e != nil {
			return stage.Result{}, e
		}
		started <- in.TaskID
		<-ctx.Done()
		return stage.Result{}, ctx.Err()
	})
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a, approvals: []}, {id: b, agent: b, approvals: []}]", crashing)
	id := h.create()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("execute never started")
	}
	eventually(t, func() bool {
		stages, _ := h.db.Stages(context.Background(), id)
		for _, s := range stages {
			if s.StageKey == "b" {
				return s.Status == "approved"
			}
		}
		return false
	})
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			if e := h.engine.Reconcile(context.Background(), id); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 2 {
		t.Fatalf("duplicate runners: %d", calls.Load())
	}
	h.engine.Close()
	var recovered atomic.Int32
	recovering := runnerFunc(func(ctx context.Context, in stage.Input) (stage.Result, error) {
		if in.Mode != "execute" || in.Stage.ID != "a" {
			return stage.Result{}, fmt.Errorf("unexpected recovered stage: %s %s", in.Stage.ID, in.Mode)
		}
		recovered.Add(1)
		if _, e := os.Stat(filepath.Join(in.Definition.Workspace, "docs/a/partial.md")); !os.IsNotExist(e) {
			return stage.Result{}, fmt.Errorf("partial file survived reset")
		}
		b, e := os.ReadFile(filepath.Join(in.Definition.Workspace, "docs/b/approved.md"))
		if e != nil || string(b) != "keep" {
			return stage.Result{}, fmt.Errorf("sibling output lost: %v", e)
		}
		return stage.Result{Content: "recovered"}, nil
	})
	h.restart(recovering)
	if e := h.engine.Recover(context.Background()); e != nil {
		t.Fatal(e)
	}
	h.status(id, "completed")
	if recovered.Load() != 1 {
		t.Fatal(recovered.Load())
	}
	var n int
	if e := h.db.Pool.QueryRow(context.Background(), "select count(*) from deliverable where task_id=$1", id).Scan(&n); e != nil || n != 2 {
		t.Fatalf("deliverable duplicates: %d %v", n, e)
	}
}
