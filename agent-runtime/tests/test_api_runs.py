"""/runs API 통합 테스트 — FakeLLM + InMemorySaver + MemoryPublisher.

Phase 1 DoD: 명령 → approval.requested(plan) → resume approve → approval.requested(deliverable)
→ resume approve → run.completed. 그리고 재시작 복구(레지스트리 비움 → /continue) 와 오류 코드.
"""
from __future__ import annotations

import shutil
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from langgraph.checkpoint.memory import InMemorySaver

import app.main as main_mod
from app.events.publisher import MemoryPublisher
from app.runs.manager import RunManager
from app.settings import settings
from app.workspace.manager import WorkspaceManager

TEMPLATES = Path(__file__).resolve().parents[2] / "examples" / "devsquad-templates"
TOKEN = {"X-Internal-Token": settings.internal_token}


@pytest.fixture
def sample_project(tmp_path: Path) -> Path:
    """Phase 1 용 프로젝트: 템플릿 .devsquad 를 복사하고 pipeline 을 planning 하나로 줄인다."""
    proj = tmp_path / "proj"
    shutil.copytree(TEMPLATES / ".devsquad", proj / ".devsquad")
    (proj / ".devsquad" / "pipeline.yaml").write_text(
        "version: 1\nstages:\n  - {id: planning, agent: planner}\npolicy: {max_retries_per_approval: 2}\n", encoding="utf-8")
    (proj / "README.md").write_text("# sample\n", encoding="utf-8")
    return proj


@pytest.fixture
def client(tmp_path: Path):
    publisher = MemoryPublisher()
    mgr = RunManager(checkpointer=InMemorySaver(), publisher=publisher,
                     workspaces=WorkspaceManager(tmp_path / "ws"), fake_llm=True)
    app = main_mod.app
    app.state.runs = mgr
    # lifespan 을 우회하고 직접 주입한 상태로 테스트 (외부 의존 없음)
    with TestClient(app) as c:
        c.mgr = mgr  # type: ignore[attr-defined]
        c.publisher = publisher  # type: ignore[attr-defined]
        yield c


def _wait(client, task_id: str, timeout: float = 30.0):
    """TestClient 는 별도 스레드의 이벤트 루프에서 앱을 돌린다. 그 루프의 asyncio.Task 를 여기서 await 할 수 없으므로
    done() 을 폴링한다."""
    import time
    entry = client.mgr._entries.get(task_id)
    assert entry is not None and entry.task is not None, "실행이 시작되지 않았습니다"
    deadline = time.time() + timeout
    while not entry.task.done():
        if time.time() > deadline:
            raise TimeoutError(task_id)
        time.sleep(0.02)
    return entry.last_outcome


def test_phase1_flow_over_http(client, sample_project):
    task_id = "t-http-1"
    r = client.post("/runs", json={"task_id": task_id, "command": "다크 모드 토글 추가",
                                   "project": {"local_path": str(sample_project)}}, headers=TOKEN)
    assert r.status_code == 202, r.text
    assert r.json()["stages"] == [{"id": "planning", "agent": "planner", "depends_on": [], "approvals": ["plan", "deliverable"]}]
    assert r.json()["levels"] == [["planning"]]
    out = _wait(client, task_id)
    assert out.status == "waiting_approval" and out.interrupts[0]["kind"] == "plan"

    types = client.publisher.types(task_id)
    assert types[0] == "run.started" and "stage.started" in types and types[-1] == "approval.requested"
    seqs = [e["seq"] for e in client.publisher.events[task_id]]
    assert seqs == list(range(1, len(seqs) + 1))  # seq 단조 증가, 구멍 없음

    st = client.get(f"/runs/{task_id}/state", headers=TOKEN).json()
    assert st["interrupts"] and st["interrupts"][0]["value"]["kind"] == "plan" and st["active"] is False

    r = client.post(f"/runs/{task_id}/resume", json={"decision": "approve"}, headers=TOKEN)
    assert r.status_code == 202
    out = _wait(client, task_id)
    assert out.status == "waiting_approval" and out.interrupts[0]["kind"] == "deliverable"
    assert "deliverable.produced" in client.publisher.types(task_id)

    r = client.post(f"/runs/{task_id}/resume", json={"decision": "approve"}, headers=TOKEN)
    out = _wait(client, task_id)
    assert out.status == "completed"
    assert client.publisher.types(task_id)[-1] == "run.completed"
    written = list((client.mgr.workspaces.path_for(task_id) / "docs" / "spec").glob("*.md"))
    assert written, "결과물 파일이 workspace 에 없습니다"


