"""Agent Runtime — 14-Backend 설계 Part B.

골격 단계(Phase 0): /health 와 /runs 라우트 시그니처만. 실제 그래프 실행은 AR-5~12 에서.
"""
from datetime import datetime, timezone

from fastapi import FastAPI, Header, HTTPException, status
from pydantic import BaseModel

from app.settings import settings

app = FastAPI(title="DevSquad Agent Runtime", version="0.0.1")


def _require_internal(token: str | None) -> None:
    if token != settings.internal_token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid internal token")


class HealthResponse(BaseModel):
    status: str
    service: str
    active_runs: int
    ts: str


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    return HealthResponse(
        status="ok",
        service="agent-runtime",
        active_runs=0,
        ts=datetime.now(timezone.utc).isoformat(),
    )


class RunRequest(BaseModel):
    task_id: str
    command: str
    project: dict
    options: dict = {}


@app.post("/runs", status_code=status.HTTP_501_NOT_IMPLEMENTED)
def create_run(body: RunRequest, x_internal_token: str | None = Header(default=None)) -> dict:
    """15-API 명세 §4. AR-12 에서 구현."""
    _require_internal(x_internal_token)
    return {"detail": "not implemented yet (AR-12)", "task_id": body.task_id}
