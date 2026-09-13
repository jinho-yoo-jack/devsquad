package dev.devsquad.event.ws;

import java.io.IOException;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Predicate;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.TextMessage;
import org.springframework.web.socket.WebSocketSession;

/**
 * 세션별 구독 집합. task_id 구독과 "요약 채널"(task 없는 subscribe → approval.* / run.* 만) 을 구분한다.
 * 다중 인스턴스 팬아웃(Redis Pub/Sub)은 Phase 3.
 */
@Component
public class WsSessionRegistry {
    private static final Logger log = LoggerFactory.getLogger(WsSessionRegistry.class);
    private static final Predicate<String> SUMMARY_TYPES = t -> t.startsWith("approval.") || t.startsWith("run.") || t.startsWith("stage.");

    private final Map<String, WebSocketSession> sessions = new ConcurrentHashMap<>();
    private final Map<String, Set<UUID>> taskSubs = new ConcurrentHashMap<>();
    private final Set<String> summarySubs = ConcurrentHashMap.newKeySet();

    public void register(WebSocketSession s) { sessions.put(s.getId(), s); taskSubs.put(s.getId(), ConcurrentHashMap.newKeySet()); }

    public void unregister(WebSocketSession s) { sessions.remove(s.getId()); taskSubs.remove(s.getId()); summarySubs.remove(s.getId()); }

    public void subscribe(WebSocketSession s, UUID taskId) { taskSubs.computeIfAbsent(s.getId(), k -> ConcurrentHashMap.newKeySet()).add(taskId); }

    public void subscribeSummary(WebSocketSession s) { summarySubs.add(s.getId()); }

    public void unsubscribe(WebSocketSession s, UUID taskId) { Set<UUID> set = taskSubs.get(s.getId()); if (set != null) set.remove(taskId); }

    /** 이벤트를 구독자에게 팬아웃. 전송 실패 세션은 정리한다. */
    public void broadcast(UUID taskId, String type, String json) {
        for (var e : sessions.entrySet()) {
            String sid = e.getKey();
            boolean wants = taskSubs.getOrDefault(sid, Set.of()).contains(taskId) || (summarySubs.contains(sid) && SUMMARY_TYPES.test(type));
            if (wants) send(e.getValue(), json);
        }
    }

    public void send(WebSocketSession s, String json) {
        try {
            if (s.isOpen()) synchronized (s) { s.sendMessage(new TextMessage(json)); }
        } catch (IOException | IllegalStateException ex) {
            log.debug("ws send failed to {}: {}", s.getId(), ex.getMessage());
            unregister(s);
        }
    }

    public int sessionCount() { return sessions.size(); }
}
