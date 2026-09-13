package dev.devsquad.approval;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import dev.devsquad.approval.domain.AlreadyDecidedException;
import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.ApprovalKind;
import dev.devsquad.approval.domain.ApprovalStatus;
import dev.devsquad.approval.domain.Decision;
import dev.devsquad.approval.domain.FeedbackRequiredException;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class ApprovalDomainTest {

    private Approval pending() {
        return new Approval(UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), ApprovalKind.PLAN, 0, "planner plan v1", "## 계획");
    }

    @Test
    void approve_sets_status_actor_channel_and_resume_pending() {
        var a = pending();
        var now = Instant.parse("2026-09-13T10:05:00Z");
        a.decide(Decision.approve(), "jinho", "web", now);
        assertThat(a.getStatus()).isEqualTo(ApprovalStatus.APPROVED);
        assertThat(a.getDecidedBy()).isEqualTo("jinho");
        assertThat(a.getDecidedVia()).isEqualTo("web");
        assertThat(a.getDecidedAt()).isEqualTo(now);
        assertThat(a.isResumePending()).isTrue();
        a.markResumed();
        assertThat(a.isResumePending()).isFalse();
    }

    @Test
    void second_decision_is_rejected_with_first_channel_info() {
        var a = pending();
        a.decide(Decision.approve(), "jinho", "discord", Instant.now());
        assertThatThrownBy(() -> a.decide(Decision.reject("x"), "jinho", "web", Instant.now()))
            .isInstanceOf(AlreadyDecidedException.class)
            .satisfies(e -> assertThat(((AlreadyDecidedException) e).decidedVia()).isEqualTo("discord"));
    }

    @Test
    void reject_requires_feedback_and_edit_requires_content() {
        assertThatThrownBy(() -> Decision.reject("  ")).isInstanceOf(FeedbackRequiredException.class);
        assertThatThrownBy(() -> Decision.edit("")).isInstanceOf(IllegalArgumentException.class);
        var a = pending();
        a.decide(Decision.edit("## 사람이 고친 계획"), "jinho", "web", Instant.now());
        assertThat(a.getStatus()).isEqualTo(ApprovalStatus.EDITED);
        assertThat(a.getEditedContent()).startsWith("## 사람이");
    }

    @Test
    void wire_values_match_agent_runtime_contract() {
        assertThat(Decision.approve().wireValue()).isEqualTo("approve");
        assertThat(Decision.reject("f").wireValue()).isEqualTo("reject");
        assertThat(Decision.edit("c").wireValue()).isEqualTo("edit");
    }
}
