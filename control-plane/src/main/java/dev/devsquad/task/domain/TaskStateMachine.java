package dev.devsquad.task.domain;

import static dev.devsquad.task.domain.TaskStatus.*;

import java.util.EnumSet;
import java.util.Set;

/**
 * Task 상태 전이 — 순수 함수. JPA/트랜잭션을 모른다.
 *
 * <pre>
 * queued ─run→ running ─approval.requested→ waiting_approval
 * waiting_approval ─approve|edit→ running
 * waiting_approval ─reject(retry<max)→ running ; ─reject(retry≥max)→ blocked
 * running ─pause→ paused ─resume→ running
 * running ─fail→ failed ; ─complete→ completed
 * blocked ─manualResume→ running
 * {waiting_approval, running, paused, queued, blocked} ─cancel→ cancelled
 * </pre>
 */
public final class TaskStateMachine {

    private TaskStateMachine() {}

    public enum Event {
        RUN, APPROVAL_REQUESTED, APPROVED, REJECTED, REJECTED_OVER_LIMIT, PAUSE, RESUME,
        MANUAL_RESUME, FAIL, COMPLETE, CANCEL
    }

    private static final Set<TaskStatus> CANCELLABLE = EnumSet.of(QUEUED, RUNNING, WAITING_APPROVAL, PAUSED, BLOCKED);

    public static TaskStatus next(TaskStatus from, Event event) {
        TaskStatus to = switch (event) {
            case RUN -> from == QUEUED ? RUNNING : null;
            case APPROVAL_REQUESTED -> from == RUNNING ? WAITING_APPROVAL : null;
            case APPROVED, REJECTED -> from == WAITING_APPROVAL ? RUNNING : null;
            case REJECTED_OVER_LIMIT -> from == WAITING_APPROVAL ? BLOCKED : null;
            case PAUSE -> from == RUNNING ? PAUSED : null;
            case RESUME -> from == PAUSED ? RUNNING : null;
            case MANUAL_RESUME -> from == BLOCKED ? RUNNING : null;
            case FAIL -> (from == RUNNING || from == QUEUED) ? FAILED : null;
            case COMPLETE -> from == RUNNING ? COMPLETED : null;
            case CANCEL -> CANCELLABLE.contains(from) ? CANCELLED : null;
        };
        if (to == null) throw new InvalidTransitionException(from, event.name());
        return to;
    }

    public static boolean canTransition(TaskStatus from, Event event) {
        try {
            next(from, event);
            return true;
        } catch (InvalidTransitionException e) {
            return false;
        }
    }
}
