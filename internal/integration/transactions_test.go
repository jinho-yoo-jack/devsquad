package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
)

func TestRollbackDoesNotPublishOrConsumeSequence(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]", nil)
	h.tasks.Coordinator = nil
	id := h.create()
	sub := h.bus.Subscribe(id)
	defer sub.Close()
	ctx := context.Background()
	sentinel := errors.New("rollback")
	e := h.db.WithTx(ctx, func(tx *store.Tx) error {
		if e := h.emitter.Emit(ctx, tx, id, "a", "a", "usage", event.UsagePayload{Model: "fake/echo"}); e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
	select {
	case ev := <-sub.Events:
		t.Fatalf("published rolled back event: %+v", ev)
	default:
	}
	var usage int
	if e = h.db.Pool.QueryRow(ctx, "select count(*) from usage_record").Scan(&usage); e != nil || usage != 0 {
		t.Fatalf("usage rollback: %d %v", usage, e)
	}
	task, _ := h.db.Task(ctx, id)
	if task.EventSeq != 0 {
		t.Fatal(task.EventSeq)
	}
	if e = h.db.WithTx(ctx, func(tx *store.Tx) error { return h.emitter.Emit(ctx, tx, id, "", "", "run.started", struct{}{}) }); e != nil {
		t.Fatal(e)
	}
	select {
	case ev := <-sub.Events:
		if ev.Seq != 1 {
			t.Fatal(ev.Seq)
		}
	case <-time.After(time.Second):
		t.Fatal("missing committed event")
	}
	stages, _ := h.db.Stages(ctx, id)
	a := sqlc.CreateApprovalParams{ID: uuid.NewString(), TaskID: id, StageID: stages[0].ID, Kind: "plan", RetryNo: 0, Title: "plan", Content: "plan"}
	if e = h.db.WithTx(ctx, func(tx *store.Tx) error { return tx.Q.CreateApproval(ctx, a) }); e != nil {
		t.Fatal(e)
	}
	a.ID = uuid.NewString()
	a.RetryNo = 1
	if e = h.db.WithTx(ctx, func(tx *store.Tx) error { return tx.Q.CreateApproval(ctx, a) }); e == nil {
		t.Fatal("multiple pending approvals accepted")
	}
	if _, e = h.db.Pool.Exec(ctx, "update stage set status='illegal' where id=$1", stages[0].ID); e == nil {
		t.Fatal("invalid stage status accepted")
	}
}
func TestRejectionLimitAndManualResume(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]\npolicy: {max_retries_per_approval: 2}", nil)
	id := h.create()
	for retry := 0; retry < 2; retry++ {
		a := h.pending(id, "a", "plan", retry)
		h.decide(a, "reject", map[string]any{"feedback": "revise"})
	}
	h.status(id, "blocked")
	h.request("POST", "/api/v1/tasks/"+id+"/resume", nil, 200, nil)
	a := h.pending(id, "a", "plan", 2)
	h.decide(a, "approve", nil)
	a = h.pending(id, "a", "deliverable", 0)
	h.decide(a, "approve", nil)
	h.status(id, "completed")
	h.request("POST", "/api/v1/tasks/"+id+"/cancel", nil, 409, nil)
}
func TestCancellationFencesLateResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	runner := runnerFunc(func(ctx context.Context, in stage.Input) (stage.Result, error) {
		if in.Mode == "plan" {
			return stage.Result{Content: "plan"}, nil
		}
		close(started)
		<-release
		close(finished)
		return stage.Result{Content: "late"}, nil
	})
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a, approvals: []}]", runner)
	id := h.create()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("execute never started")
	}
	h.request("POST", "/api/v1/tasks/"+id+"/cancel", nil, 200, nil)
	close(release)
	<-finished
	h.engine.Close()
	h.status(id, "cancelled")
	var n int
	if e := h.db.Pool.QueryRow(context.Background(), "select count(*) from deliverable where task_id=$1", id).Scan(&n); e != nil || n != 0 {
		t.Fatalf("late result committed: %d %v", n, e)
	}
	events, _ := h.db.Events(context.Background(), id, 0, 100)
	for _, ev := range events {
		if ev.Type == "run.failed" || ev.Type == "run.completed" || ev.Type == "approval.requested" {
			t.Fatal(ev.Type)
		}
	}
}
func TestInvalidCreationIsAtomicAndHTTPCompatibility(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]", nil)
	p, e := h.projects.CreateProject(context.Background(), app.CreateProjectRequest{Name: "test", GithubOwner: "test", GithubRepo: "repo", LocalPath: &h.source})
	if e != nil {
		t.Fatal(e)
	}
	write(t, h.source+"/.devsquad/pipeline.yaml", "version: 1\nstages: [{id: a, agent: missing}]\n")
	var fault domain.Error
	h.request("POST", "/api/v1/tasks", map[string]any{"project_id": p.ID, "command": "work"}, 400, &fault)
	if fault.Code != "PIPELINE_INVALID" {
		t.Fatal(fault)
	}
	tasks, e := h.db.Tasks(context.Background(), nil, nil)
	if e != nil || len(tasks) != 0 {
		t.Fatalf("partial task: %v %v", tasks, e)
	}
	entries, e := os.ReadDir(h.tasks.WorkspaceRoot)
	if e != nil || len(entries) != 0 {
		t.Fatalf("orphan workspace: %v %v", entries, e)
	}
	for _, path := range []string{"/api/v1/tasks/not-uuid", "/api/v1/tasks?project_id=bad", "/api/v1/tasks?status=unknown"} {
		h.request("GET", path, nil, 400, &fault)
		if fault.Code != "BAD_REQUEST" {
			t.Fatal(fault)
		}
	}
	h.request("GET", "/api/v1/tasks?project_id="+p.ID+"&status=RUNNING,QUEUED", nil, 200, nil)
	h.request("GET", "/api/v1/health", nil, 200, nil)
}
func TestRunnerFailureCancelsSiblings(t *testing.T) {
	ready := make(chan struct{})
	cancelled := make(chan struct{})
	runner := runnerFunc(func(ctx context.Context, in stage.Input) (stage.Result, error) {
		if in.Stage.ID == "a" {
			<-ready
			return stage.Result{}, errors.New("fixture failure")
		}
		close(ready)
		<-ctx.Done()
		close(cancelled)
		return stage.Result{}, ctx.Err()
	})
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}, {id: b, agent: b}]", runner)
	id := h.create()
	h.status(id, "failed")
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("sibling was not cancelled")
	}
	events, _ := h.db.Events(context.Background(), id, 0, 100)
	n := 0
	for _, ev := range events {
		if ev.Type == "run.failed" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("failure count: %d", n)
	}
}
