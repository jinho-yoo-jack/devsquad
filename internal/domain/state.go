package domain

import "strings"

type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

func (e *Error) Error() string                     { return e.Message }
func (e *Error) GetStatus() int                    { return e.Status }
func Fault(status int, code, message string) error { return &Error{status, code, message, struct{}{}} }
func Text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func Ptr[T any](v T) *T { return &v }
func Terminal(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}
func TaskTransition(status, action string) (string, error) {
	switch action {
	case "start":
		if status == "queued" {
			return "running", nil
		}
	case "pause":
		if status == "running" || status == "waiting_approval" {
			return "paused", nil
		}
	case "resume":
		if status == "paused" || status == "blocked" {
			return "running", nil
		}
	case "cancel":
		if !Terminal(status) {
			return "cancelled", nil
		}
	}
	return "", Fault(409, "INVALID_TRANSITION", "cannot "+action+" task in "+status)
}
func Ready(s StageEntity, stages []StageEntity) bool {
	for _, dep := range s.DependsOn {
		found := false
		for _, p := range stages {
			if p.StageKey == dep && p.Status == "approved" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func Derive(status string, stages []StageEntity) string {
	if Terminal(status) || status == "paused" || status == "queued" {
		return status
	}
	review, runnable, blocked, all := false, false, false, len(stages) > 0
	for _, s := range stages {
		review = review || s.Status == "plan_review" || s.Status == "deliverable_review"
		runnable = runnable || s.Status == "planning" || s.Status == "executing" || (s.Status == "pending" && Ready(s, stages))
		blocked = blocked || s.Status == "blocked"
		all = all && s.Status == "approved"
	}
	if review {
		return "waiting_approval"
	}
	if all {
		return "completed"
	}
	if runnable {
		return "running"
	}
	if blocked {
		return "blocked"
	}
	return status
}
func ValidateDecision(decision string, feedback, edited *string) error {
	switch decision {
	case "approve":
		return nil
	case "reject":
		if strings.TrimSpace(Text(feedback)) == "" {
			return Fault(400, "FEEDBACK_REQUIRED", "reject requires feedback")
		}
		return nil
	case "edit":
		if strings.TrimSpace(Text(edited)) == "" {
			return Fault(400, "EDITED_CONTENT_REQUIRED", "edit requires edited_content")
		}
		return nil
	default:
		return Fault(400, "BAD_REQUEST", "invalid decision")
	}
}
func Decide(s StageEntity, kind, decision string, feedback, edited *string, maxRetries int) (StageEntity, error) {
	if err := ValidateDecision(decision, feedback, edited); err != nil {
		return s, err
	}
	if s.Status != kind+"_review" {
		return s, Fault(409, "INVALID_TRANSITION", "stage is not awaiting this approval")
	}
	s.LastFeedback = feedback
	if decision == "reject" {
		retry := &s.PlanRetryNo
		s.Status = "planning"
		if kind == "deliverable" {
			retry = &s.DeliverableRetryNo
			s.Status = "executing"
		}
		*retry++
		s.RetryCount = s.PlanRetryNo + s.DeliverableRetryNo
		if *retry >= maxRetries {
			s.Status = "blocked"
			s.BlockedReason = Ptr(kind)
		}
	} else if kind == "plan" {
		s.Status = "executing"
		if decision == "edit" {
			s.Plan = edited
		}
	} else {
		s.Status = "approved"
		if decision == "edit" {
			s.DeliverableSummary = edited
		}
	}
	return s, nil
}
