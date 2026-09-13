package dev.devsquad.event.ingest;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.ApprovalKind;
import dev.devsquad.approval.infra.ApprovalRepository;
import dev.devsquad.budget.UsageRecord;
import dev.devsquad.budget.UsageRecordRepository;
import dev.devsquad.deliverable.domain.Deliverable;
import dev.devsquad.deliverable.infra.DeliverableRepository;
import dev.devsquad.event.domain.TaskEvent;
import dev.devsquad.event.infra.TaskEventRepository;
import dev.devsquad.task.app.TaskService;
import dev.devsquad.task.domain.Stage;
import dev.devsquad.task.domain.StageStateMachine;
import dev.devsquad.task.domain.TaskStateMachine.Event;
import dev.devsquad.task.infra.StageRepository;
import java.time.Instant;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Agent Runtime 이벤트 → 저장 + 상태 전이 + 도메인 이벤트 — 14-Backend 설계 A.4 EventDispatcher.
 *
 * 입력은 이미 Map 으로 파싱된 envelope (Redis consumer 또는 테스트가 넘긴다).
 * event_id 로 중복을 걸러 at-least-once 전달에 멱등하다. 처리 후 `TaskEventIngested` 를 발행해
 * WebSocket 허브·Discord 어댑터가 팬아웃한다.
 */
@Service
public class EventDispatcher {
    private static final Logger log = LoggerFactory.getLogger(EventDispatcher.class);

    /** 저장 완료 후 팬아웃용. envelope 은 WS 로 그대로 나간다. */
    public record TaskEventIngested(UUID taskId, TaskEvent event, Map<String, Object> envelope) {}
    /** approval.requested 처리 결과 — Discord 카드 생성 트리거. */
    public record ApprovalRequested(UUID taskId, Approval approval, String stageKey) {}

    private final TaskEventRepository events;
    private final StageRepository stages;
    private final ApprovalRepository approvals;
    private final DeliverableRepository deliverables;
    private final UsageRecordRepository usage;
    private final TaskService taskService;
    private final ApplicationEventPublisher bus;
    private final ObjectMapper json;

    public EventDispatcher(TaskEventRepository events, StageRepository stages, ApprovalRepository approvals,
                           DeliverableRepository deliverables, UsageRecordRepository usage, TaskService taskService,
                           ApplicationEventPublisher bus, ObjectMapper json) {
        this.events = events; this.stages = stages; this.approvals = approvals; this.deliverables = deliverables;
        this.usage = usage; this.taskService = taskService; this.bus = bus; this.json = json;
    }

    @Transactional
    public boolean dispatch(Map<String, Object> env) {
        UUID eventId = UUID.fromString(str(env, "event_id"));
        if (events.existsByEventId(eventId)) return false; // 중복 (XREADGROUP 재전달)

        UUID taskId = UUID.fromString(str(env, "task_id"));
        long seq = ((Number) env.get("seq")).longValue();
        String type = str(env, "type");
        String stageKey = optStr(env, "stage_key");
        String agent = optStr(env, "agent");
        Map<String, Object> payload = payload(env);
        Instant ts = Instant.parse(str(env, "ts"));

        TaskEvent saved = events.save(new TaskEvent(eventId, taskId, seq, stageKey, agent, type, payload, ts));
        Map<String, Object> outgoing = saved.toEnvelope();

        switch (type) {
            case "run.started" -> { /* Task 는 run 수락 시점에 RUNNING 으로 이미 전이됨 */ }
            case "run.resumed" -> taskService.transitionIfPossible(taskId, Event.APPROVED, "run.resumed");
            case "run.completed" -> {
                boolean blocked = payload.get("error") != null;
                if (!blocked) taskService.transitionIfPossible(taskId, Event.COMPLETE, "run.completed");
            }
            case "run.failed" -> taskService.transitionIfPossible(taskId, Event.FAIL, String.valueOf(payload.get("error")));
            case "stage.started" -> stage(taskId, stageKey).ifPresent(s -> applyStage(s, StageStateMachine.Event.STARTED));
            case "stage.completed" -> stage(taskId, stageKey).ifPresent(s -> applyStage(s, StageStateMachine.Event.COMPLETED));
            case "stage.blocked" -> {
                stage(taskId, stageKey).ifPresent(s -> applyStage(s, StageStateMachine.Event.BLOCKED_BY_RETRIES));
                taskService.transitionIfPossible(taskId, Event.REJECTED_OVER_LIMIT, "retry limit");
            }
            case "approval.requested" -> outgoing = onApprovalRequested(taskId, stageKey, payload, outgoing);
            case "deliverable.produced" -> outgoing = onDeliverable(taskId, stageKey, payload, outgoing);
            case "usage" -> usage.save(new UsageRecord(taskId, stageKey, agent, str(payload, "model"),
                num(payload, "input_tokens"), num(payload, "output_tokens"), ts));
            default -> { /* agent.thinking / tool_call / tool_result / message → 저장 + 팬아웃만 */ }
        }

        bus.publishEvent(new TaskEventIngested(taskId, saved, outgoing));
        return true;
    }

