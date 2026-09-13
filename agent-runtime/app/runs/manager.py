"""실행 관리 — 14-Backend 설계 B.9.

task_id → 컴파일된 그래프 + 실행 중 asyncio.Task 레지스트리. 그래프는 Task 마다 pipeline.yaml 로 새로
조립되며(사용자 정의 팀원), 프로세스가 재시작되면 레지스트리는 비지만 체크포인트는 남아 있으므로
`continue_run` 이 workspace 의 .devsquad 를 다시 읽어 그래프를 재조립하고 이어 간다.
"""
from __future__ import annotations

import asyncio
import contextlib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from langgraph.types import Command

from app.events.publisher import EventPublisher
from app.events.stream_bridge import RunOutcome, drive_sync
from app.graph.builder import build_from_context
from app.graph.pipeline import PipelineSpec
from app.workspace.manager import WorkspaceManager


class RunNotFound(KeyError):
    pass


class RunBusy(RuntimeError):
    """같은 task 에 대해 실행이 이미 진행 중 (중복 run/resume 방지)."""


@dataclass
class RunEntry:
    task_id: str
    graph: Any  # CompiledStateGraph
    pipeline: PipelineSpec
    task: asyncio.Task | None = None
    last_outcome: RunOutcome | None = None
    history: list[str] = field(default_factory=list)

    @property
    def active(self) -> bool:
        return self.task is not None and not self.task.done()


class RunManager:
    def __init__(self, checkpointer: Any, publisher: EventPublisher, workspaces: WorkspaceManager, fake_llm: bool):
        self.checkpointer = checkpointer
        self.publisher = publisher
        self.workspaces = workspaces
        self.fake_llm = fake_llm
        self._entries: dict[str, RunEntry] = {}
        self._lock = asyncio.Lock()

    # ---- 그래프 준비 --------------------------------------------------------

    def _config(self, task_id: str) -> dict:
        return {"configurable": {"thread_id": task_id}}

    def _compile(self, task_id: str, context_dir: Path, workspace: Path) -> RunEntry:
        graph, pipeline = build_from_context(context_dir, workspace, fake_llm=self.fake_llm)
        compiled = graph.compile(checkpointer=self.checkpointer)
        entry = RunEntry(task_id=task_id, graph=compiled, pipeline=pipeline)
        self._entries[task_id] = entry
        return entry

    def _entry_or_rebuild(self, task_id: str, context_path: str = ".devsquad") -> RunEntry:
        e = self._entries.get(task_id)
        if e:
            return e
        if not self.workspaces.exists(task_id):
            raise RunNotFound(task_id)
        ws = self.workspaces.prepare(task_id, None, context_path)
        return self._compile(task_id, ws.context_dir, ws.path)

    # ---- 명령 ---------------------------------------------------------------

    async def start(self, task_id: str, command: str, project: dict[str, Any]) -> RunEntry:
        async with self._lock:
            if task_id in self._entries and self._entries[task_id].active:
                raise RunBusy(task_id)
            ws = self.workspaces.prepare(task_id, project.get("local_path"), project.get("context_path") or ".devsquad")
            entry = self._compile(task_id, ws.context_dir, ws.path)
            initial = {"task_id": task_id, "command": command, "project": project, "results": {}, "messages_log": [], "usage": []}
            entry.task = asyncio.create_task(self._run(entry, initial, announce=True))
            entry.history.append("start")
            return entry

    async def resume(self, task_id: str, decision: dict[str, Any]) -> RunEntry:
        async with self._lock:
            entry = self._entry_or_rebuild(task_id)
            if entry.active:
                raise RunBusy(task_id)
            entry.task = asyncio.create_task(self._run(entry, Command(resume=decision), announce=True))
            entry.history.append(f"resume:{decision.get('decision')}")
            return entry

    async def continue_run(self, task_id: str) -> RunEntry:
        """재개 값 없이 마지막 체크포인트에서 이어 실행 (런타임 재시작 복구)."""
        async with self._lock:
            entry = self._entry_or_rebuild(task_id)
            if entry.active:
                raise RunBusy(task_id)
            entry.task = asyncio.create_task(self._run(entry, None, announce=False))
            entry.history.append("continue")
            return entry

    async def cancel(self, task_id: str) -> bool:
        entry = self._entries.get(task_id)
        if entry and entry.active and entry.task:
            entry.task.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await entry.task
            self.publisher.publish(task_id, {"type": "run.cancelled", "agent": "system", "payload": {}})
            entry.history.append("cancel")
            return True
        return False

    async def state(self, task_id: str) -> dict[str, Any]:
        entry = self._entry_or_rebuild(task_id)
        snap = await asyncio.to_thread(entry.graph.get_state, self._config(task_id))
        interrupts = []
        for t in snap.tasks or ():
            for intr in getattr(t, "interrupts", ()) or ():
                interrupts.append({"id": getattr(intr, "id", None), "value": getattr(intr, "value", None)})
        values = snap.values or {}
        return {
            "task_id": task_id,
            "next": list(snap.next or ()),
            "interrupts": interrupts,
            "active": entry.active,
            "checkpoint_id": (snap.config or {}).get("configurable", {}).get("checkpoint_id"),
            "values_summary": {
                "stages": {k: {"approved": r.get("approved"), "deliverable_ref": r.get("deliverable_ref")}
                           for k, r in (values.get("results") or {}).items()},
                "error": values.get("error"),
                "usage_events": len(values.get("usage") or []),
            },
            "last_outcome": entry.last_outcome.status if entry.last_outcome else None,
            "history": entry.history[-10:],
        }

    def active_count(self) -> int:
        return sum(1 for e in self._entries.values() if e.active)

    async def wait(self, task_id: str, timeout: float = 30.0) -> RunOutcome | None:
        """테스트/디버그용: 현재 실행이 끝나기를 기다린다."""
        entry = self._entries.get(task_id)
        if not entry or not entry.task:
            return None
        await asyncio.wait_for(asyncio.shield(entry.task), timeout)
        return entry.last_outcome

    # ---- 내부 ---------------------------------------------------------------

    async def _run(self, entry: RunEntry, input_or_command: Any, announce: bool) -> RunOutcome:
        try:
            # 동기 stream 을 워커 스레드에서 (stream_bridge 모듈 docstring 참고)
            outcome = await asyncio.to_thread(drive_sync, entry.graph, entry.task_id, input_or_command,
                                              self._config(entry.task_id), self.publisher, announce_start=announce)
        except Exception as e:  # noqa: BLE001 - drive_sync 가 run.failed 를 이미 발행함
            outcome = RunOutcome(status="failed", error=str(e))
        entry.last_outcome = outcome
        return outcome
