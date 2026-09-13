package dev.devsquad.approval.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "approval")
public class Approval {
    @Id private UUID id;
    @Column(name = "task_id", nullable = false) private UUID taskId;
    @Column(name = "stage_id", nullable = false) private UUID stageId;
    @Enumerated(EnumType.STRING) @Column(nullable = false) private ApprovalKind kind;
    @Column(name = "retry_no", nullable = false) private int retryNo;
    @Enumerated(EnumType.STRING) @Column(nullable = false) private ApprovalStatus status = ApprovalStatus.PENDING;
    @Column(nullable = false) private String title;
    @Column(nullable = false, columnDefinition = "text") private String content;
    @Column(name = "decision_feedback", columnDefinition = "text") private String decisionFeedback;
    @Column(name = "edited_content", columnDefinition = "text") private String editedContent;
    @Column(name = "decided_by") private String decidedBy;
    @Column(name = "decided_via") private String decidedVia;
    @Column(name = "resume_pending", nullable = false) private boolean resumePending;
    @Column(name = "requested_at", nullable = false, updatable = false) private Instant requestedAt = Instant.now();
    @Column(name = "decided_at") private Instant decidedAt;
    @Version private int version;

    protected Approval() {}

    public Approval(UUID id, UUID taskId, UUID stageId, ApprovalKind kind, int retryNo, String title, String content) {
        this.id = id; this.taskId = taskId; this.stageId = stageId; this.kind = kind; this.retryNo = retryNo;
        this.title = title; this.content = content;
    }

    /**
     * 결정은 한 번만. 이미 결정되어 있으면 AlreadyDecidedException (409).
     * 동시 결정은 @Version 낙관적 락이 두 번째 커밋을 거절한다 — 호출자가 ObjectOptimisticLockingFailureException 을
     * AlreadyDecidedException 으로 변환한다.
     */
    public void decide(Decision decision, String actor, String via, Instant now) {
        if (status != ApprovalStatus.PENDING) throw new AlreadyDecidedException(decidedVia, decidedAt);
        this.status = decision.resultingStatus();
        this.decisionFeedback = decision.feedback();
        this.editedContent = decision.editedContent();
        this.decidedBy = actor;
        this.decidedVia = via;
        this.decidedAt = now;
        this.resumePending = true; // Agent Runtime resume 호출이 성공하면 false 로
    }

    public void markResumed() { this.resumePending = false; }

    public UUID getId() { return id; }
    public UUID getTaskId() { return taskId; }
    public UUID getStageId() { return stageId; }
    public ApprovalKind getKind() { return kind; }
    public int getRetryNo() { return retryNo; }
    public ApprovalStatus getStatus() { return status; }
    public String getTitle() { return title; }
    public String getContent() { return content; }
    public String getDecisionFeedback() { return decisionFeedback; }
    public String getEditedContent() { return editedContent; }
    public String getDecidedBy() { return decidedBy; }
    public String getDecidedVia() { return decidedVia; }
    public boolean isResumePending() { return resumePending; }
    public Instant getRequestedAt() { return requestedAt; }
    public Instant getDecidedAt() { return decidedAt; }
    public boolean isPending() { return status == ApprovalStatus.PENDING; }
}
