"""그래프 State 스키마 — 14-Backend 설계 B.3.

부모 TaskState 는 모든 Stage 가 공유한다. FE·BE 처럼 같은 super-step 에서 병렬로 실행되는
Stage 들이 `results` 에 동시에 쓰기 때문에 **키 병합 reducer** 가 필수다. 기본(덮어쓰기)
reducer 를 쓰면 한쪽 결과가 사라진다.
"""
from __future__ import annotations

from operator import add
from typing import Annotated, Any, Literal, TypedDict

Decision = Literal["approve", "reject", "edit"]


class ProjectRef(TypedDict, total=False):
    owner: str
    repo: str
    branch: str
    context_path: str  # 기본 ".devsquad"
    local_path: str  # Phase 1: 로컬 디렉토리 (git clone 은 Phase 2)


class Usage(TypedDict):
    model: str
    input_tokens: int
    output_tokens: int


class StageResult(TypedDict, total=False):
    stage_key: str
    role: str
    plan: str | None
    deliverable_ref: str | None
    deliverable_summary: str | None
    commit_sha: str | None
    approved: bool
    retries: int


def merge_by_key(left: dict[str, StageResult] | None, right: dict[str, StageResult] | None) -> dict[str, StageResult]:
    """stage_key 기준 병합. 같은 키는 오른쪽(새 갱신)이 이긴다."""
    out: dict[str, StageResult] = dict(left or {})
    out.update(right or {})
    return out


class TaskState(TypedDict, total=False):
    task_id: str
    project: ProjectRef
    command: str
    results: Annotated[dict[str, StageResult], merge_by_key]
    messages_log: Annotated[list[str], add]
    usage: Annotated[list[Usage], add]
    error: str | None


class ApprovalDecision(TypedDict, total=False):
    """Command(resume=...) 로 들어오는 값. Control Plane 이 15-API §4 형식으로 보낸다."""

    approval_id: str
    decision: Decision
    feedback: str | None
    edited_content: str | None


class StageState(TypedDict, total=False):
    """Stage 서브그래프의 private state. 부모에는 commit_result 에서 results 로만 반영된다."""

    # 부모에서 넘어오는 것
    task_id: str
    project: ProjectRef
    command: str
    results: Annotated[dict[str, StageResult], merge_by_key]
    # 이 Stage 고유
    stage_key: str
    role: str
    approvals: list[str]
    max_retries: int
    plan: str | None
    plan_retry_no: int
    deliverable_ref: str | None
    deliverable_summary: str | None
    deliverable_retry_no: int
    commit_sha: str | None
    last_feedback: str | None
    blocked_reason: str | None
    messages_log: Annotated[list[str], add]
    usage: Annotated[list[Usage], add]


def interrupt_payload(kind: str, s: StageState) -> dict[str, Any]:
    """interrupt() 에 실어 보내는 값 = approval.requested 이벤트 payload 의 원본 (15-API §3)."""
    retry_no = s.get("plan_retry_no", 0) if kind == "plan" else s.get("deliverable_retry_no", 0)
    content = s.get("plan") if kind == "plan" else s.get("deliverable_summary")
    return {
        "kind": kind,
        "stage_key": s["stage_key"],
        "role": s["role"],
        "retry_no": retry_no,
        "title": f"{s['role']} {kind} v{retry_no + 1}",
        "content": content or "",
        "deliverable_ref": s.get("deliverable_ref"),
    }
