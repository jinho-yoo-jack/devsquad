package dev.devsquad.task.api;

import dev.devsquad.approval.domain.ApprovalStatus;
import dev.devsquad.approval.infra.ApprovalRepository;
import dev.devsquad.event.infra.TaskEventRepository;
import dev.devsquad.task.app.TaskService;
import dev.devsquad.task.domain.Stage;
import dev.devsquad.task.domain.Task;
import dev.devsquad.task.domain.TaskStatus;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;

/** 15-API 명세 §1 Tasks. */
@RestController
@RequestMapping("/api/v1/tasks")
public class TaskController {
    private final TaskService service;
    private final ApprovalRepository approvals;
    private final TaskEventRepository events;

    public TaskController(TaskService service, ApprovalRepository approvals, TaskEventRepository events) {
        this.service = service; this.approvals = approvals; this.events = events;
    }

    public record CreateRequest(@NotNull UUID projectId, @NotBlank String command, Map<String, Object> options) {}

    public record StageDto(String key, String role, String status, List<String> dependsOn, int retryCount) {
        static StageDto of(Stage s) { return new StageDto(s.getStageKey(), s.getRole(), s.getStatus().name().toLowerCase(), s.getDependsOn(), s.getRetryCount()); }
    }

    public record PendingApprovalDto(UUID id, String kind, String stageKey, String title, Instant requestedAt) {}

    public record LastEventDto(long seq, String type, Instant ts) {}

    public record TaskDto(UUID id, UUID projectId, String command, String status, Instant createdAt, Instant updatedAt,
                          String discordThreadId, List<StageDto> stages, List<PendingApprovalDto> pendingApprovals, LastEventDto lastEvent) {}

    public record Page<T>(List<T> items, String nextCursor) {}

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public TaskDto create(@Valid @RequestBody CreateRequest req,
                          @RequestHeader(value = "X-User", required = false) String user) {
        // Phase 1 단일 사용자. JWT 인증은 CP-10 에서 principal 로 교체.
        Task t = service.create(req.projectId(), req.command(), user == null || user.isBlank() ? "local-user" : user);
        return toDto(t);
    }

    @GetMapping
    public Page<TaskDto> list(@RequestParam(required = false) UUID projectId,
                              @RequestParam(required = false) List<TaskStatus> status) {
        return new Page<>(service.list(projectId, status).stream().map(this::toDto).toList(), null);
    }

    @GetMapping("/{id}")
    public TaskDto get(@PathVariable UUID id) { return toDto(service.get(id)); }

    @PostMapping("/{id}/pause")  public TaskDto pause(@PathVariable UUID id)  { return toDto(service.pause(id)); }
    @PostMapping("/{id}/resume") public TaskDto resume(@PathVariable UUID id) { return toDto(service.resume(id)); }
    @PostMapping("/{id}/cancel") public TaskDto cancel(@PathVariable UUID id) { return toDto(service.cancel(id)); }

    private TaskDto toDto(Task t) {
        List<StageDto> st = service.stagesOf(t.getId()).stream().map(StageDto::of).toList();
        Map<UUID, String> stageKeyById = service.stagesOf(t.getId()).stream()
            .collect(java.util.stream.Collectors.toMap(Stage::getId, Stage::getStageKey));
        List<PendingApprovalDto> pend = approvals.findByTaskIdAndStatus(t.getId(), ApprovalStatus.PENDING).stream()
            .map(a -> new PendingApprovalDto(a.getId(), a.getKind().name().toLowerCase(), stageKeyById.get(a.getStageId()), a.getTitle(), a.getRequestedAt()))
            .toList();
        LastEventDto last = events.findTopByTaskIdOrderBySeqDesc(t.getId())
            .map(e -> new LastEventDto(e.getSeq(), e.getType(), e.getTs())).orElse(null);
        return new TaskDto(t.getId(), t.getProjectId(), t.getCommand(), t.getStatus().name().toLowerCase(), t.getCreatedAt(),
            t.getUpdatedAt(), t.getDiscordThreadId(), st, pend, last);
    }
}
