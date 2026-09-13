"""graph.stream(...) → EventPublisher — 14-Backend 설계 B.8.

- `custom` 모드: 노드가 get_stream_writer() 로 쓴 이벤트(agent.*, approval.requested, deliverable.produced,
  usage, stage.*)를 그대로 발행한다.
- `updates` 모드: `__interrupt__` 를 감지해 실행이 승인 대기로 멈췄음을 알린다 (approval.requested 자체는
  gate 노드가 이미 custom 으로 발행했으므로 중복 발행하지 않는다).
- 시작/종료/실패는 여기서 run.* 로 발행한다.

실행은 **동기 `graph.stream()` 을 워커 스레드**에서 돌린다. 노드 함수가 동기이고 interrupt()/
get_stream_writer() 가 runnable 컨텍스트(contextvars)를 요구하는데, Python 3.10 의 async 경로에서는
sync 노드로 컨텍스트가 전파되지 않기 때문이다. RunManager 가 asyncio.to_thread 로 감싼다.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from app.events.publisher import EventPublisher


@dataclass
class RunOutcome:
    status: str  # waiting_approval | completed | blocked | failed | cancelled
    interrupts: list[dict[str, Any]] = field(default_factory=list)
    error: str | None = None
    events: int = 0


def _unpack(chunk: Any) -> tuple[str, Any]:
    """subgraphs=True 면 (namespace, mode, data), 아니면 (mode, data)."""
    if isinstance(chunk, tuple):
        if len(chunk) == 3:
            return chunk[1], chunk[2]
        if len(chunk) == 2:
            return chunk[0], chunk[1]
    return "updates", chunk


def _is_command(x: Any) -> bool:
    return type(x).__name__ == "Command"


def drive_sync(graph: Any, task_id: str, input_or_command: Any, config: dict, publisher: EventPublisher,
               *, announce_start: bool = True) -> RunOutcome:
    outcome = RunOutcome(status="completed")
    if announce_start:
        kind = "run.resumed" if _is_command(input_or_command) else "run.started"
        publisher.publish(task_id, {"type": kind, "agent": "system", "payload": {}})
        outcome.events += 1
    try:
        for chunk in graph.stream(input_or_command, config, stream_mode=["updates", "custom"], subgraphs=True):
            mode, data = _unpack(chunk)
            if mode == "custom" and isinstance(data, dict) and "type" in data:
                publisher.publish(task_id, data)
                outcome.events += 1
            elif mode == "updates" and isinstance(data, dict) and "__interrupt__" in data:
                for intr in data["__interrupt__"] or ():
                    value = getattr(intr, "value", intr)
                    outcome.interrupts.append(value if isinstance(value, dict) else {"value": value})
        if outcome.interrupts:
            outcome.status = "waiting_approval"
        else:
            values = graph.get_state(config).values or {}
            err = values.get("error")
            outcome.status = "completed" if not err else "blocked"
            publisher.publish(task_id, {"type": "run.completed", "agent": "system",
                                        "payload": {"error": err, "results": _results_summary(values)}})
            outcome.events += 1
    except Exception as e:
        outcome.status = "failed"
        outcome.error = f"{type(e).__name__}: {e}"
        publisher.publish(task_id, {"type": "run.failed", "agent": "system", "payload": {"error": outcome.error}})
        outcome.events += 1
        raise
    return outcome


def _results_summary(values: dict[str, Any]) -> dict[str, Any]:
    out = {}
    for k, r in (values.get("results") or {}).items():
        out[k] = {"approved": r.get("approved"), "deliverable_ref": r.get("deliverable_ref"), "retries": r.get("retries")}
    return out
