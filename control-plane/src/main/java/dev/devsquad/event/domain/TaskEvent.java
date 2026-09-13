package dev.devsquad.event.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

/** 11-아키텍처 §3.2 envelope 의 영속 형태. (task_id, seq) 유일. */
@Entity
@Table(name = "task_event")
public class TaskEvent {
    @Id @GeneratedValue(strategy = GenerationType.IDENTITY) private Long id;
    @Column(name = "event_id", nullable = false, unique = true) private UUID eventId;
    @Column(name = "task_id", nullable = false) private UUID taskId;
    @Column(nullable = false) private long seq;
    @Column(name = "stage_key") private String stageKey;
    private String agent;
    @Column(nullable = false) private String type;
    @JdbcTypeCode(SqlTypes.JSON) @Column(nullable = false, columnDefinition = "jsonb") private Map<String, Object> payload;
    @Column(nullable = false) private Instant ts;

    protected TaskEvent() {}

    public TaskEvent(UUID eventId, UUID taskId, long seq, String stageKey, String agent, String type, Map<String, Object> payload, Instant ts) {
        this.eventId = eventId; this.taskId = taskId; this.seq = seq; this.stageKey = stageKey; this.agent = agent;
        this.type = type; this.payload = payload; this.ts = ts;
    }

    public Long getId() { return id; }
    public UUID getEventId() { return eventId; }
    public UUID getTaskId() { return taskId; }
    public long getSeq() { return seq; }
    public String getStageKey() { return stageKey; }
    public String getAgent() { return agent; }
    public String getType() { return type; }
    public Map<String, Object> getPayload() { return payload; }
    public Instant getTs() { return ts; }

    /** WS/REST 로 나가는 envelope (15-API §3). */
    public Map<String, Object> toEnvelope() {
        var m = new java.util.LinkedHashMap<String, Object>();
        m.put("event_id", eventId.toString()); m.put("task_id", taskId.toString()); m.put("seq", seq); m.put("ts", ts.toString());
        m.put("stage_key", stageKey); m.put("agent", agent); m.put("type", type); m.put("payload", payload == null ? Map.of() : payload);
        return m;
    }
}
