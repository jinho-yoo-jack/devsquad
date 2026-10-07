package domain

import (
	"encoding/json"
	"time"
)

type ProjectEntity struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	GithubOwner    string  `json:"github_owner"`
	GithubRepo     string  `json:"github_repo"`
	DefaultBranch  string  `json:"default_branch"`
	InstallationID *int64  `json:"installation_id,omitempty"`
	ContextPath    string  `json:"context_path"`
	LocalPath      *string `json:"local_path"`
	TokenBudget    int64   `json:"token_budget"`
}
type TaskEntity struct {
	Pipeline        json.RawMessage `json:"pipeline"`
	Workspace       string          `json:"workspace"`
	Version         int             `json:"version"`
	EventSeq        int64           `json:"event_seq"`
	TokenBudget     int64           `json:"token_budget"`
	ID              string          `json:"id"`
	ProjectID       string          `json:"project_id"`
	Command         string          `json:"command"`
	Status          string          `json:"status"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	DiscordThreadID *string         `json:"discord_thread_id"`
}
type StageEntity struct {
	Plan               *string  `json:"plan"`
	PlanRetryNo        int      `json:"plan_retry_no"`
	DeliverableRef     *string  `json:"deliverable_ref"`
	DeliverableSummary *string  `json:"deliverable_summary"`
	DeliverableRetryNo int      `json:"deliverable_retry_no"`
	LastFeedback       *string  `json:"last_feedback"`
	BlockedReason      *string  `json:"blocked_reason"`
	Attempt            int      `json:"attempt"`
	Version            int      `json:"version"`
	ID                 string   `json:"id"`
	TaskID             string   `json:"task_id"`
	StageKey           string   `json:"stage_key"`
	Role               string   `json:"role"`
	Status             string   `json:"status"`
	DependsOn          []string `json:"depends_on"`
	RetryCount         int      `json:"retry_count"`
}
type ApprovalEntity struct {
	Version          int        `json:"version"`
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
	ResumePending    bool       `json:"resume_pending"`
}
type Event struct {
	EventID  string          `json:"event_id"`
	TaskID   string          `json:"task_id"`
	Seq      int64           `json:"seq"`
	TS       time.Time       `json:"ts"`
	StageKey *string         `json:"stage_key"`
	Agent    *string         `json:"agent"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}
