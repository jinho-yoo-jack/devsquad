package dev.devsquad.event.ingest;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.data.redis.connection.stream.*;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * `devsquad:events` 를 consumer group 으로 읽어 EventDispatcher 에 넘긴다 — 14-Backend A.4.
 *
 * XREADGROUP ... COUNT 100 BLOCK 1s 를 스케줄러로 반복(단순·재시작 안전). 처리 성공 시 XACK, 실패 시
 * dead-letter 스트림으로 옮기고 XACK (무한 재시도로 큐가 막히지 않게).
 */
@Component
public class RedisStreamConsumer {
    private static final Logger log = LoggerFactory.getLogger(RedisStreamConsumer.class);

    private final StringRedisTemplate redis;
    private final EventDispatcher dispatcher;
    private final String streamKey;
    private final String group;
    private final String consumer = "cp-" + Long.toHexString(System.nanoTime());
    private volatile boolean ready;

    public RedisStreamConsumer(StringRedisTemplate redis, EventDispatcher dispatcher,
                               @Value("${devsquad.events.stream-key}") String streamKey,
                               @Value("${devsquad.events.consumer-group}") String group) {
        this.redis = redis; this.dispatcher = dispatcher; this.streamKey = streamKey; this.group = group;
    }

    @EventListener(ApplicationReadyEvent.class)
    public void ensureGroup() {
        try {
            redis.opsForStream().createGroup(streamKey, ReadOffset.from("0"), group);
            log.info("created consumer group {} on {}", group, streamKey);
        } catch (Exception e) {
            // BUSYGROUP (이미 있음) 또는 스트림 없음 → MKSTREAM 은 Spring Data 가 기본 수행. 이미 있으면 무시.
            log.debug("consumer group create skipped: {}", e.getMessage());
        }
        ready = true;
    }

    @Scheduled(fixedDelay = 200)
    public void poll() {
        if (!ready) return;
        List<MapRecord<String, Object, Object>> records;
        try {
            records = redis.opsForStream().read(Consumer.from(group, consumer),
                StreamReadOptions.empty().count(100).block(Duration.ofSeconds(1)),
                StreamOffset.create(streamKey, ReadOffset.lastConsumed()));
        } catch (Exception e) {
            log.warn("stream read failed: {}", e.toString());
            return;
        }
        if (records == null) return;
        for (var rec : records) handle(rec);
    }

    @SuppressWarnings({"unchecked", "rawtypes"})
    private void handle(MapRecord<String, Object, Object> rec) {
        Map<String, Object> env = (Map) rec.getValue();
        try {
            // Redis Stream 은 flat string map: seq 는 문자열 → 숫자로
            var normalized = new java.util.LinkedHashMap<>(env);
            if (normalized.get("seq") instanceof String s) normalized.put("seq", Long.parseLong(s));
            dispatcher.dispatch(normalized);
            redis.opsForStream().acknowledge(group, rec);
        } catch (Exception e) {
            log.error("event {} failed, moving to dead-letter: {}", env.get("event_id"), e.toString());
            try {
                redis.opsForStream().add(StreamRecords.mapBacked(env).withStreamKey(streamKey + ":dead"));
                redis.opsForStream().acknowledge(group, rec);
            } catch (Exception e2) {
                log.error("dead-letter failed for {}: {}", env.get("event_id"), e2.toString());
            }
        }
    }
}
