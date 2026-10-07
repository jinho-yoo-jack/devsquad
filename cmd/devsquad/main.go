package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go/v3/option"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/auth"
	"github.com/jinho-yoo-jack/devsquad/internal/config"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/httpapi"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/anthropic"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/fake"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/openai"
	"github.com/jinho-yoo-jack/devsquad/internal/notify"
	"github.com/jinho-yoo-jack/devsquad/internal/orchestrator"
	"github.com/jinho-yoo-jack/devsquad/internal/scm"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/workspace"
	"github.com/jinho-yoo-jack/devsquad/internal/ws"
)

func main() {
	if e := run(); e != nil {
		slog.Error("[DevSquad] Service stopped", "error", e)
		os.Exit(1)
	}
}
func run() error {
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, e := pgxpool.New(ctx, cfg.DBURL)
	if e != nil {
		return e
	}
	defer pool.Close()
	if e = pool.Ping(ctx); e != nil {
		return e
	}
	instance, e := store.AcquireInstance(ctx, pool)
	if e != nil {
		return e
	}
	defer func() { _ = instance.Conn().Close(context.Background()); instance.Release() }()
	if e = store.Migrate(ctx, pool); e != nil {
		return e
	}
	registry := llm.Registry{ForceFake: cfg.FakeLLM, Fake: fake.LLM{}, Providers: map[string]llm.Factory{
		"anthropic": func(m string) llm.LLM { return anthropic.New(m) },
		"openai":    func(m string) llm.LLM { return openai.New(m) },
		"ollama": func(m string) llm.LLM {
			a := openai.New(m, option.WithBaseURL(cfg.OllamaURL), option.WithAPIKey("ollama"))
			a.Provider = "ollama"
			return a
		},
	}}
	db := store.New(pool)
	bus := event.NewBus()
	emitter := &event.Emitter{Bus: bus}
	budgets := &app.BudgetService{Store: db, Emitter: emitter}
	engine := orchestrator.New(ctx, db, emitter, &stage.AgentRunner{Registry: registry, Budget: budgets, TestTimeout: cfg.TestTimeout}, &workspace.Manager{})
	defer engine.Close()
	engine.WorkspaceRoot = cfg.WorkspaceRoot
	hub := ws.New(db, bus)
	defer hub.Close()
	projects := &app.ProjectService{Store: db}
	tasks := &app.TaskService{Store: db, Emitter: emitter, Coordinator: engine, Registry: registry, WorkspaceRoot: cfg.WorkspaceRoot}
	if cfg.GitHubToken != "" {
		github := &scm.GitHub{Token: cfg.GitHubToken, APIURL: cfg.GitHubAPIURL, GitURL: cfg.GitHubURL}
		tasks.SCM, engine.SCM = github, github
	}
	approvals := &app.ApprovalService{Store: db, Emitter: emitter, Coordinator: engine}
	failures := prometheus.NewCounter(prometheus.CounterOpts{Name: "devsquad_notifier_failures_total", Help: "Notification failures and subscriber overflows."})
	prometheus.MustRegister(failures)
	var authn *auth.Authenticator
	if cfg.AuthDisabled {
		slog.Warn("[Auth] Authentication disabled; use only for local development")
	} else {
		authn = &auth.Authenticator{Password: cfg.AdminPassword, Secret: []byte(cfg.JWTSecret), TTL: cfg.JWTTTL}
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.Handler(projects, tasks, approvals, hub, promhttp.Handler(), authn), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	engine.StartRecovery(cfg.RecoveryInterval)
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-ticker.C:
				check, cancel := context.WithTimeout(gctx, 5*time.Second)
				err := instance.Conn().Ping(check)
				cancel()
				if err != nil {
					return err
				}
			}
		}
	})
	group.Go(func() error { notify.Run(gctx, bus, notify.Noop{}, failures); return nil })
	group.Go(func() error {
		slog.Info("[DevSquad] Listening", "address", cfg.HTTPAddr)
		e := server.ListenAndServe()
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	})
	group.Go(func() error {
		<-gctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		hub.Close()
		return server.Shutdown(shutdown)
	})
	return group.Wait()
}
