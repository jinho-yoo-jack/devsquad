package scm

import (
	"fmt"
	"strings"
	"time"
)

type ApprovalReport struct {
	Kind      string
	RetryNo   int
	Status    string
	DecidedBy string
	DecidedAt *time.Time
	Feedback  string
}

// StageReport.Branch is the pushed branch holding the stage's deliverable.
type StageReport struct {
	Key, Role, Branch, Ref, Summary string
	Approvals                       []ApprovalReport
}

// Report is what a pull request tells reviewers (10-PRD FR-42). Without a
// WebURL (no remote repository) deliverables are named, not linked.
type Report struct {
	TaskID, Command, Owner, Repo, WebURL string
	Stages                               []StageReport
}

func Title(command, role string) string {
	line := []rune(strings.TrimSpace(strings.SplitN(strings.TrimSpace(command), "\n", 2)[0]))
	if len(line) > 80 {
		line = append(line[:80], '…')
	}
	if role == "" {
		return "[DevSquad] " + string(line)
	}
	return fmt.Sprintf("[DevSquad] %s (%s)", string(line), role)
}
func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "|", `\|`), "\n", " ")
}
func quote(s string) string {
	return "> " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n> ")
}
func Body(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Task\n\n%s\n\nTask ID: `%s`\n\n## 단계별 산출물\n", quote(r.Command), r.TaskID)
	for _, s := range r.Stages {
		fmt.Fprintf(&b, "\n### %s (%s)\n\n", s.Key, s.Role)
		if s.Ref != "" && r.WebURL != "" && s.Branch != "" {
			fmt.Fprintf(&b, "- 산출물: [%s](%s/%s/%s/blob/%s/%s)\n", s.Ref, strings.TrimRight(r.WebURL, "/"), r.Owner, r.Repo, s.Branch, s.Ref)
		} else if s.Ref != "" {
			fmt.Fprintf(&b, "- 산출물: `%s`\n", s.Ref)
		}
		if strings.TrimSpace(s.Summary) != "" {
			fmt.Fprintf(&b, "\n%s\n", quote(s.Summary))
		}
	}
	b.WriteString("\n## 승인 이력\n\n| 단계 | 종류 | 버전 | 결정 | 결정자 | 시각 | 피드백 |\n|---|---|---|---|---|---|---|\n")
	for _, s := range r.Stages {
		for _, a := range s.Approvals {
			at := ""
			if a.DecidedAt != nil {
				at = a.DecidedAt.UTC().Format("2006-01-02 15:04 UTC")
			}
			fmt.Fprintf(&b, "| %s | %s | v%d | %s | %s | %s | %s |\n", s.Key, a.Kind, a.RetryNo+1, a.Status, cell(a.DecidedBy), at, cell(a.Feedback))
		}
	}
	b.WriteString("\n---\nDevSquad가 사람의 승인을 거친 산출물로 만든 PR입니다.\n")
	return b.String()
}
