"""`.devsquad/agents/<name>/` → AgentSpec — 17-Agent 정의 가이드 §2.

Phase 1 범위: persona.md 프론트매터 파싱, conventions.md, knowledge/ 전부 로드, 필수 검증.
토큰 예산에 따른 knowledge 선별·임베딩 검색은 Phase 2.
"""
from __future__ import annotations

import fnmatch
from pathlib import Path

import yaml

from app.agents.prompt import AgentSpec
from app.graph.pipeline import PipelineSpec

TOOL_PROFILES = {"docs-writer", "code-writer", "reader", "publisher"}
SECRET_PATTERNS = (".env", ".env.*", "*.pem", "*.key", "secrets/*", ".git/config")


class AgentDefinitionError(ValueError):
    """사용자에게 400 으로 돌려줄 팀원 정의 오류."""


def split_frontmatter(text: str) -> tuple[dict, str]:
    if not text.startswith("---"):
        return {}, text
    parts = text.split("---", 2)
    if len(parts) < 3:
        return {}, text
    fm = yaml.safe_load(parts[1]) or {}
    if not isinstance(fm, dict):
        raise AgentDefinitionError("persona.md 프론트매터는 매핑이어야 합니다")
    return fm, parts[2]


def load_agent(agents_dir: Path, name: str) -> AgentSpec:
    d = agents_dir / name
    if not d.is_dir():
        raise AgentDefinitionError(f"팀원 폴더가 없습니다: agents/{name}/")
    persona_path, conv_path = d / "persona.md", d / "conventions.md"
    for p in (persona_path, conv_path):
        if not p.exists():
            raise AgentDefinitionError(f"agents/{name}/{p.name} 이 없습니다")

    fm, persona_body = split_frontmatter(persona_path.read_text(encoding="utf-8"))
    if fm.get("name") != name:
        raise AgentDefinitionError(f"agents/{name}/persona.md 의 name('{fm.get('name')}') 이 폴더명과 다릅니다")
    profile = fm.get("tool_profile")
    if profile not in TOOL_PROFILES:
        raise AgentDefinitionError(f"agents/{name}: tool_profile 은 {sorted(TOOL_PROFILES)} 중 하나여야 합니다 (현재 {profile!r})")
    write_paths = list(fm.get("write_paths") or [])
    if profile in ("docs-writer", "code-writer") and not write_paths:
        raise AgentDefinitionError(f"agents/{name}: {profile} 는 write_paths 가 필요합니다")
    if profile in ("reader", "publisher") and write_paths:
        # reader 도 docs/review/** 처럼 리포트를 남길 수 있으므로 경고 수준으로 허용. 단 코드 경로는 금지.
        for wp in write_paths:
            if not wp.startswith("docs/"):
                raise AgentDefinitionError(f"agents/{name}: {profile} 는 docs/ 밖에 쓸 수 없습니다 ({wp})")
    for wp in write_paths:
        if wp.startswith("/") or ".." in wp:
            raise AgentDefinitionError(f"agents/{name}: write_paths 는 저장소 상대 경로여야 합니다 ({wp})")

    knowledge: dict[str, str] = {}
    kdir = d / "knowledge"
    if kdir.is_dir():
        for f in sorted(kdir.rglob("*")):
            if f.is_file() and not any(fnmatch.fnmatch(f.name, pat) for pat in SECRET_PATTERNS):
                knowledge[str(f.relative_to(d))] = f.read_text(encoding="utf-8", errors="replace")

    return AgentSpec(
        name=name,
        display_name=str(fm.get("display_name") or name),
        tool_profile=profile,
        write_paths=write_paths,
        persona=persona_body,
        conventions=conv_path.read_text(encoding="utf-8"),
        knowledge=knowledge,
        model=fm.get("model"),
        test_command=fm.get("test_command"),
    )


def load_agents_for(pipeline: PipelineSpec, context_dir: Path) -> dict[str, AgentSpec]:
    """pipeline 이 참조하는 팀원을 모두 로드하고, write_paths 충돌을 검사한다 (FR-25)."""
    agents_dir = context_dir / "agents"
    specs: dict[str, AgentSpec] = {}
    for name in sorted(pipeline.required_agents()):
        specs[name] = load_agent(agents_dir, name)
    _check_write_conflicts(specs)
    return specs


def _check_write_conflicts(specs: dict[str, AgentSpec]) -> None:
    """두 팀원이 같은 경로 패턴에 쓰면 병렬 실행 시 충돌한다. 동일/포함 관계 패턴을 거절."""
    owners: list[tuple[str, str]] = [(n, wp) for n, s in specs.items() for wp in s.write_paths]
    for i, (n1, p1) in enumerate(owners):
        for n2, p2 in owners[i + 1 :]:
            if n1 == n2:
                continue
            if p1 == p2 or fnmatch.fnmatch(p1.replace("**", "x/y"), p2) or fnmatch.fnmatch(p2.replace("**", "x/y"), p1):
                raise AgentDefinitionError(f"write_paths 충돌: {n1}({p1}) ↔ {n2}({p2})")


def load_project_spec(context_dir: Path) -> str:
    p = context_dir / "spec.md"
    return p.read_text(encoding="utf-8") if p.exists() else "(spec.md 없음)"
