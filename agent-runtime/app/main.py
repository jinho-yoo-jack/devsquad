"""Agent Runtime — 14-Backend 설계 Part B, 15-API 명세 §4 (CP → AR 내부 HTTP).

기동 시 checkpointer(memory|postgres)·publisher(redis|jsonl)·RunManager 를 만들고 app.state 에 둔다.
모든 /runs 엔드포인트는 X-Internal-Token 을 요구한다.
"""
from __future__ import annotations

from contextlib import asynccontextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from fastapi import Depends, FastAPI, Header, HTTPException, Request, status
from pydantic import BaseModel, Field

from app.agents.loader import AgentDefinitionError
from app.checkpointer import open_checkpointer
from app.events.publisher import make_publisher
from app.graph.pipeline import PipelineError
from app.runs.manager import RunBusy, RunManager, RunNotFound
from app.settings import settings
from app.workspace.manager import WorkspaceManager


@asynccontextmanager
async def lifespan(app: FastAPI):
    if getattr(app.state, "runs", None) is not None:  # 테스트가 RunManager 를 직접 주입한 경우
        yield
        return
    with open_checkpointer(settings.db_url if not settings.fake_llm or settings.force_db else None) as cp:
        publisher = make_publisher(settings.redis_url if settings.use_redis else None,
                                   fallback_dir=Path(settings.workspace_root) / "_events")
        app.state.runs = RunManager(
            checkpointer=cp,
            publisher=publisher,
            workspaces=WorkspaceManager(Path(settings.workspace_root)),
            fake_llm=settings.fake_llm,
        )
        yield


app = FastAPI(title="DevSquad Agent Runtime", version="0.1.0", lifespan=lifespan)


def require_internal(x_internal_token: str | None = Header(default=None)) -> None:
    if x_internal_token != settings.internal_token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid internal token")


def runs(request: Request) -> RunManager:
    return request.app.state.runs


# ---- 스키마 -----------------------------------------------------------------


class HealthResponse(BaseModel):
    status: str
    service: str
    active_runs: int
    ts: str


class ProjectRef(BaseModel):
    owner: str | None = None
    repo: str | None = None
    branch: str | None = None
    installation_token: str | None = None
    context_path: str = ".devsquad"
    local_path: str | None = None  # Phase 1: 로컬 디렉토리


class RunRequest(BaseModel):
    task_id: str = Field(min_length=1)
    command: str = Field(min_length=1)
    project: ProjectRef
    options: dict[str, Any] = Field(default_factory=dict)


class ResumeRequest(BaseModel):
    approval_id: str | None = None
    decision: str = Field(pattern="^(approve|reject|edit)$")
    feedback: str | None = None
    edited_content: str | None = None


class StageInfo(BaseModel):
    id: str
    agent: str
    depends_on: list[str]
    approvals: list[str]


class Accepted(BaseModel):
    task_id: str
    thread_id: str
    status: str = "accepted"
    stages: list[StageInfo] = Field(default_factory=list)
    levels: list[list[str]] = Field(default_factory=list)


# ---- 엔드포인트 -------------------------------------------------------------


@app.get("/health", response_model=HealthResponse)
def health(request: Request) -> HealthResponse:
    mgr: RunManager | None = getattr(request.app.state, "runs", None)
    return HealthResponse(status="ok", service="agent-runtime", active_runs=mgr.active_count() if mgr else 0,
                          ts=datetime.now(timezone.utc).isoformat())


@app.post("/runs", status_code=status.HTTP_202_ACCEPTED, response_model=Accepted, dependencies=[Depends(require_internal)])
async def create_run(body: RunRequest, mgr: RunManager = Depends(runs)) -> Accepted:
    try:
        entry = await mgr.start(body.task_id, body.command, body.project.model_dump())
    except RunBusy:
        raise HTTPException(status.HTTP_409_CONFLICT, detail="run already active") from None
    except (PipelineError, AgentDefinitionError) as e:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, detail={"code": "PIPELINE_INVALID", "message": str(e)}) from None
    except (FileNotFoundError, ValueError) as e:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, detail={"code": "PROJECT_INVALID", "message": str(e)}) from None
    p = entry.pipeline
    return Accepted(
        task_id=body.task_id, thread_id=body.task_id,
        stages=[StageInfo(id=s.id, agent=s.agent, depends_on=s.depends_on, approvals=list(s.approvals)) for s in p.stages],
        levels=p.levels(),
    )


@app.post("/runs/{task_id}/resume", status_code=status.HTTP_202_ACCEPTED, response_model=Accepted, dependencies=[Depends(require_internal)])
async def resume_run(task_id: str, body: ResumeRequest, mgr: RunManager = Depends(runs)) -> Accepted:
    if body.decision == "reject" and not (body.feedback or "").strip():
        raise HTTPException(status.HTTP_400_BAD_REQUEST, detail={"code": "FEEDBACK_REQUIRED"})
    try:
        await mgr.resume(task_id, body.model_dump())
    except RunNotFound:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="unknown task") from None
    except RunBusy:
        raise HTTPException(status.HTTP_409_CONFLICT, detail="run already active") from None
    return Accepted(task_id=task_id, thread_id=task_id)


@app.post("/runs/{task_id}/continue", status_code=status.HTTP_202_ACCEPTED, response_model=Accepted, dependencies=[Depends(require_internal)])
async def continue_run(task_id: str, mgr: RunManager = Depends(runs)) -> Accepted:
    try:
        await mgr.continue_run(task_id)
    except RunNotFound:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="unknown task") from None
    except RunBusy:
        raise HTTPException(status.HTTP_409_CONFLICT, detail="run already active") from None
    return Accepted(task_id=task_id, thread_id=task_id)


@app.post("/runs/{task_id}/cancel", dependencies=[Depends(require_internal)])
async def cancel_run(task_id: str, mgr: RunManager = Depends(runs)) -> dict[str, Any]:
    cancelled = await mgr.cancel(task_id)
    return {"task_id": task_id, "cancelled": cancelled}


@app.get("/runs/{task_id}/state", dependencies=[Depends(require_internal)])
async def run_state(task_id: str, mgr: RunManager = Depends(runs)) -> dict[str, Any]:
    try:
        return await mgr.state(task_id)
    except RunNotFound:
        raise HTTPException(status.HTTP_404_NOT_FOUND, detail="unknown task") from None
