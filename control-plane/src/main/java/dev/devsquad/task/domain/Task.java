package dev.devsquad.task.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "task")
public class Task {
    @Id private UUID id;
    @Column(name = "project_id", nullable = false) private UUID projectId;
    @Column(nullable = false, columnDefinition = "text") private String command;
    @Enumerated(EnumType.STRING) @Column(nullable = false) private TaskStatus status;
    @Column(name = "discord_thread_id") private String discordThreadId;
    @Column(name = "created_by", nullable = false) private String createdBy;
    @Column(name = "created_at", nullable = false, updatable = false) private Instant createdAt = Instant.now();
    @Column(name = "updated_at", nullable = false) private Instant updatedAt = Instant.now();
    @Version private int version;

    protected Task() {}

    public Task(UUID id, UUID projectId, String command, String createdBy) {
        this.id = id; this.projectId = projectId; this.command = command; this.createdBy = createdBy;
        this.status = TaskStatus.QUEUED;
    }

    /** 상태 기계를 통해서만 전이한다. */
    public TaskStatus apply(TaskStateMachine.Event event) {
        this.status = TaskStateMachine.next(this.status, event);
        this.updatedAt = Instant.now();
        return this.status;
    }

    @PreUpdate void touch() { this.updatedAt = Instant.now(); }

    public UUID getId() { return id; }
    public UUID getProjectId() { return projectId; }
    public String getCommand() { return command; }
    public TaskStatus getStatus() { return status; }
    public String getDiscordThreadId() { return discordThreadId; }
    public String getCreatedBy() { return createdBy; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }
    public int getVersion() { return version; }
    public void setDiscordThreadId(String v) { this.discordThreadId = v; }
}
