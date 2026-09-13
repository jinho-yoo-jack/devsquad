"""builder: 템플릿 .devsquad/ 로 그래프를 조립하고, Phase 1 파이프라인(planning 1개)을 끝까지 돌린다.
전체 6-Stage 템플릿은 구조(노드·엣지)만 스냅샷 검증한다 — 병렬 FE/BE 실행 검증은 AR-13 통합 테스트."""
from pathlib import Path

from langgraph.checkpoint.memory import InMemorySaver
from langgraph.types import Command

from app.agents.llm import FakeLLM
from app.agents.loader import AgentDefinitionError, load_agents_for, load_project_spec
from app.graph.builder import BuildInputs, build_from_context, build_task_graph
from app.graph.pipeline import load_pipeline, parse_pipeline

TEMPLATES = Path(__file__).resolve().parents[2] / "examples" / "devsquad-templates" / ".devsquad"


def test_full_template_graph_structure(tmp_path):
    g, _pipeline = build_from_context(TEMPLATES, tmp_path, fake_llm=True)
    compiled = g.compile()
    graph = compiled.get_graph()
    node_ids = set(graph.nodes)
    assert {"planning", "design", "backend", "frontend", "review", "pr", "finalize"} <= node_ids
    edges = {(e.source, e.target) for e in graph.edges}
    assert ("planning", "design") in edges
    assert ("design", "backend") in edges and ("design", "frontend") in edges  # 병렬 fan-out
    assert ("backend", "review") in edges and ("frontend", "review") in edges  # fan-in
    assert ("pr", "finalize") in edges


def test_template_agents_load_and_no_write_conflicts():
    pipeline = load_pipeline(TEMPLATES / "pipeline.yaml")
    agents = load_agents_for(pipeline, TEMPLATES)
    assert set(agents) == {"planner", "designer", "backend", "frontend", "reviewer"}
    assert agents["backend"].tool_profile == "code-writer"
    assert "db-schema.md" in " ".join(agents["backend"].knowledge)
    assert "# 역할" in agents["planner"].persona
    assert load_project_spec(TEMPLATES).startswith("# 프로젝트 공통 컨텍스트")


def test_missing_agent_folder_is_rejected(tmp_path):
    pipeline = parse_pipeline("version: 1\nstages:\n  - {id: x, agent: ghost}\n")
    try:
        load_agents_for(pipeline, TEMPLATES)
    except AgentDefinitionError as e:
        assert "ghost" in str(e)
    else:  # pragma: no cover
        raise AssertionError("없는 팀원이 통과했습니다")


def test_phase1_single_stage_pipeline_end_to_end(tmp_path):
    """Phase 1 DoD 1~4: 명령 → Plan 승인 → 실행 → Deliverable 승인 → completed."""
    pipeline = parse_pipeline("version: 1\nstages:\n  - {id: planning, agent: planner}\n")
    agents = load_agents_for(pipeline, TEMPLATES)
    llm = FakeLLM()
    g = build_task_graph(BuildInputs(
        pipeline=pipeline, agents=agents, project_spec=load_project_spec(TEMPLATES),
        workspace=tmp_path, llm_factory=lambda _m: llm,
    )).compile(checkpointer=InMemorySaver())
    cfg = {"configurable": {"thread_id": "task-e2e"}}

    r = g.invoke({"task_id": "task-e2e", "command": "설정 페이지에 다크 모드 토글 추가", "project": {}}, cfg)
    assert r["__interrupt__"][0].value["kind"] == "plan"

    r = g.invoke(Command(resume={"decision": "approve"}), cfg)
    assert r["__interrupt__"][0].value["kind"] == "deliverable"

    r = g.invoke(Command(resume={"decision": "approve"}), cfg)
    assert "__interrupt__" not in r
    assert r["results"]["planning"]["approved"] is True
    assert r["error"] is None
    assert any("모든 Stage 완료" in m for m in r["messages_log"])
    written = list((tmp_path / "docs" / "spec").glob("*.md"))
    assert len(written) == 1 and "Task: task-e2e" in written[0].read_text(encoding="utf-8")
    # usage 가 reducer(add) 로 누적됨: plan 1 + execute 2턴
    assert len(r["usage"]) == 3

    # 재개(resume) 후 상태 조회 — Control Plane 복구 잡이 쓰는 형태
    snap = g.get_state(cfg)
    assert snap.next == () and not snap.tasks
