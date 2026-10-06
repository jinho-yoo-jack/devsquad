package event

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
)

type Emitter struct {
	Bus *Bus
	Now func() time.Time
}
type UsagePayload struct {
	Model string `json:"model"`
	llm.Usage
}

func (e *Emitter) Emit(ctx context.Context, tx *store.Tx, task, stage, agent, kind string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	seq, err := tx.Q.NextSequence(ctx, task)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	// PostgreSQL stores microseconds; live and replay envelopes must agree.
	now = now.Truncate(time.Microsecond)
	ev := domain.Event{EventID: uuid.NewString(), TaskID: task, Seq: seq, TS: now, Type: kind, Payload: raw}
	if stage != "" {
		ev.StageKey = &stage
	}
	if agent != "" {
		ev.Agent = &agent
	}
	ts := pgtype.Timestamptz{Time: now, Valid: true}
	if err = tx.Q.CreateEvent(ctx, sqlc.CreateEventParams{EventID: ev.EventID, TaskID: task, Seq: seq, StageKey: ev.StageKey, Agent: ev.Agent, Type: kind, Payload: raw, Ts: ts}); err != nil {
		return err
	}
	if kind == "usage" {
		var u UsagePayload
		if err = json.Unmarshal(raw, &u); err != nil {
			return err
		}
		if err = tx.Q.CreateUsage(ctx, sqlc.CreateUsageParams{TaskID: task, StageKey: ev.StageKey, Agent: ev.Agent, Model: u.Model, InputTokens: u.Input, OutputTokens: u.Output, CacheReadTokens: u.CacheRead, CacheWriteTokens: u.CacheWrite, Ts: ts}); err != nil {
			return err
		}
	}
	tx.AfterCommit(func() {
		if e.Bus != nil {
			e.Bus.Publish(ev)
		}
	})
	return nil
}
