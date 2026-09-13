package dev.devsquad.task.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

@Entity
@Table(name = "stage")
public class Stage {
    @Id private UUID id;
    @Column(name = "task_id", nullable = false) private UUID taskId;
    @Column(name = "stage_key", nullable = false) private String stageKey;
    @Column(nullable = false) private String role;
    @JdbcTypeCode(SqlTypes.ARRAY) @Column(name = "depends_on", nullable = false, columnDefinition = "text[]")
    private List<String> dependsOn = List.of();
    @Enumerated(EnumType.STRING) @Column(nullable = false) private StageStatus status = StageStatus.PENDING;
    @Column(name = "retry_count", nullable = false) private int retryCount;
    @Column(name = "started_at") private Instant startedAt;
    @Column(name = "completed_at") private Instant completedAt;

    protected Stage() {}

    public Stage(UUID id, UUID taskId, String stageKey, String role, List<String> dependsOn) {
        this.id = id; this.taskId = taskId; this.stageKey = stageKey; this.role = role; this.dependsOn = List.copyOf(dependsOn);
    }

    public StageStatus apply(StageStateMachine.Event event) {
        StageStatus before = this.status;
        this.status = StageStateMachine.next(before, event);
        if (before == StageStatus.PENDING) this.startedAt = Instant.now();
        if (this.status == StageStatus.APPROVED || this.status == StageStatus.BLOCKED) this.completedAt = Instant.now();
        if (event == StageStateMachine.Event.PLAN_REJECTED || event == StageStateMachine.Event.DELIVERABLE_REJECTED) this.retryCount++;
        return this.status;
    }

    public UUID getId() { return id; }
    public UUID getTaskId() { return taskId; }
    public String getStageKey() { return stageKey; }
    public String getRole() { return role; }
    public List<String> getDependsOn() { return dependsOn; }
    public StageStatus getStatus() { return status; }
    public int getRetryCount() { return retryCount; }
    public Instant getStartedAt() { return startedAt; }
    public Instant getCompletedAt() { return completedAt; }
}
