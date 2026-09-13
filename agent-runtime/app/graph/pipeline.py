"""pipeline.yaml 파서와 검증 — 15-API 명세 §6, 17-Agent 정의 가이드 §1.

pipeline.yaml 은 사용자 저장소의 .devsquad/ 에 있고, 사용자가 팀원(Stage)을 자유롭게
추가·제거한다. 여기서는 (1) 스키마 검증, (2) DAG 검증, (3) 위상 정렬(super-step 레벨) 을 한다.
agent 폴더 존재·tool_profile·write_paths 충돌은 agents.loader 가 검증한다 (AR-7).
"""
from __future__ import annotations

from collections import deque
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, Field, ValidationError, field_validator, model_validator

ApprovalKind = Literal["plan", "deliverable"]
BUILTIN_AGENTS = {"publisher"}  # 서비스 내장 팀원: agents/ 폴더 불필요


class PipelineError(ValueError):
    """사용자에게 400 으로 돌려줄 검증 오류."""


class StageSpec(BaseModel):
    id: str = Field(pattern=r"^[a-z][a-z0-9_]*$")
    agent: str = Field(min_length=1)
    depends_on: list[str] = Field(default_factory=list)
    approvals: list[ApprovalKind] = Field(default_factory=lambda: ["plan", "deliverable"])
    tools: list[str] | None = None
    model: str | None = None

    @field_validator("approvals")
    @classmethod
    def _unique_approvals(cls, v: list[str]) -> list[str]:
        if len(set(v)) != len(v):
            raise ValueError("approvals 에 중복이 있습니다")
        return v


class PrPolicy(BaseModel):
    mode: Literal["per-role", "single"] = "per-role"
    base_branch: str = "main"


class Policy(BaseModel):
    max_retries_per_approval: int = Field(default=3, ge=1, le=10)
    token_budget: int | None = Field(default=None, ge=1)
    execute_max_iterations: int = Field(default=40, ge=1, le=500)
    default_model: str | None = None
    pr: PrPolicy = Field(default_factory=PrPolicy)


class PipelineSpec(BaseModel):
    version: Literal[1]
    stages: list[StageSpec] = Field(min_length=1)
    policy: Policy = Field(default_factory=Policy)

    @model_validator(mode="after")
    def _validate_graph(self) -> PipelineSpec:
        ids = [s.id for s in self.stages]
        dupes = {i for i in ids if ids.count(i) > 1}
        if dupes:
            raise ValueError(f"stage id 중복: {sorted(dupes)}")
        known = set(ids)
        for s in self.stages:
            for d in s.depends_on:
                if d not in known:
                    raise ValueError(f"stage '{s.id}' 의 depends_on '{d}' 가 존재하지 않습니다")
                if d == s.id:
                    raise ValueError(f"stage '{s.id}' 가 자기 자신에 의존합니다")
        # 사이클 검사는 levels() 가 수행 (Kahn). 여기서 호출해 실패를 앞당긴다.
        self.levels()
        return self

    # ---- 그래프 유틸 ---------------------------------------------------------

    def by_id(self) -> dict[str, StageSpec]:
        return {s.id: s for s in self.stages}

    def levels(self) -> list[list[str]]:
        """Kahn 위상 정렬을 레벨 단위로. 같은 레벨 = 같은 super-step 에서 병렬 실행.

        사이클이 있으면 PipelineError.
        """
        by_id = self.by_id()
        indeg = {s.id: len(s.depends_on) for s in self.stages}
        children: dict[str, list[str]] = {s.id: [] for s in self.stages}
        for s in self.stages:
            for d in s.depends_on:
                children[d].append(s.id)

        level = sorted(i for i, n in indeg.items() if n == 0)
        out: list[list[str]] = []
        seen = 0
        q = deque(level)
        while q:
            current = list(q)
            q.clear()
            out.append(current)
            seen += len(current)
            nxt: list[str] = []
            for sid in current:
                for c in children[sid]:
                    indeg[c] -= 1
                    if indeg[c] == 0:
                        nxt.append(c)
            q.extend(sorted(nxt))
        if seen != len(by_id):
            remaining = sorted(i for i, n in indeg.items() if n > 0)
            raise PipelineError(f"pipeline 에 사이클이 있습니다: {remaining}")
        return out

    def terminal_stages(self) -> list[str]:
        """아무도 의존하지 않는 Stage — END 로 연결된다."""
        depended = {d for s in self.stages for d in s.depends_on}
        return [s.id for s in self.stages if s.id not in depended]

    def required_agents(self) -> set[str]:
        return {s.agent for s in self.stages} - BUILTIN_AGENTS


# ---- 로딩 --------------------------------------------------------------------


def parse_pipeline(text: str) -> PipelineSpec:
    try:
        raw = yaml.safe_load(text)
    except yaml.YAMLError as e:  # pragma: no cover - 메시지 포맷만
        raise PipelineError(f"pipeline.yaml 파싱 실패: {e}") from e
    if not isinstance(raw, dict):
        raise PipelineError("pipeline.yaml 최상위는 매핑이어야 합니다")
    try:
        return PipelineSpec.model_validate(raw)
    except ValidationError as e:
        raise PipelineError(_format_validation_error(e)) from e
    except ValueError as e:
        raise PipelineError(str(e)) from e


def load_pipeline(path: Path) -> PipelineSpec:
    if not path.exists():
        raise PipelineError(f"{path} 가 없습니다")
    return parse_pipeline(path.read_text(encoding="utf-8"))


def _format_validation_error(e: ValidationError) -> str:
    parts = []
    for err in e.errors():
        loc = ".".join(str(p) for p in err["loc"])
        parts.append(f"{loc}: {err['msg']}")
    return "pipeline.yaml 검증 실패 — " + "; ".join(parts)
