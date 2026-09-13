package dev.devsquad.approval.app;

import dev.devsquad.approval.domain.AlreadyDecidedException;
import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.ApprovalKind;
import dev.devsquad.approval.domain.ApprovalStatus;
import dev.devsquad.approval.domain.Decision;
import dev.devsquad.approval.infra.ApprovalRepository;
import dev.devsquad.common.NotFoundException;
import dev.devsquad.runtime.AgentRuntimeClient;
import dev.devsquad.task.app.TaskService;
import dev.devsquad.task.domain.Stage;
import dev.devsquad.task.domain.StageStateMachine;
import dev.devsquad.task.domain.TaskStateMachine.Event;
import dev.devsquad.task.infra.StageRepository;
import java.time.Clock;
import java.util.List;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.orm.ObjectOptimisticLockingFailureException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.event.TransactionPhase;
import org.springframework.transaction.event.TransactionalEventListener;

/**
 * 승인 결정 — 14-Backend 설계 A.4 ApprovalService.
 *
 * decide(): 한 트랜잭션에서 Approval(pending→decided)·Stage·Task 전이를 커밋한다. 커밋 **후** (AFTER_COMMIT)
 * Agent Runtime resume 을 호출한다 — 커밋 전에 호출하면 재개된 그래프가 낸 이벤트가 아직 pending 인 Approval 을
 * 보게 된다. resume 호출이 실패하면 resume_pending 이 남아 RunRecoveryJob 이 재시도한다.
 *
 * 동시 결정(Discord 와 웹에서 동시에 클릭)은 @Version 낙관적 락으로 한쪽만 이긴다. 진 쪽은 409.
 */
@Service
public class ApprovalService {
    private static final Logger log = LoggerFactory.getLogger(ApprovalService.class);

    public record ApprovalDecided(UUID approvalId, UUID taskId, Decision decision, String actor, String via) {}

    private final ApprovalRepository approvals;
    private final StageRepository stages;
    private final TaskService taskService;
    private final AgentRuntimeClient runtime;
    private final ApplicationEventPublisher bus;
    private final Clock clock;

    public ApprovalService(ApprovalRepository approvals, StageRepository stages, TaskService taskService,
                           AgentRuntimeClient runtime, ApplicationEventPublisher bus, Clock clock) {
        this.approvals = approvals; this.stages = stages; this.taskService = taskService; this.runtime = runtime;
        this.bus = bus; this.clock = clock;
    }

    @Transactional
    public Approval decide(UUID approvalId, Decision decision, String actor, String via) {
        Approval a = approvals.findById(approvalId).orElseThrow(() -> new NotFoundException("approval", approvalId));
        a.decide(decision, actor, via, clock.instant()); // AlreadyDecidedException if not pending

        Stage stage = stages.findById(a.getStageId()).orElseThrow();
        boolean plan = a.getKind() == ApprovalKind.PLAN;
        switch (decision.kind()) {
            case APPROVE, EDIT -> stage.apply(plan ? StageStateMachine.Event.PLAN_APPROVED : StageStateMachine.Event.DELIVERABLE_APPROVED);
            case REJECT -> stage.apply(plan ? StageStateMachine.Event.PLAN_REJECTED : StageStateMachine.Event.DELIVERABLE_REJECTED);
        }
        // Task: waiting_approval → running. 반려 상한 초과는 Agent Runtime 이 stage.blocked 로 알려 준다 (EventDispatcher).
        taskService.transitionIfPossible(a.getTaskId(), decision.kind() == Decision.Kind.REJECT ? Event.REJECTED : Event.APPROVED,
            "approval " + decision.kind().name().toLowerCase() + " via " + via);

        try {
            approvals.flush(); // 낙관적 락 충돌을 여기서 드러낸다
        } catch (ObjectOptimisticLockingFailureException e) {
            throw new AlreadyDecidedException("other", clock.instant());
        }
        bus.publishEvent(new ApprovalDecided(a.getId(), a.getTaskId(), decision, actor, via));
        return a;
    }

    /** 커밋 후 Agent Runtime 재개. 실패는 로그 + resume_pending 유지 (RunRecoveryJob 재시도). */
    @TransactionalEventListener(phase = TransactionPhase.AFTER_COMMIT)
    public void onDecided(ApprovalDecided ev) {
        sendResume(ev.approvalId(), ev.taskId(), ev.decision());
    }

    public void sendResume(UUID approvalId, UUID taskId, Decision decision) {
        try {
            runtime.resume(taskId, new AgentRuntimeClient.ResumeRequest(approvalId, decision.wireValue(), decision.feedback(), decision.editedContent()));
            markResumed(approvalId);
        } catch (RuntimeException e) {
            log.warn("resume failed for approval {} (task {}): {} — will retry", approvalId, taskId, e.toString());
        }
    }

    @Transactional
    public void markResumed(UUID approvalId) {
        approvals.findById(approvalId).ifPresent(Approval::markResumed);
    }

    @Transactional(readOnly = true)
    public Approval get(UUID id) { return approvals.findById(id).orElseThrow(() -> new NotFoundException("approval", id)); }

    @Transactional(readOnly = true)
    public List<Approval> pending() { return approvals.findByStatusOrderByRequestedAtAsc(ApprovalStatus.PENDING); }

    @Transactional(readOnly = true)
    public List<Approval> ofTask(UUID taskId, ApprovalStatus status) {
        return status == null ? approvals.findByTaskId(taskId) : approvals.findByTaskIdAndStatus(taskId, status);
    }

    /** resume 이 아직 안 나간 결정들 — RunRecoveryJob 이 부른다. */
    @Transactional(readOnly = true)
    public List<Approval> resumePending() { return approvals.findByResumePendingTrue(); }

    public static Decision toDecision(String kind, String feedback, String editedContent) {
        return switch (kind == null ? "" : kind.toLowerCase()) {
            case "approve" -> Decision.approve();
            case "reject" -> Decision.reject(feedback);
            case "edit" -> Decision.edit(editedContent);
            default -> throw new IllegalArgumentException("decision 은 approve|reject|edit 중 하나여야 합니다");
        };
    }
}
