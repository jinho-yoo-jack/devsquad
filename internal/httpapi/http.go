package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/auth"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
)

type ProjectController struct{ Service *app.ProjectService }
type TaskController struct{ Service *app.TaskService }
type ApprovalController struct{ Service *app.ApprovalService }
type Empty struct{}
type ID struct {
	ID string `path:"id" format:"uuid"`
}
type CreateProject struct{ Body app.CreateProjectRequest }
type CreateTask struct {
	User string `header:"X-User" default:"local-user"`
	Body app.CreateTaskRequest
}
type SetBudget struct {
	ID   string `path:"id" format:"uuid"`
	Body app.BudgetRequest
}
type Decide struct {
	ID   string `path:"id" format:"uuid"`
	User string `header:"X-User" default:"local-user"`
	Body app.DecisionRequest
}
type TasksQuery struct {
	Project       string   `query:"project_id" format:"uuid"`
	LegacyProject string   `query:"projectId" format:"uuid"`
	Status        []string `query:"status,explode"`
}
type ApprovalQuery struct {
	Status string `query:"status" default:"pending"`
}
type TaskApprovalQuery struct {
	ID     string `path:"id" format:"uuid"`
	Status string `query:"status"`
}
type EventsQuery struct {
	ID          string `path:"id" format:"uuid"`
	After       int64  `query:"after_seq" minimum:"0"`
	LegacyAfter int64  `query:"afterSeq" minimum:"0"`
	Limit       int    `query:"limit" default:"500"`
}
type response[T any] struct{ Body T }

var errorsOnce sync.Once

func register[I, O any](api huma.API, id, method, path string, status int, fn func(context.Context, *I) (O, error)) {
	huma.Register(api, huma.Operation{OperationID: id, Method: method, Path: path, DefaultStatus: status, MaxBodyBytes: 4 << 20, Errors: []int{400, 401, 404, 409, 500}}, func(ctx context.Context, in *I) (*response[O], error) {
		v, e := fn(ctx, in)
		if e != nil {
			var apiErr *domain.Error
			if errors.As(e, &apiErr) {
				return nil, apiErr
			}
			slog.Error("[HTTP] Request failed", "operation", id, "error", e)
			return nil, domain.Fault(500, "INTERNAL_ERROR", "internal server error")
		}
		return &response[O]{v}, nil
	})
}

// Handler serves the API. A nil authn disables authentication (local development).
func Handler(p *app.ProjectService, t *app.TaskService, a *app.ApprovalService, ws http.Handler, metrics http.Handler, authn *auth.Authenticator) http.Handler {
	errorsOnce.Do(func() {
		huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
			if status == 422 {
				status = 400
			}
			return &domain.Error{Status: status, Code: "BAD_REQUEST", Message: message, Details: struct{}{}}
		}
	})
	mux := http.NewServeMux()
	config := huma.DefaultConfig("DevSquad", "1.0.0")
	config.Transformers = nil
	api := humago.New(mux, config)
	ProjectController{p}.Register(api)
	TaskController{t}.Register(api)
	ApprovalController{a}.Register(api)
	registerMe(api)
	if authn != nil {
		AuthController{authn}.Register(api)
	}
	health := func(ctx context.Context, _ *Empty) (map[string]any, error) {
		if err := t.Store.Pool.Ping(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"status": "ok", "service": "devsquad", "ts": time.Now().UTC()}, nil
	}
	register(api, "health", "GET", "/api/v1/health", 200, health)
	register(api, "health-compat", "GET", "/actuator/health", 200, health)
	if ws != nil {
		mux.Handle("GET /ws", ws)
	}
	if metrics != nil {
		mux.Handle("GET /metrics", metrics)
	}
	if authn == nil {
		return mux
	}
	return authn.Middleware(mux)
}
func (c ProjectController) Register(api huma.API) {
	register(api, "projects-list", "GET", "/api/v1/projects", 200, func(ctx context.Context, _ *Empty) ([]app.ProjectEntity, error) { return c.Service.FetchProjects(ctx) })
	register(api, "projects-get", "GET", "/api/v1/projects/{id}", 200, func(ctx context.Context, in *ID) (app.ProjectEntity, error) {
		return c.Service.FetchProject(ctx, in.ID)
	})
	register(api, "projects-create", "POST", "/api/v1/projects", 201, func(ctx context.Context, in *CreateProject) (app.ProjectEntity, error) {
		return c.Service.CreateProject(ctx, in.Body)
	})
}
func (c TaskController) Register(api huma.API) {
	register(api, "tasks-list", "GET", "/api/v1/tasks", 200, func(ctx context.Context, in *TasksQuery) (app.Page[app.TaskResponse], error) {
		p := in.Project
		if p == "" {
			p = in.LegacyProject
		}
		var filter *string
		if p != "" {
			filter = &p
		}
		// The web client repeats status; other callers may send a comma-separated list.
		var statuses []string
		for _, v := range in.Status {
			statuses = append(statuses, strings.Split(v, ",")...)
		}
		return c.Service.FetchTasks(ctx, filter, statuses)
	})
	register(api, "tasks-get", "GET", "/api/v1/tasks/{id}", 200, func(ctx context.Context, in *ID) (app.TaskResponse, error) { return c.Service.FetchTask(ctx, in.ID) })
	register(api, "tasks-create", "POST", "/api/v1/tasks", 201, func(ctx context.Context, in *CreateTask) (app.TaskResponse, error) {
		return c.Service.CreateTask(ctx, in.Body, actor(ctx, in.User))
	})
	for _, action := range []string{"pause", "resume", "cancel"} {
		register(api, "tasks-"+action, "POST", "/api/v1/tasks/{id}/"+action, 200, func(ctx context.Context, in *ID) (app.TaskResponse, error) {
			return c.Service.UpdateTask(ctx, in.ID, action)
		})
	}
	register(api, "tasks-budget", "POST", "/api/v1/tasks/{id}/budget", 200, func(ctx context.Context, in *SetBudget) (app.TaskResponse, error) {
		return c.Service.SetBudget(ctx, in.ID, in.Body)
	})
	register(api, "tasks-events", "GET", "/api/v1/tasks/{id}/events", 200, func(ctx context.Context, in *EventsQuery) (app.EventPage, error) {
		after := in.After
		if after == 0 {
			after = in.LegacyAfter
		}
		return c.Service.FetchEvents(ctx, in.ID, after, in.Limit)
	})
}
func (c ApprovalController) Register(api huma.API) {
	register(api, "approvals-list", "GET", "/api/v1/approvals", 200, func(ctx context.Context, in *ApprovalQuery) ([]app.ApprovalEntity, error) {
		return c.Service.FetchApprovals(ctx, nil, &in.Status, true)
	})
	register(api, "approvals-get", "GET", "/api/v1/approvals/{id}", 200, func(ctx context.Context, in *ID) (app.ApprovalEntity, error) {
		return c.Service.FetchApproval(ctx, in.ID)
	})
	register(api, "approvals-decide", "POST", "/api/v1/approvals/{id}/decide", 200, func(ctx context.Context, in *Decide) (app.DecidedResponse, error) {
		return c.Service.UpdateApproval(ctx, in.ID, in.Body, actor(ctx, in.User))
	})
	register(api, "tasks-approvals", "GET", "/api/v1/tasks/{id}/approvals", 200, func(ctx context.Context, in *TaskApprovalQuery) ([]app.ApprovalEntity, error) {
		var status *string
		if strings.TrimSpace(in.Status) != "" {
			status = &in.Status
		}
		return c.Service.FetchApprovals(ctx, &in.ID, status, false)
	})
}
