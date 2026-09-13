package dev.devsquad.task.domain;

import static dev.devsquad.task.domain.StageStatus.*;

/** Stage 상태 전이 — Agent Runtime 이벤트(stage.*, approval.*)에 따라 Control Plane 이 갱신한다. */
public final class StageStateMachine {

    private StageStateMachine() {}

    public enum Event { STARTED, PLAN_REQUESTED, PLAN_APPROVED, PLAN_REJECTED, DELIVERABLE_REQUESTED,
        DELIVERABLE_APPROVED, DELIVERABLE_REJECTED, COMPLETED, BLOCKED_BY_RETRIES }

    public static StageStatus next(StageStatus from, Event event) {
        StageStatus to = switch (event) {
            case STARTED -> from == PENDING ? PLANNING : null;
            case PLAN_REQUESTED -> from == PLANNING ? PLAN_REVIEW : null;
            case PLAN_APPROVED -> from == PLAN_REVIEW ? EXECUTING : null;
            case PLAN_REJECTED -> from == PLAN_REVIEW ? PLANNING : null;
            case DELIVERABLE_REQUESTED -> from == EXECUTING ? DELIVERABLE_REVIEW : null;
            case DELIVERABLE_APPROVED -> from == DELIVERABLE_REVIEW ? APPROVED : null;
            case DELIVERABLE_REJECTED -> from == DELIVERABLE_REVIEW ? EXECUTING : null;
            // approvals 에 gate 가 없는 Stage 는 PLANNING/EXECUTING 에서 바로 완료될 수 있다
            case COMPLETED -> (from == EXECUTING || from == DELIVERABLE_REVIEW || from == PLANNING) ? APPROVED : null;
            case BLOCKED_BY_RETRIES -> (from == PLAN_REVIEW || from == DELIVERABLE_REVIEW) ? BLOCKED : null;
        };
        if (to == null) throw new InvalidTransitionException(from, event.name());
        return to;
    }
}
