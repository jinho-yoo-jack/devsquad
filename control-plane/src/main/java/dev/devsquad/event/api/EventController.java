package dev.devsquad.event.api;

import dev.devsquad.event.domain.TaskEvent;
import dev.devsquad.event.infra.TaskEventRepository;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.data.domain.PageRequest;
import org.springframework.web.bind.annotation.*;

/** GET /api/v1/tasks/{id}/events?after_seq=&limit= — WS 구멍 채우기·초기 로드 (15-API §1). */
@RestController
@RequestMapping("/api/v1/tasks/{taskId}/events")
public class EventController {
    private final TaskEventRepository events;

    public EventController(TaskEventRepository events) { this.events = events; }

    public record Page(List<Map<String, Object>> items, Long nextAfterSeq) {}

    @GetMapping
    public Page list(@PathVariable UUID taskId,
                     @RequestParam(defaultValue = "0") long afterSeq,
                     @RequestParam(defaultValue = "500") int limit) {
        int size = Math.max(1, Math.min(limit, 1000));
        List<TaskEvent> batch = events.findAfter(taskId, afterSeq, PageRequest.of(0, size));
        Long next = batch.size() == size ? batch.get(batch.size() - 1).getSeq() : null;
        return new Page(batch.stream().map(TaskEvent::toEnvelope).toList(), next);
    }
}
