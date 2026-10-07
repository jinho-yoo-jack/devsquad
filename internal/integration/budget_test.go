package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
)

// 15-API-이벤트-명세 §6: policy.token_budget, otherwise project.token_budget (default 2,000,000).
func TestTaskBudgetSnapshot(t *testing.T) {
	h := newHarness(t, gated, nil)
	var task app.TaskResponse
	h.request("GET", "/api/v1/tasks/"+h.create(), nil, 200, &task)
	if task.TokenBudget != 2_000_000 || task.TokensUsed != 0 {
		t.Fatalf("project default: %d/%d", task.TokensUsed, task.TokenBudget)
	}
	p := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]\npolicy: {token_budget: 5000}\n", nil)
	p.request("GET", "/api/v1/tasks/"+p.create(), nil, 200, &task)
	if task.TokenBudget != 5000 {
		t.Fatalf("pipeline policy: %d", task.TokenBudget)
	}
}

func TestUsageCountsEveryTokenKind(t *testing.T) {
	h := newHarness(t, gated, nil)
	h.tasks.Coordinator = nil
	id := h.create()
	ctx := context.Background()
	if e := h.db.WithTx(ctx, func(tx *store.Tx) error {
		return h.emitter.Emit(ctx, tx, id, "a", "a", "usage", event.UsagePayload{Model: "m", Usage: llm.Usage{Input: 1, Output: 20, CacheRead: 300, CacheWrite: 4000}})
	}); e != nil {
		t.Fatal(e)
	}
	var task app.TaskResponse
	h.request("GET", "/api/v1/tasks/"+id, nil, 200, &task)
	if task.TokensUsed != 4321 {
		t.Fatalf("tokens_used=%d", task.TokensUsed)
	}
	var page app.Page[app.TaskResponse]
	h.request("GET", "/api/v1/tasks", nil, 200, &page)
	if len(page.Items) != 1 || page.Items[0].TokensUsed != 4321 {
		t.Fatalf("list: %+v", page.Items)
	}
}

// 10-PRD NFR-04, 16-구현-로드맵 Phase 3 DoD 5, 15 §3 run.paused{reason:"budget"} and §7 BUDGET_EXCEEDED.
func TestBudgetExceededPausesUntilIncreased(t *testing.T) {
	h := newHarness(t, "version: 1\nstages: [{id: a, agent: a}]\npolicy: {token_budget: 1}\n", nil)
	id := h.create()
	h.decide(h.pending(id, "a", "plan", 0), "approve", nil)
	h.status(id, "paused")
	p := h.payloads(id)
	if len(p["run.paused"]) != 1 || p["run.paused"][0]["reason"] != "budget" || len(p["run.failed"]) != 0 {
		t.Fatalf("paused=%v failed=%v", p["run.paused"], p["run.failed"])
	}
	stages, _ := h.db.Stages(context.Background(), id)
	if stages[0].Status != "executing" {
		t.Fatalf("interrupted stage must wait to run again: %+v", stages[0])
	}
	var task app.TaskResponse
	h.request("GET", "/api/v1/tasks/"+id, nil, 200, &task)
	if task.TokensUsed < task.TokenBudget {
		t.Fatalf("used %d of %d", task.TokensUsed, task.TokenBudget)
	}
	h.fails("POST", "/api/v1/tasks/"+id+"/resume", nil, 409, "BUDGET_EXCEEDED")
	h.fails("POST", "/api/v1/tasks/"+id+"/budget", map[string]any{"token_budget": 0}, 400, "BAD_REQUEST")
	h.fails("POST", "/api/v1/tasks/"+uuid.NewString()+"/budget", map[string]any{"token_budget": 10}, 404, "TASK_NOT_FOUND")
	h.request("POST", "/api/v1/tasks/"+id+"/budget", map[string]any{"token_budget": 10_000_000}, 200, &task)
	if task.TokenBudget != 10_000_000 || task.Status != "paused" {
		t.Fatalf("increase must not resume by itself: %+v", task)
	}
	h.request("POST", "/api/v1/tasks/"+id+"/resume", nil, 200, nil)
	h.decide(h.pending(id, "a", "deliverable", 0), "approve", nil)
	h.status(id, "completed")
	if p = h.payloads(id); len(p["run.paused"]) != 1 {
		t.Fatalf("paused again: %v", p["run.paused"])
	}
	h.fails("POST", "/api/v1/tasks/"+id+"/budget", map[string]any{"token_budget": 20_000_000}, 409, "INVALID_TRANSITION")
}

// A user pause within budget keeps the documented meaning: running work may finish.
func TestUserPauseWithinBudgetIsNotBudgetPause(t *testing.T) {
	h := newHarness(t, gated, nil)
	id := h.create()
	h.pending(id, "a", "plan", 0)
	h.request("POST", "/api/v1/tasks/"+id+"/pause", nil, 200, nil)
	h.request("POST", "/api/v1/tasks/"+id+"/resume", nil, 200, nil)
	h.status(id, "waiting_approval")
	if p := h.payloads(id); len(p["run.paused"]) != 1 || p["run.paused"][0]["reason"] != "user" {
		t.Fatal(p["run.paused"])
	}
}
