package dev.devsquad.runtime;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Component;
import org.springframework.web.client.HttpServerErrorException;
import org.springframework.web.client.ResourceAccessException;
import org.springframework.web.client.RestClient;

/**
 * Control Plane → Agent Runtime 내부 HTTP (15-API 명세 §4). X-Internal-Token 으로 보호.
 *
 * 재시도는 idempotent 한 호출(state, continue)에만 하고, run/resume 은 호출자(TaskService/ApprovalService)가
 * resume_pending 플래그 + RunRecoveryJob 으로 재시도한다 — 여기서 재시도하면 중복 run 위험.
 */
@Component
public class AgentRuntimeClient {

    private final RestClient http;

    public AgentRuntimeClient(RestClient.Builder builder,
                              @Value("${devsquad.agent-runtime.url}") String baseUrl,
                              @Value("${devsquad.agent-runtime.internal-token}") String internalToken) {
        this.http = builder.baseUrl(baseUrl)
            .defaultHeader("X-Internal-Token", internalToken)
            .build();
    }

    // ---- 요청/응답 DTO -----------------------------------------------------

    public record ProjectRef(String owner, String repo, String branch, String installationToken, String contextPath, String localPath) {}

    public record RunRequest(UUID taskId, String command, ProjectRef project, Map<String, Object> options) {}

    public record ResumeRequest(UUID approvalId, String decision, String feedback, String editedContent) {}

    public record RunAccepted(UUID taskId, String threadId) {}

    public record Interrupt(String id, Map<String, Object> value) {}

    public record RunState(List<String> next, List<Interrupt> interrupts, boolean active, String checkpointId,
                           Map<String, Object> valuesSummary) {
        /** 승인 대기 중 = interrupt 가 있고 다음 노드가 없다. */
        public boolean isWaitingApproval() { return interrupts != null && !interrupts.isEmpty(); }
        /** 실행이 중간에 끊겼다 = 다음 노드가 있는데 active 가 아니다 → POST /continue 로 복구. */
        public boolean isStranded() { return next != null && !next.isEmpty() && !active; }
    }

    public record Health(String status, int activeRuns) {}

    // ---- 호출 ---------------------------------------------------------------

    public RunAccepted run(RunRequest req) {
        return http.post().uri("/runs").body(req).retrieve().body(RunAccepted.class);
    }

    public void resume(UUID taskId, ResumeRequest req) {
        http.post().uri("/runs/{id}/resume", taskId).body(req).retrieve().toBodilessEntity();
    }

    public void continueRun(UUID taskId) {
        http.post().uri("/runs/{id}/continue", taskId).retrieve().toBodilessEntity();
    }

    public void cancel(UUID taskId) {
        http.post().uri("/runs/{id}/cancel", taskId).retrieve().toBodilessEntity();
    }

    public RunState state(UUID taskId) {
        return http.get().uri("/runs/{id}/state", taskId).retrieve().body(RunState.class);
    }

    /** 503 RUNTIME_UNAVAILABLE 판단용. 예외를 던지지 않는다. */
    public boolean isHealthy() {
        try {
            Health h = http.get().uri("/health").retrieve().body(Health.class);
            return h != null && "ok".equals(h.status());
        } catch (ResourceAccessException | HttpServerErrorException e) {
            return false;
        }
    }

    public static boolean isUnavailable(RuntimeException e) {
        return e instanceof ResourceAccessException
            || (e instanceof HttpServerErrorException h && h.getStatusCode() == HttpStatus.SERVICE_UNAVAILABLE);
    }

    static Duration defaultTimeout() { return Duration.ofSeconds(10); }
}
