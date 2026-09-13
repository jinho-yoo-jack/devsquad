package dev.devsquad;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import dev.devsquad.common.HealthController;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

/** DB 없이 도는 슬라이스 테스트. Testcontainers 기반 통합 테스트는 CP-1 에서 추가. */
@WebMvcTest(HealthController.class)
class HealthControllerTest {

    @Autowired MockMvc mvc;

    @Test
    void health_returns_ok() throws Exception {
        mvc.perform(get("/api/v1/health"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.service").value("control-plane"));
    }
}
