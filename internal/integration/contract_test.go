package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
)

const gated = "version: 1\nstages:\n- {id: a, agent: a}\n"

type apiError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details"`
}

func (h *harness) fails(method, path string, body any, status int, code string) {
	h.t.Helper()
	var got apiError
	h.request(method, path, body, status, &got)
	if got.Code != code || got.Message == "" || string(got.Details) != "{}" {
		h.t.Fatalf("%s %s: error body %+v want code %s", method, path, got, code)
	}
}
func (h *harness) project() string {
	h.t.Helper()
	var p app.ProjectEntity
	h.request("POST", "/api/v1/projects", map[string]any{"name": "test", "github_owner": "test", "github_repo": "repo", "local_path": h.source}, 201, &p)
	return p.ID
}

// 15-API-이벤트-명세 §1 common error body and §7 codes.
func TestErrorResponsesFollowContract(t *testing.T) {
	h := newHarness(t, gated, nil)
	missing := uuid.NewString()
	h.fails("GET", "/api/v1/tasks/"+missing, nil, 404, "TASK_NOT_FOUND")
	h.fails("GET", "/api/v1/approvals/"+missing, nil, 404, "APPROVAL_NOT_FOUND")
	h.fails("GET", "/api/v1/projects/"+missing, nil, 404, "PROJECT_NOT_FOUND")
	h.fails("GET", "/api/v1/tasks/"+missing+"/events", nil, 404, "TASK_NOT_FOUND")
	h.fails("GET", "/api/v1/tasks/not-a-uuid", nil, 400, "BAD_REQUEST")
	h.fails("GET", "/api/v1/tasks?status=sleeping", nil, 400, "BAD_REQUEST")
	h.fails("POST", "/api/v1/tasks", map[string]any{"project_id": "x", "command": "c"}, 400, "BAD_REQUEST")
	h.fails("POST", "/api/v1/tasks", map[string]any{"project_id": missing, "command": "c"}, 404, "PROJECT_NOT_FOUND")
	h.fails("POST", "/api/v1/tasks", map[string]any{"project_id": h.project(), "command": "  "}, 400, "BAD_REQUEST")
	h.fails("POST", "/api/v1/projects", map[string]any{"name": "x"}, 400, "BAD_REQUEST")
	h.fails("POST", "/api/v1/approvals/"+missing+"/decide", map[string]any{"decision": "approve"}, 404, "APPROVAL_NOT_FOUND")
	var health map[string]any
	h.request("GET", "/api/v1/health", nil, 200, &health)
	if health["status"] != "ok" || health["service"] != "devsquad" || health["ts"] == nil {
		t.Fatal(health)
	}
}

func TestDecisionValidationAndResponse(t *testing.T) {
	h := newHarness(t, gated, nil)
	id := h.create()
	a := h.pending(id, "a", "plan", 0)
	path := "/api/v1/approvals/" + a.ID + "/decide"
	h.fails("POST", path, map[string]any{"decision": "reject"}, 400, "FEEDBACK_REQUIRED")
	h.fails("POST", path, map[string]any{"decision": "reject", "feedback": "   "}, 400, "FEEDBACK_REQUIRED")
	h.fails("POST", path, map[string]any{"decision": "edit"}, 400, "EDITED_CONTENT_REQUIRED")
	h.fails("POST", path, map[string]any{"decision": "maybe"}, 400, "BAD_REQUEST")
	var listed []app.ApprovalEntity
	h.request("GET", "/api/v1/approvals?status=pending", nil, 200, &listed)
	if len(listed) != 1 || listed[0].ID != a.ID || listed[0].Content != nil {
		t.Fatalf("header list must omit content: %+v", listed)
	}
	var detail app.ApprovalEntity
	h.request("GET", "/api/v1/approvals/"+a.ID, nil, 200, &detail)
	if domain.Text(detail.Content) == "" || detail.Status != "pending" {
		t.Fatalf("detail must include content: %+v", detail)
	}
	var decided map[string]any
	h.request("POST", path, map[string]any{"decision": "APPROVE"}, 200, &decided)
	if decided["id"] != a.ID || decided["status"] != "approved" || decided["decided_via"] != "web" || decided["decided_at"] == nil {
		t.Fatalf("decide response: %v", decided)
	}
	h.fails("POST", path, map[string]any{"decision": "approve"}, 409, "APPROVAL_ALREADY_DECIDED")
	h.request("GET", "/api/v1/approvals/"+a.ID, nil, 200, &detail)
	if domain.Text(detail.DecidedBy) != "local-user" {
		t.Fatalf("decided_by: %+v", detail)
	}
}

