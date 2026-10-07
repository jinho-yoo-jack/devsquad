package app

import (
	"context"

	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
)

// BudgetService is the llm.BudgetGuard checked before every model call. An
// exhausted budget pauses the Task once; every runner then stops without failing.
type BudgetService struct {
	Store   *store.Store
	Emitter *event.Emitter
}

func (s *BudgetService) Check(ctx context.Context, id string) error {
	t, e := s.Store.Task(ctx, id)
	if e != nil {
		return e
	}
	used, e := s.Store.TokensUsed(ctx, id)
	if e != nil || used < t.TokenBudget {
		return e
	}
	exceeded := false
	// Re-check under the Task lock: the budget may have been raised meanwhile.
	e = s.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if used, e = tx.TokensUsed(ctx, id); e != nil || used < t.TokenBudget {
			return e
		}
		exceeded = true
		if t.Status != "running" && t.Status != "waiting_approval" {
			return nil
		}
		if e = tx.SetStatus(ctx, &t, "paused"); e != nil {
			return e
		}
		return s.Emitter.Emit(ctx, tx, id, "", "", "run.paused", map[string]any{"reason": "budget", "tokens_used": used, "token_budget": t.TokenBudget})
	})
	if e == nil && exceeded {
		return llm.ErrBudgetExceeded
	}
	return e
}
