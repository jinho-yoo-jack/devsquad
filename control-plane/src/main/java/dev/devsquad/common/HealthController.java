package dev.devsquad.common;

import java.time.Instant;
import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/** 14-Backend 설계 A.2 — common. 통합 상태는 Phase 1 CP-10 에서 /api/v1/integrations/status 로 확장. */
@RestController
public class HealthController {

    @GetMapping("/api/v1/health")
    public Map<String, Object> health() {
        return Map.of("status", "ok", "service", "control-plane", "ts", Instant.now().toString());
    }
}