// The web client sends repeated, upper-case status parameters (web/lib/api/tasks.ts).
func TestTaskListFiltersLikeWebClient(t *testing.T) {
	h := newHarness(t, gated, nil)
	waiting, paused := h.create(), h.create()
	h.pending(waiting, "a", "plan", 0)
	h.pending(paused, "a", "plan", 0)
	h.request("POST", "/api/v1/tasks/"+paused+"/pause", nil, 200, nil)
	ids := func(query string) []string {
		var page app.Page[app.TaskResponse]
		h.request("GET", "/api/v1/tasks"+query, nil, 200, &page)
		out := []string{}
		for _, t := range page.Items {
			out = append(out, t.ID)
		}
		slices.Sort(out)
		return out
	}
	both := []string{waiting, paused}
	slices.Sort(both)
	for query, want := range map[string][]string{
		"":                                       both,
		"?status=PAUSED&status=WAITING_APPROVAL": both,
		"?status=paused,waiting_approval":        both,
		"?status=paused":                         {paused},
		"?status=WAITING_APPROVAL":               {waiting},
		"?status=completed":                      {},
		"?project_id=" + uuid.NewString():        {},
	} {
		if got := ids(query); !slices.Equal(got, want) {
			t.Errorf("%q: %v want %v", query, got, want)
		}
	}
	var task app.TaskResponse
	h.request("GET", "/api/v1/tasks/"+waiting, nil, 200, &task)
	if len(task.PendingApprovals) != 1 || task.PendingApprovals[0].StageKey != "a" || task.PendingApprovals[0].Kind != "plan" || task.LastEvent == nil || task.LastEvent.Type != "approval.requested" {
		t.Fatalf("task detail: %+v", task)
	}
	if len(task.Stages) != 1 || task.Stages[0].Key != "a" || task.Stages[0].Role != "a" || task.Stages[0].Status != "plan_review" || task.Stages[0].DependsOn == nil {
		t.Fatalf("stages: %+v", task.Stages)
	}
	h.fails("POST", "/api/v1/tasks/"+waiting+"/resume", nil, 409, "INVALID_TRANSITION")
	h.request("POST", "/api/v1/tasks/"+waiting+"/cancel", nil, 200, nil)
	h.fails("POST", "/api/v1/tasks/"+waiting+"/cancel", nil, 409, "INVALID_TRANSITION")
	h.fails("POST", "/api/v1/tasks/"+waiting+"/pause", nil, 409, "INVALID_TRANSITION")
}

func TestEventPaging(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a, approvals: []}]\n", nil)
	id := h.create()
	h.status(id, "completed")
	var all app.EventPage
	h.request("GET", "/api/v1/tasks/"+id+"/events", nil, 200, &all)
	if len(all.Items) < 4 || all.NextAfterSeq != nil {
		t.Fatalf("default page: %d %v", len(all.Items), all.NextAfterSeq)
	}
	var first, rest app.EventPage
	h.request("GET", "/api/v1/tasks/"+id+"/events?limit=2", nil, 200, &first)
	if len(first.Items) != 2 || first.NextAfterSeq == nil || *first.NextAfterSeq != 2 {
		t.Fatalf("first page: %+v", first)
	}
	h.request("GET", fmt.Sprintf("/api/v1/tasks/%s/events?after_seq=%d&limit=1000", id, *first.NextAfterSeq), nil, 200, &rest)
	if len(rest.Items) != len(all.Items)-2 || rest.Items[0].Seq != 3 || rest.NextAfterSeq != nil {
		t.Fatalf("second page: %d %v", len(rest.Items), rest.NextAfterSeq)
	}
	h.fails("GET", "/api/v1/tasks/"+id+"/events?after_seq=-1", nil, 400, "BAD_REQUEST")
}

type payloads map[string][]map[string]any

func (h *harness) payloads(id string) payloads {
	h.t.Helper()
	events, e := h.db.Events(context.Background(), id, 0, 10000)
	if e != nil {
		h.t.Fatal(e)
	}
	out := payloads{}
	for _, ev := range events {
		var p map[string]any
		if e = json.Unmarshal(ev.Payload, &p); e != nil {
			h.t.Fatal(e)
		}
		out[ev.Type] = append(out[ev.Type], p)
	}
	return out
}
func (p payloads) require(t *testing.T, kind string, keys ...string) {
	t.Helper()
	if len(p[kind]) == 0 {
		t.Errorf("no %s event", kind)
		return
	}
	for _, payload := range p[kind] {
		for _, k := range keys {
			if _, ok := payload[k]; !ok {
				t.Errorf("%s payload missing %q: %v", kind, k, payload)
			}
		}
	}
}

