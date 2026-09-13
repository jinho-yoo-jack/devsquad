"""이벤트 발행 — 11-아키텍처 §3.2 envelope, Redis Stream `devsquad:events`.

`EventPublisher` 인터페이스 뒤에 Redis 구현과 테스트/로컬용 메모리·파일 구현을 둔다.
seq 는 task 별 단조 증가여야 하므로 Redis `INCR devsquad:seq:<task_id>` 로 발급한다.
"""
from __future__ import annotations

import json
import threading
import uuid
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Protocol


def make_envelope(task_id: str, seq: int, ev: dict[str, Any]) -> dict[str, Any]:
    return {
        "event_id": str(uuid.uuid4()),
        "task_id": task_id,
        "seq": seq,
        "ts": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "stage_key": ev.get("stage_key"),
        "agent": ev.get("agent"),
        "type": ev["type"],
        "payload": ev.get("payload") or {},
    }


class EventPublisher(Protocol):
    def publish(self, task_id: str, ev: dict[str, Any]) -> dict[str, Any]: ...


class MemoryPublisher:
    """테스트용. 발행된 envelope 를 task 별로 보관한다."""

    def __init__(self) -> None:
        self.events: dict[str, list[dict[str, Any]]] = defaultdict(list)
        self._seq: dict[str, int] = defaultdict(int)
        self._lock = threading.Lock()

    def publish(self, task_id: str, ev: dict[str, Any]) -> dict[str, Any]:
        with self._lock:
            self._seq[task_id] += 1
            env = make_envelope(task_id, self._seq[task_id], ev)
            self.events[task_id].append(env)
            return env

    def types(self, task_id: str) -> list[str]:
        return [e["type"] for e in self.events[task_id]]


class JsonlPublisher(MemoryPublisher):
    """로컬 개발용: Redis 없이 `<dir>/<task_id>.jsonl` 에 append (Control Plane 없이 흐름 확인)."""

    def __init__(self, directory: Path) -> None:
        super().__init__()
        self.dir = directory
        self.dir.mkdir(parents=True, exist_ok=True)

    def publish(self, task_id: str, ev: dict[str, Any]) -> dict[str, Any]:
        env = super().publish(task_id, ev)
        with (self.dir / f"{task_id}.jsonl").open("a", encoding="utf-8") as f:
            f.write(json.dumps(env, ensure_ascii=False) + "\n")
        return env


class RedisPublisher:
    """XADD devsquad:events * <envelope 필드…>. payload 는 JSON 문자열로 넣는다 (Stream 은 flat map)."""

    def __init__(self, redis_url: str, stream_key: str = "devsquad:events", maxlen: int = 1_000_000) -> None:
        import redis  # 지연 import

        self.r = redis.Redis.from_url(redis_url, decode_responses=True)
        self.stream_key = stream_key
        self.maxlen = maxlen

    def publish(self, task_id: str, ev: dict[str, Any]) -> dict[str, Any]:
        seq = int(self.r.incr(f"devsquad:seq:{task_id}"))
        env = make_envelope(task_id, seq, ev)
        fields = {k: ("" if v is None else (json.dumps(v, ensure_ascii=False) if isinstance(v, dict) else str(v)))
                  for k, v in env.items()}
        self.r.xadd(self.stream_key, fields, maxlen=self.maxlen, approximate=True)
        return env


def make_publisher(redis_url: str | None, fallback_dir: Path | None = None) -> EventPublisher:
    if redis_url:
        try:
            p = RedisPublisher(redis_url)
            p.r.ping()
            return p
        except Exception:
            if fallback_dir is None:
                raise
    return JsonlPublisher(fallback_dir) if fallback_dir else MemoryPublisher()
