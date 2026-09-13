from pathlib import Path

import pytest

from app.graph.pipeline import PipelineError, load_pipeline, parse_pipeline

TEMPLATES = Path(__file__).resolve().parents[2] / "examples" / "devsquad-templates" / ".devsquad"


def test_template_pipeline_parses_and_levels():
    p = load_pipeline(TEMPLATES / "pipeline.yaml")
    assert [s.id for s in p.stages] == ["planning", "design", "backend", "frontend", "review", "pr"]
    # 같은 레벨 = 같은 super-step 에서 병렬
    assert p.levels() == [["planning"], ["design"], ["backend", "frontend"], ["review"], ["pr"]]
    assert p.terminal_stages() == ["pr"]
    assert p.required_agents() == {"planner", "designer", "backend", "frontend", "reviewer"}  # publisher 는 내장


def test_defaults_and_minimal_pipeline():
    p = parse_pipeline("version: 1\nstages:\n  - id: planning\n    agent: planner\n")
    s = p.stages[0]
    assert s.approvals == ["plan", "deliverable"]
    assert p.policy.max_retries_per_approval == 3
    assert p.levels() == [["planning"]]


@pytest.mark.parametrize(
    "yaml_text, needle",
    [
        ("version: 2\nstages: [{id: a, agent: x}]", "version"),
        ("version: 1\nstages: []", "stages"),
        ("version: 1\nstages:\n  - {id: a, agent: x}\n  - {id: a, agent: y}", "중복"),
        ("version: 1\nstages:\n  - {id: a, agent: x, depends_on: [zzz]}", "존재하지"),
        ("version: 1\nstages:\n  - {id: a, agent: x, depends_on: [b]}\n  - {id: b, agent: y, depends_on: [a]}", "사이클"),
        ("version: 1\nstages:\n  - {id: A-1, agent: x}", "id"),
        ("version: 1\nstages:\n  - {id: a, agent: x, approvals: [plan, plan]}", "중복"),
        ("- just\n- a list", "매핑"),
    ],
)
def test_invalid_pipelines(yaml_text, needle):
    with pytest.raises(PipelineError) as ei:
        parse_pipeline(yaml_text)
    assert needle in str(ei.value)
