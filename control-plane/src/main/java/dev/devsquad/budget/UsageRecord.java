package dev.devsquad.budget;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "usage_record")
public class UsageRecord {
    @Id @GeneratedValue(strategy = GenerationType.IDENTITY) private Long id;
    @Column(name = "task_id", nullable = false) private UUID taskId;
    @Column(name = "stage_key") private String stageKey;
    private String agent;
    @Column(nullable = false) private String model;
    @Column(name = "input_tokens", nullable = false) private long inputTokens;
    @Column(name = "output_tokens", nullable = false) private long outputTokens;
    @Column(nullable = false) private Instant ts = Instant.now();

    protected UsageRecord() {}

    public UsageRecord(UUID taskId, String stageKey, String agent, String model, long inputTokens, long outputTokens, Instant ts) {
        this.taskId = taskId; this.stageKey = stageKey; this.agent = agent; this.model = model;
        this.inputTokens = inputTokens; this.outputTokens = outputTokens; this.ts = ts;
    }

    public UUID getTaskId() { return taskId; }
    public long getInputTokens() { return inputTokens; }
    public long getOutputTokens() { return outputTokens; }
}
