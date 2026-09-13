"""한 Stage 의 서브그래프 — 14-Backend 설계 B.5.

    START → init → plan → plan_gate ─(approve/edit)→ execute → deliverable_gate ─(approve/edit)→ commit_result → END
                      ▲       │(reject, retry<max)   ▲              │(reject, retry<max)
                      └───────┘                      └──────────────┘
                              │(retry≥max)                          │(retry≥max)
                              └──────────────► blocked ◄────────────┘

핵심 규칙
- gate 노드는 LLM 도 도구도 호출하지 않는다. interrupt() 재개 시 노드가 처음부터 재실행되어도
  부작용이 없어 멱등하다.
- 부작용(파일 쓰기)은 execute 에서, push 같은 외부 부작용은 commit_result(승인 이후) 에서만.
- approvals 에 'plan' 이 없으면 plan_gate 를, 'deliverable' 이 없으면 deliverable_gate 를 건너뛴다.
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Literal

from langgraph.config import get_stream_writer
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command, interrupt

from app.agents.executor import run_react
from app.agents.llm import LLM, Message
from app.agents.prompt import AgentSpec, build_system, build_user
from app.graph.pipeline import StageSpec
from app.graph.state import StageResult, StageState, interrupt_payload
from app.tools.registry import tools_for
from app.tools.sandbox import Sandbox

GateKind = Literal["plan", "deliverable"]


@dataclass
class StageContext:
    stage: StageSpec
    agent: AgentSpec
    llm: LLM
    project_spec: str
    workspace: Path
    max_retries: int
    max_iterations: int = 40


def _emit(event: dict) -> None:
    """stream_mode='custom' 구독자(stream_bridge)에게 이벤트를 흘린다. 스트리밍이 아니면 no-op."""
    try:
        writer = get_stream_writer()
    except RuntimeError:  # 스트림(invoke/stream) 밖에서 호출된 경우 — 이벤트를 버린다
        return
    writer(event)


def _prior_inputs(s: StageState, stage: StageSpec) -> dict[str, str]:
    """선행 Stage 의 승인된 산출물 요약을 <inputs> 로. 원문 로딩은 AR-7(conventions §1) 에서."""
    out: dict[str, str] = {}
    for dep in stage.depends_on:
        r = (s.get("results") or {}).get(dep)
        if r and r.get("approved"):
            ref = r.get("deliverable_ref") or ""
            out[f"{dep} ({r.get('role')})"] = f"{ref}\n{r.get('deliverable_summary') or ''}"
    return out


def build_stage_subgraph(ctx: StageContext):
    stage, agent = ctx.stage, ctx.agent
    wants_plan_gate = "plan" in stage.approvals
    wants_deliverable_gate = "deliverable" in stage.approvals

    # ---- 노드 ---------------------------------------------------------------

    def init(s: StageState) -> dict:
        _emit({"type": "stage.started", "stage_key": stage.id, "agent": agent.name, "payload": {}})
        return {
            "stage_key": stage.id,
            "role": agent.name,
            "approvals": list(stage.approvals),
            "max_retries": ctx.max_retries,
            "plan_retry_no": s.get("plan_retry_no", 0),
            "deliverable_retry_no": s.get("deliverable_retry_no", 0),
        }

    def plan(s: StageState) -> dict:
        system = build_system(agent, ctx.project_spec, "plan", tools_desc="(plan 단계: 도구 없음)")
        user = build_user("plan", s["command"], _prior_inputs(s, stage), feedback=s.get("last_feedback"))
        _emit({"type": "agent.thinking", "stage_key": stage.id, "agent": agent.name, "payload": {"summary": "계획을 작성하는 중"}})
        r = ctx.llm.chat(system, [Message(role="user", content=user)], tools=None)
        _emit({"type": "usage", "stage_key": stage.id, "agent": agent.name,
               "payload": {"model": r.model, "input_tokens": r.input_tokens, "output_tokens": r.output_tokens}})
        return {
            "plan": r.text,
            "usage": [{"model": r.model, "input_tokens": r.input_tokens, "output_tokens": r.output_tokens}],
        }

    def make_gate(kind: GateKind):
        def gate(s: StageState) -> Command:
            payload = interrupt_payload(kind, s)
            _emit({"type": "approval.requested", "stage_key": stage.id, "agent": agent.name, "payload": payload})
            decision = interrupt(payload)  # ← 여기서 멈춤. 재개 시 이 노드가 처음부터 다시 실행된다.
            if not isinstance(decision, dict) or decision.get("decision") not in ("approve", "reject", "edit"):
                raise ValueError(f"잘못된 승인 응답: {decision!r}")

            retry_key = "plan_retry_no" if kind == "plan" else "deliverable_retry_no"
            retry_no = s.get(retry_key, 0)

            if decision["decision"] == "reject":
                feedback = (decision.get("feedback") or "").strip()
                if retry_no + 1 >= s.get("max_retries", ctx.max_retries):
                    _emit({"type": "stage.blocked", "stage_key": stage.id, "agent": agent.name,
                           "payload": {"reason": f"{kind} 반려 {retry_no + 1}회 — 상한 도달", "last_feedback": feedback}})
                    return Command(goto="blocked", update={"blocked_reason": f"{kind} 반려 상한 도달", "last_feedback": feedback})
                return Command(
                    goto="plan" if kind == "plan" else "execute",
                    update={retry_key: retry_no + 1, "last_feedback": feedback},
                )

            update: dict = {"last_feedback": None}
            if decision["decision"] == "edit" and decision.get("edited_content"):
                update["plan" if kind == "plan" else "deliverable_summary"] = decision["edited_content"]
            return Command(goto="execute" if kind == "plan" else "commit_result", update=update)

        gate.__name__ = f"{kind}_gate"
        return gate

    def execute(s: StageState) -> dict:
        """ReAct 루프: tool_profile 화이트리스트 안에서 도구를 부르며 결과물을 만든다 (AR-8/9)."""
        sandbox = Sandbox(ctx.workspace, write_paths=agent.write_paths, read_only=agent.tool_profile == "publisher")
        tools = tools_for(agent.tool_profile, sandbox, agent.test_command)
        tools_desc = "\n".join(f"- {t.name}: {t.description}" for t in tools) + f"\n쓰기 허용 경로: {agent.write_paths}"
        system = build_system(agent, ctx.project_spec, "execute", tools_desc=tools_desc)
        user = build_user("execute", s["command"], _prior_inputs(s, stage), plan=s.get("plan"), feedback=s.get("last_feedback"))
        _emit({"type": "agent.thinking", "stage_key": stage.id, "agent": agent.name, "payload": {"summary": "승인된 계획을 수행하는 중"}})

        res = run_react(ctx.llm, system, user, tools, ctx.max_iterations, _emit, stage.id, agent.name)

        primary = res.written_paths[-1] if res.written_paths else None
        if primary:
            content = sandbox.read_file(primary)
            header = f"> Task: {s['task_id']} · 작성: {agent.name} · 버전: v{s.get('deliverable_retry_no', 0) + 1}\n\n"
            if not content.startswith("> Task:"):
                sandbox.write_file(primary, header + content)
                content = header + content
        else:
            # 파일을 하나도 못 썼으면 최종 텍스트 자체가 결과물
            content = res.final_text
        summary = _summarize(content)
        _emit({"type": "deliverable.produced", "stage_key": stage.id, "agent": agent.name,
               "payload": {"kind": "markdown", "uri": primary, "summary": summary,
                           "written_paths": res.written_paths, "iterations": res.iterations, "hit_limit": res.hit_limit}})
        return {
            "deliverable_ref": primary,
            "deliverable_summary": content,
            "usage": res.usage,
        }

    def commit_result(s: StageState) -> dict:
        result: StageResult = {
            "stage_key": stage.id,
            "role": agent.name,
            "plan": s.get("plan"),
            "deliverable_ref": s.get("deliverable_ref"),
            "deliverable_summary": _summarize(s.get("deliverable_summary") or ""),
            "commit_sha": s.get("commit_sha"),
            "approved": True,
            "retries": s.get("plan_retry_no", 0) + s.get("deliverable_retry_no", 0),
        }
        _emit({"type": "stage.completed", "stage_key": stage.id, "agent": agent.name,
               "payload": {"deliverable_ref": s.get("deliverable_ref")}})
        return {"results": {stage.id: result}, "messages_log": [f"[{stage.id}] 완료 → {s.get('deliverable_ref')}"]}

    def blocked(s: StageState) -> dict:
        result: StageResult = {"stage_key": stage.id, "role": agent.name, "approved": False,
                               "plan": s.get("plan"), "deliverable_ref": s.get("deliverable_ref")}
        return {"results": {stage.id: result},
                "messages_log": [f"[{stage.id}] blocked: {s.get('blocked_reason')}"]}

    # ---- 배선 ---------------------------------------------------------------

    g = StateGraph(StageState)
    g.add_node("init", init)
    g.add_node("plan", plan)
    g.add_node("execute", execute)
    g.add_node("commit_result", commit_result)
    g.add_node("blocked", blocked)
    if wants_plan_gate:
        g.add_node("plan_gate", make_gate("plan"), destinations=("plan", "execute", "blocked"))
    if wants_deliverable_gate:
        g.add_node("deliverable_gate", make_gate("deliverable"), destinations=("execute", "commit_result", "blocked"))

    g.add_edge(START, "init")
    g.add_edge("init", "plan")
    g.add_edge("plan", "plan_gate" if wants_plan_gate else "execute")
    # gate 는 Command 로 라우팅하므로 나가는 normal edge 를 두지 않는다 (섞으면 안 됨).
    g.add_edge("execute", "deliverable_gate" if wants_deliverable_gate else "commit_result")
    g.add_edge("commit_result", END)
    g.add_edge("blocked", END)
    return g


# ---- 유틸 ------------------------------------------------------------------


def _summarize(text: str, limit: int = 400) -> str:
    text = text.strip()
    return text if len(text) <= limit else text[: limit - 1] + "…"
