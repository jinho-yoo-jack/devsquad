"""LLM 호출 추상화 — tool-calling 을 포함한 단일 인터페이스.

- `LLM.chat(system, messages, tools)` 는 한 턴을 돌리고 텍스트 + tool_calls + 사용량을 돌려준다.
- 실제 모델은 LangChain `init_chat_model` 로 붙인다 (anthropic/openai/ollama … `provider/model` 문자열).
- FakeLLM 은 결정적으로 동작해 테스트와 로컬 개발(DEVSQUAD_FAKE_LLM=true)에서 파이프라인을 끝까지 돌린다.
"""
from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from typing import Any, Literal, Protocol

Role = Literal["user", "assistant", "tool"]


@dataclass
class Message:
    role: Role
    content: str
    tool_call_id: str | None = None  # role == "tool"
    tool_calls: list[ToolCall] = field(default_factory=list)  # role == "assistant"


@dataclass
class ToolCall:
    id: str
    name: str
    args: dict[str, Any]


@dataclass
class LLMTurn:
    text: str
    tool_calls: list[ToolCall]
    model: str
    input_tokens: int = 0
    output_tokens: int = 0

    @property
    def wants_tools(self) -> bool:
        return bool(self.tool_calls)


class LLM(Protocol):
    model: str

    def chat(self, system: str, messages: list[Message], tools: list[dict[str, Any]] | None = None) -> LLMTurn: ...


# ---- Fake -----------------------------------------------------------------


