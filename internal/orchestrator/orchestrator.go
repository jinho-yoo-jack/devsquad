package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
	"github.com/jinho-yoo-jack/devsquad/internal/workspace"
)

type taskRun struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	running map[string]context.CancelFunc
}
type Orchestrator struct {
	Store         *store.Store
	Emitter       *event.Emitter
	Runner        stage.Runner
	Workspace     *workspace.Manager
	WorkspaceRoot string
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	tasks         map[string]*taskRun
	wg            sync.WaitGroup
	closed        bool
}

func New(ctx context.Context, s *store.Store, e *event.Emitter, r stage.Runner, w *workspace.Manager) *Orchestrator {
	ctx, cancel := context.WithCancel(ctx)
	return &Orchestrator{Store: s, Emitter: e, Runner: r, Workspace: w, ctx: ctx, cancel: cancel, tasks: map[string]*taskRun{}}
}
func (o *Orchestrator) task(id string) *taskRun {
	o.mu.Lock()
	defer o.mu.Unlock()
	if r := o.tasks[id]; r != nil {
		return r
	}
	ctx, cancel := context.WithCancel(o.ctx)
	r := &taskRun{ctx: ctx, cancel: cancel, running: map[string]context.CancelFunc{}}
	o.tasks[id] = r
	return r
}
func (o *Orchestrator) spawn(fn func()) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.ctx.Err() != nil {
		return false
	}
	o.wg.Add(1)
	go func() { defer o.wg.Done(); fn() }()
	return true
}
func (o *Orchestrator) Wake(id string) {
	o.spawn(func() {
		if e := o.Reconcile(o.ctx, id); e != nil && !errors.Is(e, context.Canceled) {
			slog.Error("[Orchestrator] Reconcile failed", "task_id", id, "error", e)
		}
	})
}
func (o *Orchestrator) Cancel(id string) { r := o.task(id); r.cancel() }
func (o *Orchestrator) Close()           { o.mu.Lock(); o.closed = true; o.cancel(); o.mu.Unlock(); o.wg.Wait() }
func (o *Orchestrator) Reconcile(ctx context.Context, id string) error {
	if o.ctx.Err() != nil {
		return o.ctx.Err()
	}
	r := o.task(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	t, e := o.Store.Task(ctx, id)
	if e != nil {
		return e
	}
	stages, e := o.Store.Stages(ctx, id)
	if e != nil {
		return e
	}
	// Approved files must be checkpointed even when the final decision completed
	// the task just before a process crash. This is outside any SQL transaction.
	if t.Status == "failed" || t.Status == "cancelled" {
		r.cancel()
		return nil
	}
	d, e := workspace.LoadDefinition(t.Workspace)
	if e != nil {
		return o.fail(ctx, t.ID, "", 0, e)
	}
	if e = json.Unmarshal(t.Pipeline, &d.Pipeline); e != nil {
		return e
	}
	for _, s := range stages {
		if s.Status == "approved" {
			if e = o.Workspace.Accept(ctx, t.Workspace, s.StageKey, d.Agents[s.Role].WritePaths); e != nil {
				return e
			}
		}
	}
	if domain.Terminal(t.Status) || t.Status == "paused" || t.Status == "blocked" {
		return nil
	}
	if t.Status == "queued" {
		if e = o.start(ctx, t.ID, d); e != nil {
			return e
		}
	}
	for _, s := range stages {
		if _, ok := r.running[s.ID]; ok {
			continue
		}
		mode := ""
		switch s.Status {
		case "pending":
			if domain.Ready(s, stages) {
				mode = "plan"
			}
		case "planning":
			mode = "plan"
		case "executing":
			mode = "execute"
		}
		if mode == "" {
			continue
		}
		claimed, e := o.claim(ctx, t.ID, s, mode)
		if errors.Is(e, store.ErrStale) {
			continue
		}
		if e != nil {
			return e
		}
		var spec pipeline.StageSpec
		for _, v := range d.Pipeline.Stages {
			if v.ID == s.StageKey {
				spec = v
				break
			}
		}
		if spec.ID == "" {
			return fmt.Errorf("missing stage in pipeline snapshot: %s", s.StageKey)
		}
		runCtx, cancel := context.WithCancel(r.ctx)
		r.running[s.ID] = cancel
		in := stage.Input{TaskID: t.ID, Command: t.Command, Mode: mode, Plan: domain.Text(claimed.Plan), Feedback: domain.Text(claimed.LastFeedback), Stage: spec, Definition: d, RetryNo: claimed.PlanRetryNo}
		if mode == "execute" {
			in.RetryNo = claimed.DeliverableRetryNo
		}
		for _, dep := range stages {
			if slices.Contains(s.DependsOn, dep.StageKey) {
				in.Inputs += fmt.Sprintf("%s: %s\n%s\n", dep.StageKey, domain.Text(dep.DeliverableRef), domain.Text(dep.DeliverableSummary))
			}
		}
		in.Emit = func(ctx context.Context, kind string, payload any) error {
			return o.progress(ctx, t.ID, claimed, kind, payload)
		}
		if !o.spawn(func() { o.run(runCtx, r, t, claimed, in) }) {
			cancel()
			delete(r.running, s.ID)
		}
	}
	return o.derive(ctx, t.ID)
}
func (o *Orchestrator) run(ctx context.Context, r *taskRun, t domain.TaskEntity, s domain.StageEntity, in stage.Input) {
	defer func() {
		r.mu.Lock()
		cancel := r.running[s.ID]
		if cancel != nil {
			cancel()
		}
		delete(r.running, s.ID)
		r.mu.Unlock()
		o.Wake(t.ID)
	}()
	var result stage.Result
	var err error
	if in.Mode == "execute" {
		err = o.Workspace.Reset(ctx, t.Workspace, in.Definition.Agents[s.Role].WritePaths)
	}
	if err == nil {
		result, err = o.Runner.Run(ctx, in)
	}
	if err == nil {
		err = o.commit(ctx, t.ID, s, in, result)
	}
	if err != nil && ctx.Err() == nil && !errors.Is(err, store.ErrStale) {
		if e := o.fail(ctx, t.ID, s.ID, s.Attempt, err); e != nil {
			slog.Error("[Orchestrator] Failure commit failed", "task_id", t.ID, "error", e)
		}
	}
}
func (o *Orchestrator) start(ctx context.Context, id string, d agentdef.Definition) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if t.Status != "queued" {
			return nil
		}
		if e = tx.SetStatus(ctx, &t, "running"); e != nil {
			return e
		}
		levels, e := d.Pipeline.Levels()
		if e != nil {
			return e
		}
		return o.Emitter.Emit(ctx, tx, id, "", "", "run.started", map[string]any{"stages": d.Pipeline.Stages, "levels": levels})
	})
}
func (o *Orchestrator) claim(ctx context.Context, id string, s domain.StageEntity, mode string) (domain.StageEntity, error) {
	e := o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if domain.Terminal(t.Status) || t.Status == "paused" || t.Status == "blocked" {
			return store.ErrStale
		}
		if s.Status == "pending" {
			all, e := tx.Stages(ctx, id)
			if e != nil {
				return e
			}
			if !domain.Ready(s, all) {
				return store.ErrStale
			}
		}
		to := "planning"
		if mode == "execute" {
			to = "executing"
		}
		attempt, e := tx.Q.ClaimStage(ctx, sqlc.ClaimStageParams{ID: s.ID, Attempt: s.Attempt, Status: to, OldStatus: s.Status})
		if e != nil {
			return e
		}
		if s.Status == "pending" {
			if e = o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, "stage.started", map[string]any{"role": s.Role}); e != nil {
				return e
			}
		}
		s.Status = to
		s.Attempt = attempt
		s.Version++
		return nil
	})
	return s, e
}
func (o *Orchestrator) progress(ctx context.Context, id string, s domain.StageEntity, kind string, payload any) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if domain.Terminal(t.Status) {
			return store.ErrStale
		}
		stages, e := tx.Stages(ctx, id)
		if e != nil {
			return e
		}
		current, e := store.FindStage(stages, s.ID)
		if e != nil {
			return e
		}
		if current.Attempt != s.Attempt || current.Status != s.Status {
			return store.ErrStale
		}
		return o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, kind, payload)
	})
}
func (o *Orchestrator) commit(ctx context.Context, id string, s domain.StageEntity, in stage.Input, result stage.Result) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if domain.Terminal(t.Status) {
			return store.ErrStale
		}
		old := s.Status
		kind := "plan"
		retry := s.PlanRetryNo
		s.Plan = &result.Content
		s.Status = "executing"
		if in.Mode == "execute" {
			kind = "deliverable"
			retry = s.DeliverableRetryNo
			s.Plan = domain.Ptr(in.Plan)
			s.Status = "approved"
			s.DeliverableRef = result.Ref
			s.DeliverableSummary = domain.Ptr(stage.Truncate(result.Content, 400))
		}
		gate := slices.Contains(in.Stage.Approvals, kind)
		if gate {
			s.Status = kind + "_review"
		}
		if e = tx.UpdateStage(ctx, s, old); e != nil {
			return e
		}
		did := ""
		if in.Mode == "execute" {
			did = uuid.NewString()
			if e = tx.Q.CreateDeliverable(ctx, sqlc.CreateDeliverableParams{ID: did, TaskID: id, StageID: s.ID, Uri: result.Ref, Content: &result.Content, Summary: s.DeliverableSummary}); e != nil {
				return e
			}
			if e = o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, "deliverable.produced", map[string]any{"deliverable_id": did, "kind": "markdown", "uri": result.Ref, "summary": s.DeliverableSummary, "written_paths": result.WrittenPaths, "iterations": result.Iterations, "hit_limit": result.HitLimit}); e != nil {
				return e
			}
		}
		if gate {
			aid := uuid.NewString()
			title := fmt.Sprintf("%s %s 승인 (v%d)", s.StageKey, kind, retry+1)
			if e = tx.Q.CreateApproval(ctx, sqlc.CreateApprovalParams{ID: aid, TaskID: id, StageID: s.ID, Kind: kind, RetryNo: retry, Title: title, Content: result.Content}); e != nil {
				return e
			}
			if e = o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, "approval.requested", map[string]any{"approval_id": aid, "kind": kind, "retry_no": retry, "title": title, "content": result.Content, "content_preview": stage.Truncate(result.Content, 1500), "deliverable_id": optionalID(did)}); e != nil {
				return e
			}
		} else if in.Mode == "execute" {
			if e = o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, "stage.completed", map[string]any{"deliverable_ref": s.DeliverableRef, "deliverable_id": did}); e != nil {
				return e
			}
		}
		return o.deriveTx(ctx, tx, &t)
	})
}
func (o *Orchestrator) derive(ctx context.Context, id string) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		return o.deriveTx(ctx, tx, &t)
	})
}
func (o *Orchestrator) deriveTx(ctx context.Context, tx *store.Tx, t *domain.TaskEntity) error {
	stages, e := tx.Stages(ctx, t.ID)
	if e != nil {
		return e
	}
	to := domain.Derive(t.Status, stages)
	if to == t.Status {
		return nil
	}
	if e = tx.SetStatus(ctx, t, to); e != nil {
		return e
	}
	if to == "completed" {
		return o.Emitter.Emit(ctx, tx, t.ID, "", "", "run.completed", struct{}{})
	}
	return nil
}
func (o *Orchestrator) fail(ctx context.Context, id, stageID string, attempt int, cause error) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if domain.Terminal(t.Status) {
			return nil
		}
		key := ""
		if stageID != "" {
			stages, e := tx.Stages(ctx, id)
			if e != nil {
				return e
			}
			s, e := store.FindStage(stages, stageID)
			if e != nil {
				return e
			}
			if s.Attempt != attempt {
				return store.ErrStale
			}
			key = s.StageKey
		}
		if e = tx.SetStatus(ctx, &t, "failed"); e != nil {
			return e
		}
		if e = o.Emitter.Emit(ctx, tx, id, key, "", "run.failed", map[string]any{"error": cause.Error(), "node": key}); e != nil {
			return e
		}
		tx.AfterCommit(func() { o.Cancel(id) })
		return nil
	})
}
func (o *Orchestrator) Recover(ctx context.Context) error {
	tasks, e := o.Store.Tasks(ctx, nil, nil)
	if e != nil {
		return e
	}
	known := map[string]bool{}
	var errs []error
	for _, t := range tasks {
		known[t.ID] = true
		if t.Status == "failed" || t.Status == "cancelled" {
			continue
		}
		if e = o.Reconcile(ctx, t.ID); e != nil {
			errs = append(errs, e)
		}
	}
	if o.WorkspaceRoot != "" {
		if e = o.Workspace.Cleanup(o.WorkspaceRoot, known, time.Now()); e != nil {
			errs = append(errs, e)
		}
	}
	return errors.Join(errs...)
}
func (o *Orchestrator) StartRecovery(interval time.Duration) {
	o.spawn(func() {
		recoveryLoop(o.ctx, interval, func() {
			if e := o.Recover(o.ctx); e != nil && o.ctx.Err() == nil {
				slog.Error("[Recovery] Reconcile failed", "error", e)
			}
		})
	})
}
func recoveryLoop(ctx context.Context, interval time.Duration, recover func()) {
	for ctx.Err() == nil {
		recover()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func optionalID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