// 15-API-이벤트-명세 §3: each event type carries its documented payload fields.
func TestEventPayloadsFollowContract(t *testing.T) {
	h := newHarness(t, gated, nil)
	id := h.create()
	h.decide(h.pending(id, "a", "plan", 0), "approve", nil)
	h.decide(h.pending(id, "a", "deliverable", 0), "reject", map[string]any{"feedback": "more detail"})
	h.decide(h.pending(id, "a", "deliverable", 1), "approve", nil)
	h.status(id, "completed")
	p := h.payloads(id)
	p.require(t, "run.started", "stages", "levels")
	p.require(t, "stage.started")
	p.require(t, "agent.thinking", "summary", "iteration", "model")
	p.require(t, "usage", "model", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens")
	p.require(t, "agent.tool_call", "call_id", "tool", "args_summary")
	p.require(t, "agent.tool_result", "call_id", "tool", "ok", "summary", "duration_ms")
	p.require(t, "deliverable.produced", "deliverable_id", "kind", "summary")
	p.require(t, "approval.requested", "approval_id", "kind", "retry_no", "title", "content_preview")
	p.require(t, "approval.decided", "approval_id", "decision", "decided_via", "decided_by")
	p.require(t, "stage.completed", "deliverable_id")
	p.require(t, "run.completed")
	if len(p["approval.requested"]) != 3 || len(p["deliverable.produced"]) != 2 {
		t.Fatalf("requests=%d deliverables=%d", len(p["approval.requested"]), len(p["deliverable.produced"]))
	}
	last := p["deliverable.produced"][1]["deliverable_id"]
	if p["approval.requested"][2]["deliverable_id"] != last || p["stage.completed"][0]["deliverable_id"] != last {
		t.Fatalf("deliverable ids diverged: %v %v %v", last, p["approval.requested"][2], p["stage.completed"][0])
	}
	if p["approval.requested"][0]["deliverable_id"] != nil {
		t.Fatalf("plan approval must not reference a deliverable: %v", p["approval.requested"][0])
	}

	other := h.create()
	h.pending(other, "a", "plan", 0)
	for _, action := range []string{"pause", "resume", "cancel"} {
		h.request("POST", "/api/v1/tasks/"+other+"/"+action, nil, 200, nil)
	}
	p = h.payloads(other)
	p.require(t, "run.paused", "reason")
	p.require(t, "run.resumed")
	p.require(t, "run.cancelled")
	if p["run.paused"][0]["reason"] != "user" {
		t.Fatal(p["run.paused"])
	}
}

func TestBlockedAndFailedPayloads(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]\npolicy: {max_retries_per_approval: 1}\n", nil)
	id := h.create()
	h.decide(h.pending(id, "a", "plan", 0), "reject", map[string]any{"feedback": "wrong scope"})
	h.status(id, "blocked")
	p := h.payloads(id)
	p.require(t, "stage.blocked", "reason", "last_feedback")
	if p["stage.blocked"][0]["last_feedback"] != "wrong scope" {
		t.Fatal(p["stage.blocked"])
	}

	failing := runnerFunc(func(context.Context, stage.Input) (stage.Result, error) {
		return stage.Result{}, fmt.Errorf("model unavailable")
	})
	f := newHarness(t, gated, failing)
	id = f.create()
	f.status(id, "failed")
	p = f.payloads(id)
	p.require(t, "run.failed", "error", "node")
	if p["run.failed"][0]["node"] != "a" || !strings.Contains(fmt.Sprint(p["run.failed"][0]["error"]), "model unavailable") {
		t.Fatal(p["run.failed"])
	}
}

