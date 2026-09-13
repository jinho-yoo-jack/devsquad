"""gate 노드의 세 경로(approve / reject→재제출 / edit)와 반려 상한 → blocked 를 검증.

InMemorySaver + thread_id 로 interrupt → Command(resume) 를 실제로 돌린다.
"""
from pathlib import Path

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.types import Command

from app.agents.llm import FakeLLM
from app.agents.prompt import AgentSpec
from app.graph.pipeline import StageSpec
from app.graph.stage_subgraph import StageContext, build_stage_subgraph


def _agent() -> AgentSpec:
    return AgentSpec(
        name="planner", display_name="기획자", tool_profile="docs-writer", write_paths=["docs/spec/**"],
        persona="# 역할\n테스트 기획자", conventions="# 4. 결과물 형식\ndocs/spec/<slug>.md",
    )


def _graph(tmp_path: Path, approvals=("plan", "deliverable"), max_retries=3):
    llm = FakeLLM()
    ctx = StageContext(
        stage=StageSpec(id="planning", agent="planner", approvals=list(approvals)),
        agent=_agent(), llm=llm, project_spec="# 테스트 프로젝트", workspace=tmp_path, max_retries=max_retries,
    )
    g = build_stage_subgraph(ctx).compile(checkpointer=InMemorySaver())
    return g, llm


def _cfg(tid="t-1"):
    return {"configurable": {"thread_id": tid}}


def _start(g, cfg):
    return g.invoke({"task_id": "task-1", "command": "회원 탈퇴 기능 추가", "project": {}}, cfg)


def _interrupt(result):
    ints = result.get("__interrupt__")
    assert ints, f"interrupt 가 없습니다: {list(result)}"
    return ints[0].value


def test_happy_path_two_approvals(tmp_path):
    g, llm = _graph(tmp_path)
    cfg = _cfg()

    r = _start(g, cfg)
    p = _interrupt(r)
    assert p["kind"] == "plan" and p["retry_no"] == 0 and p["title"] == "planner plan v1"
    assert "docs/spec/" in p["content"]
    assert len(llm.calls) == 1  # plan 1회

    r = g.invoke(Command(resume={"decision": "approve"}), cfg)
    d = _interrupt(r)
    assert d["kind"] == "deliverable" and d["deliverable_ref"].startswith("docs/spec/")
    assert (tmp_path / d["deliverable_ref"]).exists()
    assert len(llm.calls) == 3  # execute = tool-call 턴 + 마무리 턴. gate 재실행이 LLM 을 다시 부르지 않음

    r = g.invoke(Command(resume={"decision": "approve"}), cfg)
    assert "__interrupt__" not in r
    res = r["results"]["planning"]
    assert res["approved"] is True and res["retries"] == 0
    assert len(llm.calls) == 3


def test_reject_plan_then_resubmit_with_feedback(tmp_path):
    g, llm = _graph(tmp_path)
    cfg = _cfg()
    _start(g, cfg)

    r = g.invoke(Command(resume={"decision": "reject", "feedback": "유예 기간 복구 모달 필요"}), cfg)
    p = _interrupt(r)
    assert p["kind"] == "plan" and p["retry_no"] == 1 and p["title"] == "planner plan v2"
    assert "반려 반영" in p["content"] and "복구 모달" in p["content"]
    # 두 번째 plan 호출의 user 프롬프트에 피드백이 들어갔는지
    assert "[[FEEDBACK]]유예 기간 복구 모달 필요" in llm.calls[-1][1]


def test_edit_plan_replaces_content(tmp_path):
    g, _ = _graph(tmp_path)
    cfg = _cfg()
    _start(g, cfg)
    r = g.invoke(Command(resume={"decision": "edit", "edited_content": "## 사람이 고친 계획\n1. docs/spec/custom.md"}), cfg)
    _interrupt(r)  # deliverable gate
    r = g.invoke(Command(resume={"decision": "approve"}), cfg)
    res = r["results"]["planning"]
    assert res["plan"].startswith("## 사람이 고친 계획")
    assert res["deliverable_ref"] == "docs/spec/custom.md"  # 수정된 Plan 의 경로를 따라갔다


def test_reject_deliverable_reexecutes_only(tmp_path):
    g, llm = _graph(tmp_path)
    cfg = _cfg()
    _start(g, cfg)
    g.invoke(Command(resume={"decision": "approve"}), cfg)
    calls_before = len(llm.calls)
    r = g.invoke(Command(resume={"decision": "reject", "feedback": "예외 흐름 빠짐"}), cfg)
    d = _interrupt(r)
    assert d["kind"] == "deliverable" and d["retry_no"] == 1
    assert len(llm.calls) == calls_before + 2  # execute(2턴) 만 다시, plan 은 안 함
    assert "예외 흐름 빠짐" in d["content"]


def test_retry_cap_blocks_stage(tmp_path):
    g, _ = _graph(tmp_path, max_retries=2)
    cfg = _cfg()
    _start(g, cfg)
    r = g.invoke(Command(resume={"decision": "reject", "feedback": "1차"}), cfg)
    assert _interrupt(r)["retry_no"] == 1
    r = g.invoke(Command(resume={"decision": "reject", "feedback": "2차"}), cfg)
    assert "__interrupt__" not in r
    res = r["results"]["planning"]
    assert res["approved"] is False
    assert any("blocked" in m for m in r["messages_log"])


def test_no_plan_gate_when_not_requested(tmp_path):
    g, llm = _graph(tmp_path, approvals=("deliverable",))
    cfg = _cfg()
    r = _start(g, cfg)
    d = _interrupt(r)
    assert d["kind"] == "deliverable"  # plan gate 없이 바로 실행됨
    assert len(llm.calls) == 3


def test_invalid_resume_value_raises(tmp_path):
    g, _ = _graph(tmp_path)
    cfg = _cfg()
    _start(g, cfg)
    with pytest.raises(ValueError):
        g.invoke(Command(resume={"decision": "maybe"}), cfg)


def test_execute_denied_write_is_recovered_via_observation(tmp_path):
    """LLM 이 write_paths 밖에 쓰려 하면 도구가 [denied] Observation 을 돌려주고, LLM 이 고쳐 다시 쓴다."""
    from app.agents.llm import FakeLLM, LLMTurn, ToolCall

    class EvilFirstLLM(FakeLLM):
        def chat(self, system, messages, tools=None):
            if tools and not any(m.role == "tool" for m in messages):
                self.calls.append((system, messages[0].content))
                return LLMTurn(text="서버 코드를 고치겠습니다", model=self.model,
                               tool_calls=[ToolCall(id="c0", name="write_file", args={"path": "server/Evil.java", "content": "x"})])
            return super().chat(system, messages, tools)

    llm = EvilFirstLLM()
    ctx = StageContext(stage=StageSpec(id="planning", agent="planner", approvals=["deliverable"]),
                       agent=_agent(), llm=llm, project_spec="p", workspace=tmp_path, max_retries=3)
    g = build_stage_subgraph(ctx).compile(checkpointer=InMemorySaver())
    cfg = _cfg("t-evil")
    r = _start(g, cfg)
    d = _interrupt(r)
    assert not (tmp_path / "server" / "Evil.java").exists()
    assert d["deliverable_ref"] == "docs/spec/fallback.md"
    assert (tmp_path / "docs/spec/fallback.md").exists()