@dataclass
class FakeLLM:
    """결정적 응답.

    plan 모드: Plan 텍스트 1턴.
    execute 모드: (1) write_file tool call 로 결과물 작성 → (2) 완료 텍스트. 반려 피드백이 있으면 반영.
    `.calls` 에 (system, last_user_text) 를 기록해 테스트가 프롬프트를 검사할 수 있다.
    """

    model: str = "fake/echo"
    calls: list[tuple[str, str]] = field(default_factory=list)

    def chat(self, system: str, messages: list[Message], tools: list[dict[str, Any]] | None = None) -> LLMTurn:
        user_text = next((m.content for m in messages if m.role == "user"), "")
        self.calls.append((system, user_text))
        n_in = len(system) // 4 + sum(len(m.content) for m in messages) // 4

        if "[[MODE:plan]]" in user_text or not tools:
            text = self._plan(user_text) if "[[MODE:plan]]" in user_text else self._execute_text(user_text)
            return LLMTurn(text=text, tool_calls=[], model=self.model, input_tokens=n_in, output_tokens=len(text) // 4)

        # execute 모드: 아직 write_file 을 하지 않았으면 도구 호출, 했으면 마무리
        already_wrote = any(m.role == "tool" for m in messages)
        if not already_wrote and any(t["name"] == "write_file" for t in tools):
            path = self._target_path(user_text)
            content = self._execute_text(user_text)
            return LLMTurn(
                text="결과물을 파일로 작성합니다.",
                tool_calls=[ToolCall(id="call_1", name="write_file", args={"path": path, "content": content})],
                model=self.model, input_tokens=n_in, output_tokens=len(content) // 4,
            )
        last_tool = next((m for m in reversed(messages) if m.role == "tool"), None)
        if last_tool and last_tool.content.startswith("[denied]"):
            # 거부되면 허용 경로로 한 번 재시도 (샌드박스 Observation 을 읽고 고치는 흉내)
            m = re.search(r"write_paths=\['([^'*]+)", last_tool.content)
            base = (m.group(1) if m else "docs/").rstrip("/")
            return LLMTurn(text="허용 경로로 다시 씁니다.", model=self.model, input_tokens=n_in, output_tokens=10,
                           tool_calls=[ToolCall(id="call_2", name="write_file",
                                                args={"path": f"{base}/fallback.md", "content": self._execute_text(user_text)})])
        text = "완료 기준 자가 점검 완료. 결과물을 제출합니다."
        return LLMTurn(text=text, tool_calls=[], model=self.model, input_tokens=n_in, output_tokens=len(text) // 4)

    # -- helpers --
    @staticmethod
    def _slug(user: str) -> str:
        m = re.search(r"\[\[COMMAND\]\](.*?)\[\[/COMMAND\]\]", user, re.DOTALL)
        cmd = (m.group(1) if m else "task").strip()
        words = re.findall(r"[A-Za-z0-9가-힣]+", cmd)[:4]
        return "-".join(w.lower() for w in words) or "task"

    @staticmethod
    def _feedback(user: str) -> str:
        m = re.search(r"\[\[FEEDBACK\]\](.*?)\[\[/FEEDBACK\]\]", user, re.DOTALL)
        return m.group(1).strip() if m and m.group(1).strip() else ""

    def _target_path(self, user: str) -> str:
        m = re.search(r"(docs/[\w\-/]+\.md)", user)  # approved_plan 안의 경로 우선
        return m.group(1) if m else f"docs/spec/{self._slug(user)}.md"

    def _plan(self, user: str) -> str:
        slug, fb = self._slug(user), self._feedback(user)
        tail = f"\n6. 반려 반영: {fb}" if fb else ""
        return (
            f"## 계획 — {slug}\n"
            f"1. 요청 해석: (fake) 요청을 요구사항 정의서로 구조화한다\n"
            f"2. 작성할 문서: docs/spec/{slug}.md\n"
            f"3. 포함할 유스케이스 (예상): UC-001, UC-002\n"
            f"4. 확인이 필요한 질문: 없음\n"
            f"5. 범위 밖으로 둘 것: 없음{tail}\n"
        )

    def _execute_text(self, user: str) -> str:
        slug, fb = self._slug(user), self._feedback(user)
        fbs = f"\n## 반려 반영 내역\n- {fb}\n" if fb else ""
        return (
            f"# {slug} 요구사항 정의서 (fake)\n{fbs}\n"
            "## 1. 배경과 목표\n(fake)\n\n## 4. 유스케이스\n### UC-001: (fake)\n- 사전조건:\n- 주 흐름:\n  1. …\n- 대안·예외 흐름:\n  - E1 …\n- 사후조건:\n\n"
            "```yaml\n# devsquad:summary\nuse_cases: [UC-001]\nscreens: []\nentities: []\n```\n"
        )


# ---- LangChain 어댑터 -------------------------------------------------------


class LangChainLLM:
    """`provider/model` (예: anthropic/claude-sonnet-4-5, openai/gpt-4o) → LangChain chat model.

    API 키는 환경 변수(ANTHROPIC_API_KEY, OPENAI_API_KEY)에서 LangChain 이 읽는다. 코드에 두지 않는다.
    """

    def __init__(self, model: str, temperature: float = 0.2):
        from langchain.chat_models import init_chat_model  # 지연 import: 테스트 환경에 없어도 됨

        provider, _, name = model.partition("/")
        if not name:
            raise ValueError(f"model 은 'provider/model' 형식이어야 합니다: {model!r}")
        self.model = model
        self._chat = init_chat_model(name, model_provider=provider, temperature=temperature)

    def chat(self, system: str, messages: list[Message], tools: list[dict[str, Any]] | None = None) -> LLMTurn:
        from langchain_core.messages import AIMessage, HumanMessage, SystemMessage, ToolMessage

        lc_msgs: list[Any] = [SystemMessage(content=system)]
        for m in messages:
            if m.role == "user":
                lc_msgs.append(HumanMessage(content=m.content))
            elif m.role == "assistant":
                lc_msgs.append(AIMessage(content=m.content, tool_calls=[
                    {"id": tc.id, "name": tc.name, "args": tc.args} for tc in m.tool_calls]))
            else:
                lc_msgs.append(ToolMessage(content=m.content, tool_call_id=m.tool_call_id or ""))

        model = self._chat.bind_tools(_to_lc_tools(tools)) if tools else self._chat
        ai = model.invoke(lc_msgs)
        usage = getattr(ai, "usage_metadata", None) or {}
        text = ai.content if isinstance(ai.content, str) else "".join(
            (c.get("text", "") if isinstance(c, dict) else str(c)) for c in ai.content)
        calls = [ToolCall(id=str(tc.get("id") or f"call_{i}"), name=tc["name"], args=dict(tc.get("args") or {}))
                 for i, tc in enumerate(getattr(ai, "tool_calls", []) or [])]
        return LLMTurn(text=text, tool_calls=calls, model=self.model,
                       input_tokens=int(usage.get("input_tokens", 0)), output_tokens=int(usage.get("output_tokens", 0)))


def _to_lc_tools(tools: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """우리 스키마 {name, description, input_schema} → OpenAI-style function 스펙 (LangChain 이 공급자별로 변환)."""
    return [{"type": "function", "function": {"name": t["name"], "description": t["description"], "parameters": t["input_schema"]}}
            for t in tools]


def make_llm(model: str | None, fake: bool) -> LLM:
    if fake or not model:
        return FakeLLM()
    return LangChainLLM(model)


def dumps_args(args: dict[str, Any], limit: int = 200) -> str:
    s = json.dumps(args, ensure_ascii=False)
    return s if len(s) <= limit else s[: limit - 1] + "…"