// The orchestrator moves a Task between running and waiting_approval at any moment.
// A user action valid in both states must not fail because that happened mid-request.
func TestTaskActionSurvivesConcurrentStatusChange(t *testing.T) {
	h := newHarness(t, gated, nil)
	id := h.create()
	h.pending(id, "a", "plan", 0)
	ctx := context.Background()
	locked, release, held := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		held <- h.db.WithTx(ctx, func(tx *store.Tx) error {
			task, e := tx.LockTask(ctx, id)
			if e != nil {
				return e
			}
			if e = tx.SetStatus(ctx, &task, "running"); e != nil {
				return e
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	result := make(chan error, 1)
	go func() { _, e := h.tasks.UpdateTask(ctx, id, "cancel"); result <- e }()
	eventually(t, func() bool {
		var waiting int
		_ = h.db.Pool.QueryRow(ctx, "select count(*) from pg_stat_activity where wait_event_type='Lock' and datname=current_database()").Scan(&waiting)
		return waiting > 0
	})
	close(release)
	if e := <-held; e != nil {
		t.Fatal(e)
	}
	if e := <-result; e != nil {
		t.Fatalf("cancel rejected after a concurrent status change: %v", e)
	}
	h.status(id, "cancelled")
}

// A cancelled or failed Task can no longer be decided (409), so its approvals must not
// stay in the pending lists that drive the header badge and the task row badge.
func TestTerminalTaskApprovalsLeavePendingLists(t *testing.T) {
	h := newHarness(t, gated, nil)
	id := h.create()
	a := h.pending(id, "a", "plan", 0)
	h.request("POST", "/api/v1/tasks/"+id+"/cancel", nil, 200, nil)
	h.fails("POST", "/api/v1/approvals/"+a.ID+"/decide", map[string]any{"decision": "approve"}, 409, "INVALID_TRANSITION")
	var listed []app.ApprovalEntity
	h.request("GET", "/api/v1/approvals?status=pending", nil, 200, &listed)
	if len(listed) != 0 {
		t.Errorf("cancelled task approval still pending in header list: %+v", listed)
	}
	var task app.TaskResponse
	h.request("GET", "/api/v1/tasks/"+id, nil, 200, &task)
	if len(task.PendingApprovals) != 0 {
		t.Errorf("cancelled task still reports pending approvals: %+v", task.PendingApprovals)
	}
	h.request("GET", "/api/v1/tasks/"+id+"/approvals?status=pending", nil, 200, &listed)
	if len(listed) != 0 {
		t.Errorf("task pending filter: %+v", listed)
	}
	h.request("GET", "/api/v1/tasks/"+id+"/approvals", nil, 200, &listed)
	if len(listed) != 1 || listed[0].ID != a.ID {
		t.Errorf("the undecided approval must stay in the task history: %+v", listed)
	}
}

type socket struct {
	t    *testing.T
	ctx  context.Context
	conn *websocket.Conn
}

func (h *harness) socket() *socket {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	h.t.Cleanup(cancel)
	conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.server.URL, "http")+"/ws", nil)
	if e != nil {
		h.t.Fatal(e)
	}
	h.t.Cleanup(func() { conn.CloseNow() })
	return &socket{h.t, ctx, conn}
}
func (s *socket) send(v any) {
	s.t.Helper()
	if e := wsjson.Write(s.ctx, s.conn, v); e != nil {
		s.t.Fatal(e)
	}
}
func (s *socket) next() map[string]any {
	s.t.Helper()
	var m map[string]any
	if e := wsjson.Read(s.ctx, s.conn, &m); e != nil {
		s.t.Fatal(e)
	}
	return m
}

// 15-API-이벤트-명세 §2 and 19 §7.3: ping, errors, replay acknowledgement and the summary channel.
func TestWebSocketProtocol(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a, approvals: []}]\n", nil)
	done := h.create()
	h.status(done, "completed")
	task, _ := h.db.Task(context.Background(), done)
	s := h.socket()
	s.send(map[string]any{"op": "ping"})
	if m := s.next(); m["op"] != "pong" {
		t.Fatal(m)
	}
	s.send(map[string]any{"op": "dance"})
	if m := s.next(); m["op"] != "error" || m["code"] != "UNKNOWN_OP" {
		t.Fatal(m)
	}
	s.send(map[string]any{"op": "subscribe", "task_id": "nope"})
	if m := s.next(); m["op"] != "error" || m["code"] != "BAD_REQUEST" {
		t.Fatal(m)
	}
	s.send(map[string]any{"op": "subscribe", "task_id": uuid.NewString()})
	if m := s.next(); m["op"] != "error" || m["code"] != "TASK_NOT_FOUND" {
		t.Fatal(m)
	}
	s.send(map[string]any{"op": "subscribe", "task_id": done, "from_seq": task.EventSeq - 1})
	if m := s.next(); m["seq"] != float64(task.EventSeq) {
		t.Fatalf("from_seq replays events after the given seq: %v", m)
	}
	if m := s.next(); m["op"] != "subscribed" || m["task_id"] != done || m["replayed"] != float64(1) {
		t.Fatal(m)
	}
	s.send(map[string]any{"op": "subscribe", "task_id": done, "from_seq": task.EventSeq})
	if m := s.next(); m["op"] != "subscribed" || m["task_id"] != done || m["replayed"] != float64(0) {
		t.Fatalf("subscribed must report replayed even when nothing is replayed: %v", m)
	}

	summary := h.socket()
	summary.send(map[string]any{"op": "subscribe"})
	if m := summary.next(); m["op"] != "subscribed" {
		t.Fatal(m)
	}
	live := h.create()
	seen := map[string]bool{}
	for !seen["run.completed"] {
		m := summary.next()
		kind, _ := m["type"].(string)
		if m["task_id"] != live || !(strings.HasPrefix(kind, "run.") || strings.HasPrefix(kind, "stage.") || strings.HasPrefix(kind, "approval.")) {
			t.Fatalf("summary channel leaked %v", m)
		}
		seen[kind] = true
	}
	if !seen["run.started"] || !seen["stage.started"] || !seen["stage.completed"] {
		t.Fatalf("summary channel missed lifecycle events: %v", seen)
	}
}
