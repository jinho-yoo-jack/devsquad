"""System/User 프롬프트 조립 — 17-Agent 정의 가이드 §3.

순서와 구분자는 고정이다(사용자가 알고 파일을 쓴다):
<persona> <conventions> <project> <mission> <inputs> <knowledge> <feedback> <tools>

Phase 1(AR-7 전) 에는 loader 가 없으므로 AgentSpec 을 직접 받는다.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

Mode = Literal["plan", "execute"]


@dataclass
class AgentSpec:
    name: str
    display_name: str
    tool_profile: str
    write_paths: list[str]
    persona: str
    conventions: str
    knowledge: dict[str, str] = field(default_factory=dict)  # path -> content
    model: str | None = None
    test_command: str | None = None


MISSION = {
    "plan": (
        "지금은 **계획(plan) 단계**다. conventions §2 형식으로 Plan 만 작성한다. "
        "도구를 쓰지 않고, 파일을 만들지 않는다. 사람이 30초 안에 읽고 승인할 수 있게 짧게 쓴다."
    ),
    "execute": (
        "지금은 **실행(execute) 단계**다. 승인된 Plan 을 수행하고 conventions §4 형식으로 결과물을 남긴다. "
        "끝나기 전에 conventions §5 완료 기준으로 자가 점검한다. 반려 피드백이 있으면 결과물 상단에 "
        "'## 반려 반영 내역' 절을 둔다."
    ),
}


def build_system(spec: AgentSpec, project_spec: str, mode: Mode, tools_desc: str) -> str:
    knowledge = "\n\n".join(f"### {p}\n{c}" for p, c in spec.knowledge.items()) or "(없음)"
    return (
        f"<persona>\n{spec.persona.strip()}\n</persona>\n\n"
        f"<conventions>\n{spec.conventions.strip()}\n</conventions>\n\n"
        f"<project>\n{project_spec.strip()}\n</project>\n\n"
        f"<mission>\n{MISSION[mode]}\n</mission>\n\n"
        f"<knowledge>\n{knowledge}\n</knowledge>\n\n"
        f"<tools>\n{tools_desc}\n</tools>"
    )


def build_user(
    mode: Mode,
    command: str,
    inputs: dict[str, str],
    plan: str | None = None,
    feedback: str | None = None,
) -> str:
    """User 메시지. FakeLLM 이 파싱할 수 있게 [[TAG]] 구분자를 쓴다 (실제 LLM 에도 무해)."""
    parts = [f"[[MODE:{mode}]]", f"[[COMMAND]]\n{command}\n[[/COMMAND]]"]
    if inputs:
        joined = "\n\n".join(f"### {k}\n{v}" for k, v in inputs.items())
        parts.append(f"<inputs>\n{joined}\n</inputs>")
    if mode == "execute" and plan:
        parts.append(f"<approved_plan>\n{plan}\n</approved_plan>")
    if feedback:
        parts.append(f"<feedback>\n이전 제출은 다음 이유로 반려되었다. 반드시 반영한다.\n[[FEEDBACK]]{feedback}[[/FEEDBACK]]\n</feedback>")
    return "\n\n".join(parts)
