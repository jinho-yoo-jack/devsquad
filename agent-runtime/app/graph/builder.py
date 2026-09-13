"""pipeline.yaml → 부모 StateGraph 동적 조립 — 14-Backend 설계 B.4.

각 Stage 는 컴파일된 서브그래프를 **노드로** 부모에 붙인다. 부모(TaskState)와 서브그래프(StageState)가
task_id/project/command/results/messages_log/usage 키를 공유하므로 LangGraph 가 자동으로 그 키만
넘기고 돌려받는다. depends_on 을 normal edge 로 이으면 의존이 같은 Stage 들은 같은 super-step 에서
병렬 실행된다(BSP). 별도 join 노드는 필요 없지만 END 직전에 하나 둬서 최종 요약을 만든다.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from langgraph.graph import END, START, StateGraph

from app.agents.llm import LLM, make_llm
from app.agents.loader import load_agents_for, load_project_spec
from app.agents.prompt import AgentSpec
from app.graph.pipeline import BUILTIN_AGENTS, PipelineSpec, load_pipeline
from app.graph.stage_subgraph import StageContext, build_stage_subgraph
from app.graph.state import TaskState


@dataclass
class BuildInputs:
    pipeline: PipelineSpec
    agents: dict[str, AgentSpec]
    project_spec: str
    workspace: Path
    llm_factory: callable[[str | None], LLM]
    builtin_agents: dict[str, AgentSpec] = field(default_factory=dict)


def _finalize(s: TaskState) -> dict:
    results = s.get("results") or {}
    blocked = [k for k, r in results.items() if not r.get("approved")]
    line = "모든 Stage 완료" if not blocked else f"차단된 Stage: {', '.join(blocked)}"
    return {"messages_log": [f"[task] {line}"], "error": None if not blocked else f"blocked: {blocked}"}


def build_task_graph(inputs: BuildInputs) -> StateGraph:
    p = inputs.pipeline
    g = StateGraph(TaskState)

    for stage in p.stages:
        if stage.agent in BUILTIN_AGENTS:
            spec = inputs.builtin_agents.get(stage.agent) or _builtin_placeholder(stage.agent)
        else:
            spec = inputs.agents[stage.agent]
        ctx = StageContext(
            stage=stage,
            agent=spec,
            llm=inputs.llm_factory(stage.model or spec.model or p.policy.default_model),
            project_spec=inputs.project_spec,
            workspace=inputs.workspace,
            max_retries=p.policy.max_retries_per_approval,
            max_iterations=p.policy.execute_max_iterations,
        )
        g.add_node(stage.id, build_stage_subgraph(ctx).compile())

    g.add_node("finalize", _finalize)

    for stage in p.stages:
        if not stage.depends_on:
            g.add_edge(START, stage.id)
        for dep in stage.depends_on:
            g.add_edge(dep, stage.id)
    for sid in p.terminal_stages():
        g.add_edge(sid, "finalize")
    g.add_edge("finalize", END)
    return g


def _builtin_placeholder(name: str) -> AgentSpec:
    """publisher 등 내장 팀원. Phase 3 에서 실제 PR 노드로 교체. 지금은 문서형 no-op 팀원."""
    return AgentSpec(
        name=name,
        display_name=name,
        tool_profile="publisher",
        write_paths=["docs/pr/**"],
        persona="# 역할\n서비스 내장 팀원. 승인된 결과물을 모아 PR 을 만든다.",
        conventions="# 4. 결과물 형식\ndocs/pr/<task-slug>.md 에 PR 본문 초안을 남긴다.",
    )


def build_from_context(context_dir: Path, workspace: Path, fake_llm: bool) -> tuple[StateGraph, PipelineSpec]:
    """`.devsquad/` 디렉토리에서 전부 읽어 그래프를 만든다. Task 생성 시 Control Plane 이 부른다."""
    pipeline = load_pipeline(context_dir / "pipeline.yaml")
    agents = load_agents_for(pipeline, context_dir)
    project_spec = load_project_spec(context_dir)
    inputs = BuildInputs(
        pipeline=pipeline,
        agents=agents,
        project_spec=project_spec,
        workspace=workspace,
        llm_factory=lambda model: make_llm(model, fake=fake_llm),
    )
    return build_task_graph(inputs), pipeline
