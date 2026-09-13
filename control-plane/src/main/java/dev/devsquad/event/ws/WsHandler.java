package dev.devsquad.event.ws;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.devsquad.event.domain.TaskEvent;
import dev.devsquad.event.infra.TaskEventRepository;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.CloseStatus;
import org.springframework.web.socket.TextMessage;
import org.springframework.web.socket.WebSocketSession;
import org.springframework.web.socket.handler.TextWebSocketHandler;

/**
 * 15-API 명세 §2 WebSocket 프로토콜.
 * subscribe(from_seq) → task_event 에서 replay 후 라이브. replay 중 도착한 라이브 이벤트는 seq 중복이라 클라이언트가 걸러낸다
 * (eventStore.append 가 중복 seq 를 무시) — 서버 측 버퍼링 없이 단순하게 간다.
 */
@Component
public class WsHandler extends TextWebSocketHandler {
    private static final int REPLAY_BATCH = 500;

    private final WsSessionRegistry registry;
    private final TaskEventRepository events;
    private final ObjectMapper json;

    public WsHandler(WsSessionRegistry registry, TaskEventRepository events, ObjectMapper json) {
        this.registry = registry; this.events = events; this.json = json;
    }

    @Override
    public void afterConnectionEstablished(WebSocketSession session) { registry.register(session); }

    @Override
    public void afterConnectionClosed(WebSocketSession session, CloseStatus status) { registry.unregister(session); }

    @Override
    protected void handleTextMessage(WebSocketSession session, TextMessage message) throws Exception {
        Map<String, Object> msg = json.readValue(message.getPayload(), Map.class);
        String op = String.valueOf(msg.get("op"));
        switch (op) {
            case "ping" -> registry.send(session, "{\"op\":\"pong\"}");
            case "subscribe" -> {
                Object tid = msg.get("task_id");
                if (tid == null) { registry.subscribeSummary(session); registry.send(session, "{\"op\":\"subscribed\",\"replayed\":0}"); return; }
                UUID taskId = UUID.fromString(tid.toString());
                registry.subscribe(session, taskId);
                long fromSeq = msg.get("from_seq") instanceof Number n ? n.longValue() : 0L;
                int replayed = replay(session, taskId, fromSeq);
                registry.send(session, json.writeValueAsString(Map.of("op", "subscribed", "task_id", taskId.toString(), "replayed", replayed)));
            }
            case "unsubscribe" -> { Object tid = msg.get("task_id"); if (tid != null) registry.unsubscribe(session, UUID.fromString(tid.toString())); }
            default -> registry.send(session, "{\"op\":\"error\",\"code\":\"UNKNOWN_OP\"}");
        }
    }

    private int replay(WebSocketSession session, UUID taskId, long fromSeq) throws Exception {
        int total = 0;
        long cursor = fromSeq;
        while (true) {
            List<TaskEvent> batch = events.findAfter(taskId, cursor, PageRequest.of(0, REPLAY_BATCH));
            for (TaskEvent e : batch) registry.send(session, json.writeValueAsString(e.toEnvelope()));
            total += batch.size();
            if (batch.size() < REPLAY_BATCH) return total;
            cursor = batch.get(batch.size() - 1).getSeq();
        }
    }
}
