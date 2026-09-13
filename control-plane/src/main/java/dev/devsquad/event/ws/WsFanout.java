package dev.devsquad.event.ws;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.devsquad.event.ingest.EventDispatcher.TaskEventIngested;
import org.springframework.stereotype.Component;
import org.springframework.transaction.event.TransactionPhase;
import org.springframework.transaction.event.TransactionalEventListener;

/** 저장이 커밋된 뒤에만 WS 로 나간다 — 클라이언트가 REST 로 다시 읽을 때 반드시 존재하도록. */
@Component
public class WsFanout {
    private final WsSessionRegistry registry;
    private final ObjectMapper json;

    public WsFanout(WsSessionRegistry registry, ObjectMapper json) { this.registry = registry; this.json = json; }

    @TransactionalEventListener(phase = TransactionPhase.AFTER_COMMIT)
    public void on(TaskEventIngested ev) throws Exception {
        registry.broadcast(ev.taskId(), ev.event().getType(), json.writeValueAsString(ev.envelope()));
    }
}
