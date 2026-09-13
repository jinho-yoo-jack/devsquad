"""LLM 호출 추상화.

Phase 1 은 파이프라인 뼈대 검증이 목적이라 FakeLLM 으로 끝까지 돈다.
실제 모델(langchain-anthropic 등)은 AR-9 에서 같은 인터페이스로 붙인다.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Protocol


@dataclass
class LLMResult:
    text: str
    model: str
    input_tokens: int = 0
    output_tokens: int = 0


class LLM(Protocol):
    model: str

    def complete(self, system: str, user: str) -> LLMResult: ...


@dataclass
class FakeLLM:
    """결정적 응답. 테스트와 로컬 개발용 (DEVSQUAD_FAKE_LLM=true)."""

    model: str = "fake/echo"
    calls: list[tuple[str, str]] = field(default_factory=list)

    def complete(self, system: str, user: str) -> LLMResult:
        self.calls.append((system, user))
        if "[[MODE:plan]]" in user:
            text = self._plan(user)
        else:
            text = self._execute(user)
        return LLMResult(text=text, model=self.model, input_tokens=len(system) // 4 + len(user) // 4, output_tokens=len(text) // 4)

    @staticmethod
    def _slug(user: str) -> str:
        m = re.search(r"\[\[COMMAND\]\](.*?)\[\[/COMMAND\]\]", user, re.DOTALL)
        cmd = (m.group(1) if m else "task").strip()
        words = re.findall(r"[A-Za-z0-9가-힣]+", cmd)[:4]
        return "-".join(w.lower() for w in words) or "task"

    def _plan(self, user: str) -> str:
        slug = self._slug(user)
        fb = ""
        m = re.search(r"\[\[FEEDBACK\]\](.*?)\[\[/FEEDBACK\]\]", user, re.DOTALL)
        if m and m.group(1).strip():
            fb = f"\n6. 반려 반영: {m.group(1).strip()}"
        return (
            f"## 계획 — {slug}\n"
            f"1. 요청 해석: (fake) 요청을 요구사항 정의서로 구조화한다\n"
            f"2. 작성할 문서: docs/spec/{slug}.md\n"
            f"3. 포함할 유스케이스 (예상): UC-001, UC-002\n"
            f"4. 확인이 필요한 질문: 없음\n"
            f"5. 범위 밖으로 둘 것: 없음{fb}\n"
        )

    def _execute(self, user: str) -> str:
        slug = self._slug(user)
        fb = ""
        m = re.search(r"\[\[FEEDBACK\]\](.*?)\[\[/FEEDBACK\]\]", user, re.DOTALL)
        if m and m.group(1).strip():
            fb = f"\n## 반려 반영 내역\n- {m.group(1).strip()}\n"
        return (
            f"# {slug} 요구사항 정의서 (fake)\n{fb}\n"
            "## 1. 배경과 목표\n(fake)\n\n## 4. 유스케이스\n### UC-001: (fake)\n- 사전조건:\n- 주 흐름:\n  1. …\n- 대안·예외 흐름:\n  - E1 …\n- 사후조건:\n\n"
            "```yaml\n# devsquad:summary\nuse_cases: [UC-001]\nscreens: []\nentities: []\n```\n"
        )


def make_llm(model: str | None, fake: bool) -> LLM:
    if fake or not model:
        return FakeLLM()
    # AR-9: langchain-anthropic / openai 어댑터
    raise NotImplementedError("실제 LLM 어댑터는 AR-9 에서 구현 (DEVSQUAD_FAKE_LLM=true 로 실행하세요)")
