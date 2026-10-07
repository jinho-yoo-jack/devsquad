package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/scm"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
	"github.com/jinho-yoo-jack/devsquad/internal/tools"
	"github.com/jinho-yoo-jack/devsquad/internal/workspace"
)

type Coordinator interface {
	Wake(string)
	Cancel(string)
}
type TaskService struct {
	Store         *store.Store
	Emitter       *event.Emitter
	Coordinator   Coordinator
	Registry      llm.Registry
	WorkspaceRoot string
	SCM           scm.Publisher
}

func (s *TaskService) CreateTask(ctx context.Context, r CreateTaskRequest, user string) (TaskResponse, error) {
	var out TaskResponse
	if _, e := uuid.Parse(r.ProjectID); e != nil {
		return out, domain.Fault(400, "BAD_REQUEST", "project_id must be a UUID")
	}
	if strings.TrimSpace(r.Command) == "" {
		return out, domain.Fault(400, "BAD_REQUEST", "command is required")
	}
	p, e := s.Store.Project(ctx, r.ProjectID)
	if e != nil {
		return out, e
	}
	id := uuid.NewString()
	dir, e := s.prepare(ctx, id, p)
	if e != nil {
		return out, domain.Fault(400, "PROJECT_INVALID", e.Error())
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(dir)
		}
	}()
	d, e := agentdef.LoadDefinition(dir, p.ContextPath)
	if e != nil {
		return out, domain.Fault(400, "PIPELINE_INVALID", e.Error())
	}
	for _, st := range d.Pipeline.Stages {
		available := (tools.Sandbox{Agent: d.Agents[st.Agent]}).Tools()
		for _, name := range st.Tools {
			found := false
			for _, tool := range available {
				if tool.Name == name {
					found = true
				}
			}
			if !found {
				return out, domain.Fault(400, "PIPELINE_INVALID", "tool is not allowed for stage: "+name)
			}
		}
		if e = s.Registry.Validate(stage.Model(d, st)); e != nil {
			return out, domain.Fault(400, "PIPELINE_INVALID", e.Error())
		}
	}
	if e = workspace.SaveDefinition(dir, d); e != nil {
		return out, e
	}
	raw, e := json.Marshal(d.Pipeline)
	if e != nil {
		return out, e
	}
	budget := p.TokenBudget
	if d.Pipeline.Policy.TokenBudget != nil {
		budget = *d.Pipeline.Policy.TokenBudget
	}
	e = s.Store.WithTx(ctx, func(tx *store.Tx) error {
		if e := tx.Q.CreateTask(ctx, sqlc.CreateTaskParams{ID: id, ProjectID: r.ProjectID, Command: strings.TrimSpace(r.Command), CreatedBy: user, Pipeline: raw, Workspace: &dir, TokenBudget: budget}); e != nil {
			return e
		}
		for _, st := range d.Pipeline.Stages {
			if e := tx.Q.CreateStage(ctx, sqlc.CreateStageParams{ID: uuid.NewString(), TaskID: id, StageKey: st.ID, Role: st.Agent, DependsOn: st.DependsOn}); e != nil {
				return e
			}
		}
		tx.AfterCommit(func() {
			committed = true
			if s.Coordinator != nil {
				s.Coordinator.Wake(id)
			}
		})
		return nil
	})
	if e != nil {
		var uncertain *store.CommitError
		if errors.As(e, &uncertain) {
			committed = true
		}
		return out, e
	}
	return s.FetchTask(ctx, id)
}

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// prepare copies a local_path project. Without one, the GitHub repository is
// cloned so approved work can return to it as branches and pull requests.
func (s *TaskService) prepare(ctx context.Context, id string, p ProjectEntity) (string, error) {
	if local := domain.Text(p.LocalPath); local != "" {
		return workspace.Prepare(ctx, s.WorkspaceRoot, id, local)
	}
	if s.SCM == nil {
		return "", fmt.Errorf("project.local_path is required unless DEVSQUAD_GITHUB_TOKEN is set")
	}
	for _, name := range []string{p.GithubOwner, p.GithubRepo} {
		if !repoName.MatchString(name) || strings.Trim(name, ".") == "" {
			return "", fmt.Errorf("invalid GitHub owner or repository: %q", name)
		}
	}
	return workspace.Clone(ctx, s.WorkspaceRoot, id, s.SCM.RemoteURL(p.GithubOwner, p.GithubRepo), p.DefaultBranch, s.SCM.GitHeader())
}
func (s *TaskService) FetchTask(ctx context.Context, id string) (TaskResponse, error) {
	t, e := s.Store.Task(ctx, id)
	if e != nil {
		return TaskResponse{}, e
	}
	return s.response(ctx, t)
}
func (s *TaskService) response(ctx context.Context, t domain.TaskEntity) (TaskResponse, error) {
	out := TaskResponse{ID: t.ID, ProjectID: t.ProjectID, Command: t.Command, Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DiscordThreadID: t.DiscordThreadID, Stages: []StageResponse{}, PendingApprovals: []PendingApprovalResponse{}, TokenBudget: t.TokenBudget}
	stages, e := s.Store.Stages(ctx, t.ID)
	if e != nil {
		return out, e
	}
	if out.TokensUsed, e = s.Store.TokensUsed(ctx, t.ID); e != nil {
		return out, e
	}
	keys := map[string]string{}
	for _, st := range stages {
		out.Stages = append(out.Stages, StageResponse{st.StageKey, st.Role, st.Status, st.DependsOn, st.RetryCount})
		keys[st.ID] = st.StageKey
	}
	approvals, e := s.Store.Approvals(ctx, &t.ID, domain.Ptr("pending"))
	if e != nil {
		return out, e
	}
	for _, a := range approvals {
		out.PendingApprovals = append(out.PendingApprovals, PendingApprovalResponse{a.ID, a.Kind, keys[a.StageID], a.Title, a.RequestedAt})
	}
	if t.EventSeq > 0 {
		evs, e := s.Store.Events(ctx, t.ID, t.EventSeq-1, 1)
		if e != nil {
			return out, e
		}
		if len(evs) > 0 {
			ev := evs[0]
			out.LastEvent = &LastEventResponse{ev.Seq, ev.Type, ev.TS}
		}
	}
	return out, nil
}
func (s *TaskService) FetchTasks(ctx context.Context, project *string, statuses []string) (Page[TaskResponse], error) {
	out := Page[TaskResponse]{Items: []TaskResponse{}}
	allowed := ",queued,running,waiting_approval,paused,blocked,completed,failed,cancelled,"
	for i, v := range statuses {
		statuses[i] = strings.ToLower(v)
		if !strings.Contains(allowed, ","+statuses[i]+",") {
			return out, domain.Fault(400, "BAD_REQUEST", "unknown task status")
		}
	}
	list, e := s.Store.Tasks(ctx, project, statuses)
	if e != nil {
		return out, e
	}
	for _, t := range list {
		dto, e := s.response(ctx, t)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, dto)
	}
	return out, nil
}
func (s *TaskService) UpdateTask(ctx context.Context, id, action string) (TaskResponse, error) {
	// The row lock serializes this with the orchestrator; the transition is
	// validated against the locked status, not an earlier read.
	e := s.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		to, e := domain.TaskTransition(t.Status, action)
		if e != nil {
			return e
		}
		if action == "resume" {
			used, e := tx.TokensUsed(ctx, id)
			if e != nil {
				return e
			}
			if used >= t.TokenBudget {
				return domain.Fault(409, "BUDGET_EXCEEDED", "token budget is exhausted; increase it before resuming")
			}
		}
		if action == "resume" && t.Status == "blocked" {
			stages, e := tx.Stages(ctx, id)
			if e != nil {
				return e
			}
			for _, st := range stages {
				if st.Status != "blocked" {
					continue
				}
				st.Status = "planning"
				if reason := domain.Text(st.BlockedReason); reason == "deliverable" || reason == "publish" {
					st.Status = "executing"
				}
				st.BlockedReason = nil
				if e = tx.UpdateStage(ctx, st, "blocked"); e != nil {
					return e
				}
			}
		}
		if e = tx.SetStatus(ctx, &t, to); e != nil {
			return e
		}
		kind := map[string]string{"pause": "run.paused", "resume": "run.resumed", "cancel": "run.cancelled"}[action]
		payload := map[string]any{}
		if action == "pause" {
			payload["reason"] = "user"
		}
		if e = s.Emitter.Emit(ctx, tx, id, "", "", kind, payload); e != nil {
			return e
		}
		tx.AfterCommit(func() {
			if s.Coordinator != nil {
				if action == "cancel" {
					s.Coordinator.Cancel(id)
				} else if action == "resume" {
					s.Coordinator.Wake(id)
				}
			}
		})
		return nil
	})
	if e != nil {
		return TaskResponse{}, e
	}
	return s.FetchTask(ctx, id)
}

