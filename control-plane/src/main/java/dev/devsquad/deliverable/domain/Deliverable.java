package dev.devsquad.deliverable.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "deliverable")
public class Deliverable {
    @Id private UUID id;
    @Column(name = "task_id", nullable = false) private UUID taskId;
    @Column(name = "stage_id", nullable = false) private UUID stageId;
    @Column(nullable = false) private String kind;
    private String uri;
    @Column(columnDefinition = "text") private String content;
    @Column(name = "commit_sha") private String commitSha;
    @Column(columnDefinition = "text") private String summary;
    @Column(name = "created_at", nullable = false, updatable = false) private Instant createdAt = Instant.now();

    protected Deliverable() {}

    public Deliverable(UUID id, UUID taskId, UUID stageId, String kind, String uri, String content, String commitSha, String summary) {
        this.id = id; this.taskId = taskId; this.stageId = stageId; this.kind = kind; this.uri = uri; this.content = content;
        this.commitSha = commitSha; this.summary = summary;
    }

    public UUID getId() { return id; }
    public UUID getTaskId() { return taskId; }
    public UUID getStageId() { return stageId; }
    public String getKind() { return kind; }
    public String getUri() { return uri; }
    public String getContent() { return content; }
    public String getCommitSha() { return commitSha; }
    public String getSummary() { return summary; }
    public Instant getCreatedAt() { return createdAt; }
}
