package app

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
	"github.com/jinho-yoo-jack/devsquad/internal/store/sqlc"
)

type ApprovalService struct {
	Store       *store.Store
	Emitter     *event.Emitter
	Coordinator Coordinator
}

func (s *ApprovalService) FetchApproval(ctx context.Context, id string) (ApprovalEntity, error) {
	a, e := s.Store.Approval(ctx, id)
	return approvalResponse(a), e
}
func (s *ApprovalService) FetchApprovals(ctx context.Context, task, status *string, preview bool) ([]ApprovalEntity, error) {
	if status != nil {
		v := strings.ToLower(*status)
		if v != "pending" && v != "approved" && v != "rejected" && v != "edited" {
			return nil, domain.Fault(400, "BAD_REQUEST", "unknown approval status")
		}
		status = &v
	}
	if task != nil {
		if _, e := s.Store.Task(ctx, *task); e != nil {
			return nil, e
		}
	}
	list, e := s.Store.Approvals(ctx, task, status)
	if preview {
		for i := range list {
			list[i].Content = nil
			list[i].EditedContent = nil
		}
	}
	out := make([]ApprovalEntity, 0, len(list))
	for _, a := range list {
		out = append(out, approvalResponse(a))
	}
	return out, e
}
func (s *ApprovalService) UpdateApproval(ctx context.Context, id string, r DecisionRequest, user string) (DecidedResponse, error) {
	out := DecidedResponse{}
	r.Decision = strings.ToLower(r.Decision)
	if e := domain.ValidateDecision(r.Decision, r.Feedback, r.EditedContent); e != nil {
		return out, e
	}
	a, e := s.Store.Approval(ctx, id)
	if e != nil {
		return out, e
	}
	e = s.Store.WithTx(ctx, func(tx *store.Tx) error {
		t, e := tx.LockTask(ctx, a.TaskID)
		if e != nil {
			return e
		}
		status := map[string]string{"approve": "approved", "reject": "rejected", "edit": "edited"}[r.Decision]
		n, e := tx.Q.DecideApproval(ctx, sqlc.DecideApprovalParams{ID: id, Status: status, DecisionFeedback: r.Feedback, EditedContent: r.EditedContent, DecidedBy: &user, Version: a.Version})
		if e != nil {
			return e
		}
		if n != 1 {
			return domain.Fault(409, "APPROVAL_ALREADY_DECIDED", "approval already decided")
		}
		if domain.Terminal(t.Status) {
			return domain.Fault(409, "INVALID_TRANSITION", "task is terminal")
		}
		stages, e := tx.Stages(ctx, t.ID)
		if e != nil {
			return e
		}
		old, e := store.FindStage(stages, a.StageID)
		if e != nil {
			return e
		}
		var p pipeline.Pipeline
		if e = json.Unmarshal(t.Pipeline, &p); e != nil {
			return e
		}
		st, e := domain.Decide(old, a.Kind, r.Decision, r.Feedback, r.EditedContent, p.Policy.MaxRetries)
		if e != nil {
			return e
		}
		if e = tx.UpdateStage(ctx, st, old.Status); e != nil {
			return e
		}
		if e = s.Emitter.Emit(ctx, tx, t.ID, st.StageKey, st.Role, "approval.decided", map[string]any{"approval_id": id, "kind": a.Kind, "decision": r.Decision, "retry_no": a.RetryNo, "feedback": r.Feedback, "edited_content": r.EditedContent, "decided_by": user, "decided_via": "web"}); e != nil {
			return e
		}
		if st.Status == "blocked" {
			if e = s.Emitter.Emit(ctx, tx, t.ID, st.StageKey, st.Role, "stage.blocked", map[string]any{"reason": "max_retries", "kind": a.Kind, "retry_no": st.RetryCount, "last_feedback": st.LastFeedback}); e != nil {
				return e
			}
		}
		if st.Status == "approved" {
			did, e := tx.Q.LatestDeliverableID(ctx, st.ID)
			if e != nil {
				return e
			}
			if e = s.Emitter.Emit(ctx, tx, t.ID, st.StageKey, st.Role, "stage.completed", map[string]any{"deliverable_ref": st.DeliverableRef, "deliverable_id": did}); e != nil {
				return e
			}
		}
		stages, e = tx.Stages(ctx, t.ID)
		if e != nil {
			return e
		}
		to := domain.Derive(t.Status, stages)
		if to != t.Status {
			if e = tx.SetStatus(ctx, &t, to); e != nil {
				return e
			}
			if to == "completed" {
				payload, e := tx.Completion(ctx, t.ID)
				if e != nil {
					return e
				}
				if e = s.Emitter.Emit(ctx, tx, t.ID, "", "", "run.completed", payload); e != nil {
					return e
				}
			}
		}
		decided, e := tx.Approval(ctx, id)
		if e != nil {
			return e
		}
		out = DecidedResponse{decided.ID, decided.Status, decided.DecidedAt, decided.DecidedVia}
		tx.AfterCommit(func() {
			if s.Coordinator != nil {
				s.Coordinator.Wake(t.ID)
			}
		})
		return nil
	})
	return out, e
}

func approvalResponse(a domain.ApprovalEntity) ApprovalEntity {
	return ApprovalEntity{ID: a.ID, TaskID: a.TaskID, StageID: a.StageID, Kind: a.Kind, RetryNo: a.RetryNo, Status: a.Status, Title: a.Title, Content: a.Content, DecisionFeedback: a.DecisionFeedback, EditedContent: a.EditedContent, DecidedBy: a.DecidedBy, DecidedVia: a.DecidedVia, RequestedAt: a.RequestedAt, DecidedAt: a.DecidedAt}
}
