package store

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
)

var ErrStale = errors.New("stale stage attempt")

// A lost COMMIT acknowledgement cannot tell the caller whether PostgreSQL
// committed. Keep prepared files until recovery can consult the database.
type CommitError struct{ Err error }

func (e *CommitError) Error() string { return "commit acknowledgement failed: " + e.Err.Error() }
func (e *CommitError) Unwrap() error { return e.Err }

type Reader struct{ Q *sqlc.Queries }
type Store struct {
	Reader
	Pool *pgxpool.Pool
}
type Tx struct {
	Reader
	tx    pgx.Tx
	after []func()
}

func New(pool *pgxpool.Pool) *Store { return &Store{Reader: Reader{Q: sqlc.New(pool)}, Pool: pool} }
func (s *Store) WithTx(ctx context.Context, fn func(*Tx) error) error {
	raw, e := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return e
	}
	defer func() {
		rollback, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = raw.Rollback(rollback)
	}()
	tx := &Tx{Reader: Reader{Q: sqlc.New(raw)}, tx: raw}
	if e = fn(tx); e != nil {
		return e
	}
	if e = raw.Commit(ctx); e != nil {
		return &CommitError{Err: e}
	}
	for _, f := range tx.after {
		f()
	}
	return nil
}
func (t *Tx) AfterCommit(fn func()) { t.after = append(t.after, fn) }
func decode[T any](resource string, raw []byte, e error) (T, error) {
	var v T
	if errors.Is(e, pgx.ErrNoRows) {
		return v, domain.Fault(404, strings.ToUpper(resource)+"_NOT_FOUND", resource+" not found")
	}
	if e != nil {
		return v, e
	}
	return v, json.Unmarshal(raw, &v)
}
func decodeList[T any](raw [][]byte, e error) ([]T, error) {
	out := []T{}
	if e != nil {
		return nil, e
	}
	for _, b := range raw {
		var v T
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r Reader) Project(ctx context.Context, id string) (domain.ProjectEntity, error) {
	b, e := r.Q.GetProject(ctx, id)
	return decode[domain.ProjectEntity]("project", b, e)
}
func (r Reader) Projects(ctx context.Context) ([]domain.ProjectEntity, error) {
	b, e := r.Q.ListProjects(ctx)
	return decodeList[domain.ProjectEntity](b, e)
}
func (r Reader) Task(ctx context.Context, id string) (domain.TaskEntity, error) {
	b, e := r.Q.GetTask(ctx, id)
	return decode[domain.TaskEntity]("task", b, e)
}
func (t *Tx) LockTask(ctx context.Context, id string) (domain.TaskEntity, error) {
	b, e := t.Q.LockTask(ctx, id)
	return decode[domain.TaskEntity]("task", b, e)
}
func (r Reader) Tasks(ctx context.Context, project *string, statuses []string) ([]domain.TaskEntity, error) {
	b, e := r.Q.ListTasks(ctx, sqlc.ListTasksParams{ProjectID: project, Statuses: statuses})
	return decodeList[domain.TaskEntity](b, e)
}
func (r Reader) Stages(ctx context.Context, id string) ([]domain.StageEntity, error) {
	b, e := r.Q.ListStages(ctx, id)
	return decodeList[domain.StageEntity](b, e)
}
func (r Reader) Approval(ctx context.Context, id string) (domain.ApprovalEntity, error) {
	b, e := r.Q.GetApproval(ctx, id)
	return decode[domain.ApprovalEntity]("approval", b, e)
}
func (r Reader) Approvals(ctx context.Context, task, status *string) ([]domain.ApprovalEntity, error) {
	b, e := r.Q.ListApprovals(ctx, sqlc.ListApprovalsParams{TaskID: task, Status: status})
	return decodeList[domain.ApprovalEntity](b, e)
}

// Completion is the run.completed payload: pull requests opened for the Task, in order.
func (r Reader) Completion(ctx context.Context, id string) (map[string]any, error) {
	rows, e := r.Q.PullRequestURLs(ctx, id)
	if e != nil || len(rows) == 0 {
		return map[string]any{}, e
	}
	slices.SortFunc(rows, func(a, b sqlc.PullRequestURLsRow) int { return cmp.Compare(a.Seq, b.Seq) })
	urls := []string{}
	for _, r := range rows {
		urls = append(urls, r.Url)
	}
	return map[string]any{"pr_urls": urls}, nil
}
func (r Reader) TokensUsed(ctx context.Context, id string) (int64, error) {
	return r.Q.TaskTokensUsed(ctx, id)
}
func (r Reader) Events(ctx context.Context, id string, after int64, limit int) ([]domain.Event, error) {
	b, e := r.Q.ListEvents(ctx, sqlc.ListEventsParams{TaskID: id, Seq: after, Column3: int(limit)})
	return decodeList[domain.Event](b, e)
}
func (t *Tx) SetStatus(ctx context.Context, task *domain.TaskEntity, status string) error {
	n, e := t.Q.UpdateTaskStatus(ctx, sqlc.UpdateTaskStatusParams{ID: task.ID, Status: status, Version: task.Version})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Fault(409, "INVALID_TRANSITION", "task version changed")
	}
	task.Status = status
	task.Version++
	return nil
}
func (t *Tx) UpdateStage(ctx context.Context, s domain.StageEntity, oldStatus string) error {
	n, e := t.Q.UpdateStage(ctx, sqlc.UpdateStageParams{ID: s.ID, Attempt: s.Attempt, Status: s.Status, Plan: s.Plan, PlanRetryNo: s.PlanRetryNo, DeliverableRef: s.DeliverableRef, DeliverableSummary: s.DeliverableSummary, DeliverableRetryNo: s.DeliverableRetryNo, LastFeedback: s.LastFeedback, BlockedReason: s.BlockedReason, OldStatus: oldStatus, OldVersion: s.Version})
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrStale
	}
	return nil
}
func FindStage(stages []domain.StageEntity, id string) (domain.StageEntity, error) {
	for _, s := range stages {
		if s.ID == id {
			return s, nil
		}
	}
	return domain.StageEntity{}, fmt.Errorf("missing stage %s", id)
}
