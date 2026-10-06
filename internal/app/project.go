package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
)

type ProjectService struct{ Store *store.Store }

func (s *ProjectService) FetchProjects(ctx context.Context) ([]ProjectEntity, error) {
	return s.Store.Projects(ctx)
}
func (s *ProjectService) FetchProject(ctx context.Context, id string) (ProjectEntity, error) {
	return s.Store.Project(ctx, id)
}
func (s *ProjectService) CreateProject(ctx context.Context, r CreateProjectRequest) (ProjectEntity, error) {
	if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.GithubOwner) == "" || strings.TrimSpace(r.GithubRepo) == "" {
		return ProjectEntity{}, domain.Fault(400, "BAD_REQUEST", "name, github_owner and github_repo are required")
	}
	branch := strings.TrimSpace(r.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	id := uuid.NewString()
	installation := pgtype.Int8{}
	if r.InstallationID != nil {
		installation = pgtype.Int8{Int64: *r.InstallationID, Valid: true}
	}
	e := s.Store.WithTx(ctx, func(tx *store.Tx) error {
		return tx.Q.CreateProject(ctx, sqlc.CreateProjectParams{ID: id, Name: strings.TrimSpace(r.Name), GithubOwner: r.GithubOwner, GithubRepo: r.GithubRepo, DefaultBranch: branch, InstallationID: installation, LocalPath: r.LocalPath})
	})
	if e != nil {
		return ProjectEntity{}, e
	}
	return s.Store.Project(ctx, id)
}
