package dev.devsquad.task;

import static dev.devsquad.task.domain.TaskStateMachine.Event.*;
import static dev.devsquad.task.domain.TaskStatus.*;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import dev.devsquad.task.domain.InvalidTransitionException;
import dev.devsquad.task.domain.TaskStateMachine;
import dev.devsquad.task.domain.TaskStatus;
import java.util.EnumSet;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

class TaskStateMachineTest {

    @Test
    void happy_path_through_two_approvals() {
        TaskStatus s = QUEUED;
        s = TaskStateMachine.next(s, RUN);                 assertThat(s).isEqualTo(RUNNING);
        s = TaskStateMachine.next(s, APPROVAL_REQUESTED);  assertThat(s).isEqualTo(WAITING_APPROVAL);
        s = TaskStateMachine.next(s, APPROVED);            assertThat(s).isEqualTo(RUNNING);
        s = TaskStateMachine.next(s, APPROVAL_REQUESTED);  assertThat(s).isEqualTo(WAITING_APPROVAL);
        s = TaskStateMachine.next(s, APPROVED);            assertThat(s).isEqualTo(RUNNING);
        s = TaskStateMachine.next(s, COMPLETE);            assertThat(s).isEqualTo(COMPLETED);
        assertThat(s.isTerminal()).isTrue();
    }

    @Test
    void reject_keeps_running_until_limit_then_blocks_and_can_be_manually_resumed() {
        assertThat(TaskStateMachine.next(WAITING_APPROVAL, REJECTED)).isEqualTo(RUNNING);
        assertThat(TaskStateMachine.next(WAITING_APPROVAL, REJECTED_OVER_LIMIT)).isEqualTo(BLOCKED);
        assertThat(TaskStateMachine.next(BLOCKED, MANUAL_RESUME)).isEqualTo(RUNNING);
    }

    @Test
    void pause_resume_and_budget() {
        assertThat(TaskStateMachine.next(RUNNING, PAUSE)).isEqualTo(PAUSED);
        assertThat(TaskStateMachine.next(PAUSED, RESUME)).isEqualTo(RUNNING);
        assertThatThrownBy(() -> TaskStateMachine.next(WAITING_APPROVAL, PAUSE)).isInstanceOf(InvalidTransitionException.class);
    }

    @ParameterizedTest
    @EnumSource(value = TaskStatus.class, names = {"QUEUED", "RUNNING", "WAITING_APPROVAL", "PAUSED", "BLOCKED"})
    void cancellable_states(TaskStatus from) {
        assertThat(TaskStateMachine.next(from, CANCEL)).isEqualTo(CANCELLED);
    }

    @ParameterizedTest
    @EnumSource(value = TaskStatus.class, names = {"COMPLETED", "FAILED", "CANCELLED"})
    void terminal_states_reject_every_event(TaskStatus from) {
        for (TaskStateMachine.Event e : EnumSet.allOf(TaskStateMachine.Event.class)) {
            assertThat(TaskStateMachine.canTransition(from, e)).as("%s --%s-->", from, e).isFalse();
        }
    }

    @Test
    void approve_from_running_is_invalid() {
        assertThatThrownBy(() -> TaskStateMachine.next(RUNNING, APPROVED))
            .isInstanceOf(InvalidTransitionException.class)
            .hasMessageContaining("RUNNING");
    }
}