    private Map<String, Object> onApprovalRequested(UUID taskId, String stageKey, Map<String, Object> p, Map<String, Object> outgoing) {
        Stage stage = stage(taskId, stageKey).orElseThrow(() -> new IllegalStateException("unknown stage " + stageKey + " for task " + taskId));
        ApprovalKind kind = ApprovalKind.valueOf(str(p, "kind").toUpperCase());
        int retryNo = (int) num(p, "retry_no");
        Approval a = approvals.save(new Approval(UUID.randomUUID(), taskId, stage.getId(), kind, retryNo,
            str(p, "title"), String.valueOf(p.getOrDefault("content", ""))));
        applyStage(stage, kind == ApprovalKind.PLAN ? StageStateMachine.Event.PLAN_REQUESTED : StageStateMachine.Event.DELIVERABLE_REQUESTED);
        taskService.transitionIfPossible(taskId, Event.APPROVAL_REQUESTED, "approval.requested");
        bus.publishEvent(new ApprovalRequested(taskId, a, stageKey));

        // 재발행: approval_id 를 붙이고 본문은 미리보기로 (15-API §3)
        var out = new java.util.LinkedHashMap<>(outgoing);
        var np = new java.util.LinkedHashMap<String, Object>();
        np.put("approval_id", a.getId().toString());
        np.put("kind", kind.name().toLowerCase());
        np.put("retry_no", retryNo);
        np.put("title", a.getTitle());
        np.put("content_preview", preview(a.getContent()));
        np.put("deliverable_ref", p.get("deliverable_ref"));
        out.put("payload", np);
        return out;
    }

    private Map<String, Object> onDeliverable(UUID taskId, String stageKey, Map<String, Object> p, Map<String, Object> outgoing) {
        Stage stage = stage(taskId, stageKey).orElseThrow(() -> new IllegalStateException("unknown stage " + stageKey));
        Deliverable d = deliverables.save(new Deliverable(UUID.randomUUID(), taskId, stage.getId(), str(p, "kind"),
            optStr(p, "uri"), optStr(p, "content"), optStr(p, "commit_sha"), optStr(p, "summary")));
        var out = new java.util.LinkedHashMap<>(outgoing);
        var np = new java.util.LinkedHashMap<String, Object>(p);
        np.remove("content");
        np.put("deliverable_id", d.getId().toString());
        out.put("payload", np);
        return out;
    }

    private void applyStage(Stage s, StageStateMachine.Event ev) {
        try { s.apply(ev); }
        catch (RuntimeException e) { log.debug("stage {} ignores {} from {}: {}", s.getStageKey(), ev, s.getStatus(), e.getMessage()); }
    }

    private Optional<Stage> stage(UUID taskId, String key) {
        return key == null ? Optional.empty() : stages.findByTaskIdAndStageKey(taskId, key);
    }

    // ---- 파싱 유틸 ----------------------------------------------------------

    @SuppressWarnings("unchecked")
    private Map<String, Object> payload(Map<String, Object> env) {
        Object p = env.get("payload");
        if (p instanceof Map<?, ?> m) return (Map<String, Object>) m;
        if (p instanceof String s && !s.isBlank()) {
            try { return json.readValue(s, Map.class); } catch (Exception e) { log.warn("bad payload json: {}", s); }
        }
        return Map.of();
    }

    private static String str(Map<String, Object> m, String k) {
        Object v = m.get(k);
        if (v == null) throw new IllegalArgumentException("missing field: " + k);
        return v.toString();
    }
    private static String optStr(Map<String, Object> m, String k) {
        Object v = m.get(k);
        return v == null || v.toString().isEmpty() ? null : v.toString();
    }
    private static long num(Map<String, Object> m, String k) {
        Object v = m.get(k);
        return v instanceof Number n ? n.longValue() : v == null ? 0 : Long.parseLong(v.toString());
    }
    static String preview(String s) { return s == null ? "" : s.length() <= 1500 ? s : s.substring(0, 1497) + "..."; }
}