// SetBudget replaces the Task's token budget. It never resumes the Task by itself.
func (s *TaskService) SetBudget(ctx context.Context, id string, r BudgetRequest) (TaskResponse, error) {
	if r.TokenBudget < 1 {
		return TaskResponse{}, domain.Fault(400, "BAD_REQUEST", "token_budget must be positive")
	}
	e := s.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil {
			return e
		}
		if domain.Terminal(t.Status) {
			return domain.Fault(409, "INVALID_TRANSITION", "cannot change the budget of a "+t.Status+" task")
		}
		_, e = tx.Q.UpdateTaskBudget(ctx, sqlc.UpdateTaskBudgetParams{ID: id, TokenBudget: r.TokenBudget, Version: t.Version})
		return e
	})
	if e != nil {
		return TaskResponse{}, e
	}
	return s.FetchTask(ctx, id)
}
func (s *TaskService) FetchEvents(ctx context.Context, id string, after int64, limit int) (EventPage, error) {
	out := EventPage{Items: []Event{}}
	if after < 0 {
		return out, domain.Fault(400, "BAD_REQUEST", "after_seq must be nonnegative")
	}
	limit = max(1, min(limit, 1000))
	if _, e := s.Store.Task(ctx, id); e != nil {
		return out, e
	}
	list, e := s.Store.Events(ctx, id, after, limit)
	out.Items = list
	if len(list) == limit {
		out.NextAfterSeq = domain.Ptr(list[len(list)-1].Seq)
	}
	return out, e
}
