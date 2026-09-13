package dev.devsquad.task;

import static dev.devsquad.task.domain.StageStateMachine.Event.*;
import static dev.devsquad.task.domain.StageStatus.*;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import dev.devsquad.task.domain.InvalidTransitionException;
import dev.devsquad.task.domain.Stage;
import dev.devsquad.task.domain.StageStateMachine;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class StageStateMachineTest {

    @Test
    void full_cycle_with_one_rejection_each() {
        var st = new Stage(UUID.randomUUID(), UUID.randomUUID(), "planning", "planner", List.of());
        assertThat(st.getStatus()).isEqualTo(PENDING);
        st.apply(STARTED);                 assertThat(st.getStatus()).isEqualTo(PLANNING);
        assertThat(st.getStartedAt()).isNotNull();
        st.apply(PLAN_REQUESTED);          assertThat(st.getStatus()).isEqualTo(PLAN_REVIEW);
        st.apply(PLAN_REJECTED);           assertThat(st.getStatus()).isEqualTo(PLANNING);
        st.apply(PLAN_REQUESTED);
        st.apply(PLAN_APPROVED);           assertThat(st.getStatus()).isEqualTo(EXECUTING);
        st.apply(DELIVERABLE_REQUESTED);   assertThat(st.getStatus()).isEqualTo(DELIVERABLE_REVIEW);
        st.apply(DELIVERABLE_REJECTED);    assertThat(st.getStatus()).isEqualTo(EXECUTING);
        st.apply(DELIVERABLE_REQUESTED);
        st.apply(DELIVERABLE_APPROVED);    assertThat(st.getStatus()).isEqualTo(APPROVED);
        assertThat(st.getRetryCount()).isEqualTo(2);
        assertThat(st.getCompletedAt()).isNotNull();
    }

    @Test
    void stage_without_gates_completes_directly() {
        assertThat(StageStateMachine.next(PLANNING, COMPLETED)).isEqualTo(APPROVED);
        assertThat(StageStateMachine.next(EXECUTING, COMPLETED)).isEqualTo(APPROVED);
    }

    @Test
    void blocked_only_from_review_states() {
        assertThat(StageStateMachine.next(PLAN_REVIEW, BLOCKED_BY_RETRIES)).isEqualTo(BLOCKED);
        assertThatThrownBy(() -> StageStateMachine.next(EXECUTING, BLOCKED_BY_RETRIES)).isInstanceOf(InvalidTransitionException.class);
    }
}