def test_reject_requires_feedback_and_retry_cap_blocks(client, sample_project):
    task_id = "t-http-2"
    client.post("/runs", json={"task_id": task_id, "command": "x", "project": {"local_path": str(sample_project)}}, headers=TOKEN)
    _wait(client, task_id)

    r = client.post(f"/runs/{task_id}/resume", json={"decision": "reject"}, headers=TOKEN)
    assert r.status_code == 400 and r.json()["detail"]["code"] == "FEEDBACK_REQUIRED"

    client.post(f"/runs/{task_id}/resume", json={"decision": "reject", "feedback": "1차"}, headers=TOKEN)
    out = _wait(client, task_id)
    assert out.interrupts[0]["retry_no"] == 1
    client.post(f"/runs/{task_id}/resume", json={"decision": "reject", "feedback": "2차"}, headers=TOKEN)
    out = _wait(client, task_id)
    assert out.status == "blocked"
    assert "stage.blocked" in client.publisher.types(task_id)
    st = client.get(f"/runs/{task_id}/state", headers=TOKEN).json()
    assert st["values_summary"]["stages"]["planning"]["approved"] is False


def test_restart_recovery_rebuilds_graph_from_workspace(client, sample_project):
    """프로세스 재시작 = 레지스트리가 비었지만 workspace 와 체크포인트는 남아 있는 상황."""
    task_id = "t-http-3"
    client.post("/runs", json={"task_id": task_id, "command": "x", "project": {"local_path": str(sample_project)}}, headers=TOKEN)
    _wait(client, task_id)

    client.mgr._entries.clear()  # 재시작 흉내 (checkpointer 는 같은 인스턴스 = DB 가 남아 있음)

    st = client.get(f"/runs/{task_id}/state", headers=TOKEN).json()  # 그래프 재조립 후 조회
    assert st["interrupts"][0]["value"]["kind"] == "plan"

    r = client.post(f"/runs/{task_id}/resume", json={"decision": "approve"}, headers=TOKEN)
    assert r.status_code == 202
    out = _wait(client, task_id)
    assert out.status == "waiting_approval" and out.interrupts[0]["kind"] == "deliverable"


def test_auth_and_errors(client, tmp_path):
    assert client.post("/runs", json={"task_id": "t", "command": "x", "project": {}}).status_code == 401
    empty = tmp_path / "empty-project"
    empty.mkdir()
    r = client.post("/runs", json={"task_id": "t-bad", "command": "x", "project": {"local_path": str(empty)}}, headers=TOKEN)
    assert r.status_code == 400 and r.json()["detail"]["code"] == "PROJECT_INVALID"  # pipeline.yaml 없음
    assert client.get("/runs/nope/state", headers=TOKEN).status_code == 404
    assert client.post("/runs/nope/resume", json={"decision": "approve"}, headers=TOKEN).status_code == 404
    assert client.post("/runs/nope/cancel", headers=TOKEN).json()["cancelled"] is False
    h = client.get("/health").json()
    assert h["status"] == "ok" and h["active_runs"] == 0


def test_invalid_pipeline_is_400(client, tmp_path):
    proj = tmp_path / "p2"
    (proj / ".devsquad" / "agents").mkdir(parents=True)
    (proj / ".devsquad" / "pipeline.yaml").write_text("version: 1\nstages:\n  - {id: a, agent: ghost}\n", encoding="utf-8")
    r = client.post("/runs", json={"task_id": "t-inv", "command": "x", "project": {"local_path": str(proj)}}, headers=TOKEN)
    assert r.status_code == 400 and r.json()["detail"]["code"] == "PIPELINE_INVALID"
    assert "ghost" in r.json()["detail"]["message"]
