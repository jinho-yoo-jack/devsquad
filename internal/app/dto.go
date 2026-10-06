package app

import (
	"encoding/json"
	"time"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
)

type Event = domain.Event
type ProjectEntity = domain.ProjectEntity
type ApprovalEntity struct {
	ID               string     `json:"id"`
	TaskID           string     `json:"task_id"`
	StageID          string     `json:"stage_id"`
	Kind             string     `json:"kind"`
	RetryNo          int        `json:"retry_no"`
	Status           string     `json:"status"`
	Title            string     `json:"title"`
	Content          *string    `json:"content"`
	DecisionFeedback *string    `json:"decision_feedback"`
	EditedContent    *string    `json:"edited_content"`
	DecidedBy        *string    `json:"decided_by"`
	DecidedVia       *string    `json:"decided_via"`
	RequestedAt      time.Time  `json:"requested_at"`
	DecidedAt        *time.Time `json:"decided_at"`
}

type CreateProjectRequest struct {
	Name           string  `json:"name"`
	GithubOwner    string  `json:"github_owner"`
	GithubRepo     string  `json:"github_repo"`
	DefaultBranch  string  `json:"default_branch,omitempty"`
	InstallationID *int64  `json:"installation_id,omitempty"`
	LocalPath      *string `json:"local_path,omitempty"`
}
type StageResponse struct {
	Key        string   `json:"key"`
	Role       string   `json:"role"`
	Status     string   `json:"status"`
	DependsOn  []string `json:"depends_on"`
	RetryCount int      `json:"retry_count"`
}
type PendingApprovalResponse struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	StageKey    string    `json:"stage_key"`
	Title       string    `json:"title"`
	RequestedAt time.Time `json:"requested_at"`
}
type LastEventResponse struct {
	Seq  int64     `json:"seq"`
	Type string    `json:"type"`
	TS   time.Time `json:"ts"`
}
type TaskResponse struct {
	ID               string                    `json:"id"`
	ProjectID        string                    `json:"project_id"`
	Command          string                    `json:"command"`
	Status           string                    `json:"status"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
	DiscordThreadID  *string                   `json:"discord_thread_id"`
	Stages           []StageResponse           `json:"stages"`
	PendingApprovals []PendingApprovalResponse `json:"pending_approvals"`
	LastEvent        *LastEventResponse        `json:"last_event"`
}
type CreateTaskRequest struct {
	ProjectID string          `json:"project_id"`
	Command   string          `json:"command"`
	Options   json.RawMessage `json:"options,omitempty"`
}
type DecisionRequest struct {
	StageKey      string  `json:"stage_key,omitempty"`
	Kind          string  `json:"kind,omitempty"`
	RetryNo       int     `json:"retry_no,omitempty"`
	ApprovalID    string  `json:"approval_id,omitempty"`
	Decision      string  `json:"decision"`
	Feedback      *string `json:"feedback,omitempty"`
	EditedContent *string `json:"edited_content,omitempty"`
}
type DecidedResponse struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	DecidedAt  *time.Time `json:"decided_at"`
	DecidedVia *string    `json:"decided_via"`
}
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}
type EventPage struct {
	Items        []Event `json:"items"`
	NextAfterSeq *int64  `json:"next_after_seq"`
}
