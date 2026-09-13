package dev.devsquad.approval.api;

import dev.devsquad.approval.app.ApprovalService;
import dev.devsquad.approval.domain.Approval;
import dev.devsquad.approval.domain.ApprovalStatus;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.web.bind.annotation.*;

/** 15-API 명세 §1 Approvals. */
@RestController
@RequestMapping("/api/v1")
public class ApprovalController {
    private final ApprovalService service;

    public ApprovalController(ApprovalService service) { this.service = service; }

    public record DecideRequest(String decision, String feedback, String editedContent) {}

    public record ApprovalDto(UUID id, UUID taskId, UUID stageId, String kind, int retryNo, String status, String title,
                              String content, String decisionFeedback, String editedContent, String decidedBy, String decidedVia,
                              Instant requestedAt, Instant decidedAt) {
        static ApprovalDto of(Approval a) {
            return new ApprovalDto(a.getId(), a.getTaskId(), a.getStageId(), a.getKind().name().toLowerCase(), a.getRetryNo(),
                a.getStatus().name().toLowerCase(), a.getTitle(), a.getContent(), a.getDecisionFeedback(), a.getEditedContent(),
                a.getDecidedBy(), a.getDecidedVia(), a.getRequestedAt(), a.getDecidedAt());
        }
        ApprovalDto withoutContent() {
            return new ApprovalDto(id, taskId, stageId, kind, retryNo, status, title, null, decisionFeedback, null, decidedBy, decidedVia, requestedAt, decidedAt);
        }
    }

    public record DecidedResponse(UUID id, String status, Instant decidedAt, String decidedVia) {}

    @GetMapping("/approvals")
    public List<ApprovalDto> pending(@RequestParam(defaultValue = "pending") String status) {
        return service.pending().stream().map(a -> ApprovalDto.of(a).withoutContent()).toList();
    }

    @GetMapping("/approvals/{id}")
    public ApprovalDto get(@PathVariable UUID id) { return ApprovalDto.of(service.get(id)); }

    @PostMapping("/approvals/{id}/decide")
    public DecidedResponse decide(@PathVariable UUID id, @RequestBody DecideRequest req,
                                  @RequestHeader(value = "X-User", required = false) String user) {
        var decision = ApprovalService.toDecision(req.decision(), req.feedback(), req.editedContent());
        Approval a = service.decide(id, decision, user == null || user.isBlank() ? "local-user" : user, "web");
        return new DecidedResponse(a.getId(), a.getStatus().name().toLowerCase(), a.getDecidedAt(), a.getDecidedVia());
    }

    @GetMapping("/tasks/{taskId}/approvals")
    public List<ApprovalDto> ofTask(@PathVariable UUID taskId, @RequestParam(required = false) ApprovalStatus status) {
        return service.ofTask(taskId, status).stream().map(ApprovalDto::of).toList();
    }
}
