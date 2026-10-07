package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/scm"
	"github.com/jinho-yoo-jack/devsquad/internal/stage"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
)

// publishError marks a push or pull request failure. The stage is blocked so
// the user can fix access and resume, instead of failing the Task.
type publishError struct{ err error }

func (e *publishError) Error() string { return e.err.Error() }
func (e *publishError) Unwrap() error { return e.err }

type branch struct {
	role, name, source string
	patterns           []string
	stages             []int
}

// publish is the built-in publisher stage (14-Backend-설계 pr Stage). It calls no
// model: approved stages become branches of the cloned repository and pull
// requests are opened or reused, so a re-run after a crash is harmless.
func (o *Orchestrator) publish(ctx context.Context, t domain.TaskEntity, in stage.Input) (stage.Result, error) {
	p, e := o.Store.Project(ctx, t.ProjectID)
	if e != nil {
		return stage.Result{}, e
	}
	report, e := o.report(ctx, t, p, in.Definition)
	if e != nil {
		return stage.Result{}, e
	}
	policy := in.Definition.Pipeline.Policy.PR
	mode, base := policy.Mode, policy.BaseBranch
	if mode == "" {
		mode = "single"
	}
	if base == "" {
		base = p.DefaultBranch
	}
	remote := o.SCM != nil && domain.Text(p.LocalPath) == ""
	if in.Mode == "plan" {
		target := fmt.Sprintf("%s/%s (base: %s, %s)", p.GithubOwner, p.GithubRepo, base, mode)
		if !remote {
			target = "없음: 원격 저장소와 연결되지 않은 프로젝트라 PR 초안만 작성합니다"
		}
		keys := []string{}
		for _, s := range report.Stages {
			keys = append(keys, s.Key)
		}
		return stage.Result{Content: fmt.Sprintf("## PR 계획\n\n- 대상: %s\n- 포함 단계: %s\n", target, strings.Join(keys, ", "))}, nil
	}
	if !remote {
		return stage.Result{Content: "## PR 생성 건너뜀\n\n원격 저장소와 연결되지 않은 프로젝트(local_path)라 PR을 만들지 않았습니다. 아래는 PR 본문 초안입니다.\n\n" + scm.Body(report)}, nil
	}
	report.WebURL = o.SCM.Web()
	branches := []branch{}
	if mode == "single" {
		all := branch{role: "all", name: "devsquad/" + t.ID, source: "HEAD"}
		for i := range report.Stages {
			all.stages = append(all.stages, i)
		}
		branches = append(branches, all)
	} else {
		for i, s := range report.Stages {
			if paths := in.Definition.Agents[s.Role].WritePaths; len(paths) > 0 {
				branches = append(branches, branch{role: s.Role, name: "devsquad/" + t.ID + "/" + s.Role, source: "refs/devsquad/approved/" + s.Key, patterns: paths, stages: []int{i}})
			}
		}
	}
	pushed := []branch{}
	for _, b := range branches {
		sha, changed, e := o.Workspace.BranchCommit(ctx, t.Workspace, b.source, b.patterns, scm.Title(t.Command, b.role))
		if e != nil {
			return stage.Result{}, &publishError{fmt.Errorf("build branch %s: %w", b.name, e)}
		}
		if !changed {
			continue
		}
		if e = o.Workspace.Push(ctx, t.Workspace, sha, b.name, o.SCM.GitHeader()); e != nil {
			return stage.Result{}, &publishError{fmt.Errorf("push %s: %w", b.name, e)}
		}
		for _, i := range b.stages {
			report.Stages[i].Branch = b.name
		}
		pushed = append(pushed, b)
	}
	if len(pushed) == 0 {
		return stage.Result{Content: "## PR 생성 건너뜀\n\n승인된 단계에 변경된 파일이 없어 PR을 만들지 않았습니다.\n"}, nil
	}
	body := scm.Body(report)
	links := strings.Builder{}
	for _, b := range pushed {
		title := scm.Title(t.Command, b.role)
		if mode == "single" {
			title = scm.Title(t.Command, "")
		}
		pr, e := o.SCM.Open(ctx, scm.PullRequest{Owner: p.GithubOwner, Repo: p.GithubRepo, Head: b.name, Base: base, Title: title, Body: body})
		if e != nil {
			return stage.Result{}, &publishError{fmt.Errorf("open pull request for %s: %w", b.name, e)}
		}
		if e = in.Emit(ctx, "pr.opened", map[string]any{"url": pr.URL, "number": pr.Number, "role": b.role, "branch": b.name, "created": pr.Created}); e != nil {
			return stage.Result{}, e
		}
		fmt.Fprintf(&links, "- [#%d %s](%s) · `%s`\n", pr.Number, b.role, pr.URL, b.name)
	}
	return stage.Result{Content: "## Pull Requests\n\n" + links.String() + "\n" + body}, nil
}

// report collects the approved stages, their deliverables and decided approvals (10-PRD FR-42).
func (o *Orchestrator) report(ctx context.Context, t domain.TaskEntity, p domain.ProjectEntity, d agentdef.Definition) (scm.Report, error) {
	out := scm.Report{TaskID: t.ID, Command: t.Command, Owner: p.GithubOwner, Repo: p.GithubRepo}
	stages, e := o.Store.Stages(ctx, t.ID)
	if e != nil {
		return out, e
	}
	approvals, e := o.Store.Approvals(ctx, &t.ID, nil)
	if e != nil {
		return out, e
	}
	for _, s := range stages {
		if s.Status != "approved" || d.Agents[s.Role].ToolProfile == "publisher" {
			continue
		}
		r := scm.StageReport{Key: s.StageKey, Role: s.Role, Ref: domain.Text(s.DeliverableRef), Summary: domain.Text(s.DeliverableSummary)}
		for _, a := range approvals {
			if a.StageID == s.ID && a.Status != "pending" {
				r.Approvals = append(r.Approvals, scm.ApprovalReport{Kind: a.Kind, RetryNo: a.RetryNo, Status: a.Status, DecidedBy: domain.Text(a.DecidedBy), DecidedAt: a.DecidedAt, Feedback: domain.Text(a.DecisionFeedback)})
			}
		}
		out.Stages = append(out.Stages, r)
	}
	return out, nil
}

// block parks the publisher stage; resume returns it to executing (TaskService.UpdateTask).
func (o *Orchestrator) block(ctx context.Context, id string, s domain.StageEntity, cause error) error {
	return o.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, id)
		if e != nil || domain.Terminal(t.Status) {
			return e
		}
		old := s.Status
		s.Status, s.BlockedReason = "blocked", domain.Ptr("publish")
		if e = tx.UpdateStage(ctx, s, old); e != nil {
			return e
		}
		if e = o.Emitter.Emit(ctx, tx, id, s.StageKey, s.Role, "stage.blocked", map[string]any{"reason": "publish_failed", "error": cause.Error(), "last_feedback": s.LastFeedback}); e != nil {
			return e
		}
		return o.deriveTx(ctx, tx, &t)
	})
}
