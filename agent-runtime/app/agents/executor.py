"""execute 단계의 ReAct(tool-calling) 루프 — 14-Backend 설계 B.5 `execute`.

LLM 이 도구를 부르면 화이트리스트 안에서 실행하고 Observation 을 돌려준다. 도구 호출이 없는 턴이
나오면 종료. max_iterations 를 넘으면 지금까지의 결과로 마무리한다. 모든 스텝은 이벤트로 흘린다.
"""
from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from app.agents.llm import LLM, Message, dumps_args
from app.tools.registry import Tool, call_tool


@dataclass
class ExecuteResult:
    final_text: str
    written_paths: list[str] = field(default_factory=list)
    iterations: int = 0
    hit_limit: bool = False
    usage: list[dict[str, Any]] = field(default_factory=list)
    transcript: list[Message] = field(default_factory=list)


def run_react(
    llm: LLM,
    system: str,
    user: str,
    tools: list[Tool],
    max_iterations: int,
    emit: Callable[[dict], None],
    stage_key: str,
    agent: str,
) -> ExecuteResult:
    messages: list[Message] = [Message(role="user", content=user)]
    specs = [t.spec() for t in tools]
    result = ExecuteResult(final_text="")

    for i in range(1, max_iterations + 1):
        result.iterations = i
        turn = llm.chat(system, messages, specs)
        result.usage.append({"model": turn.model, "input_tokens": turn.input_tokens, "output_tokens": turn.output_tokens})
        emit({"type": "usage", "stage_key": stage_key, "agent": agent,
              "payload": {"model": turn.model, "input_tokens": turn.input_tokens, "output_tokens": turn.output_tokens}})
        if turn.text.strip():
            emit({"type": "agent.thinking", "stage_key": stage_key, "agent": agent, "payload": {"summary": turn.text.strip()[:200]}})

        messages.append(Message(role="assistant", content=turn.text, tool_calls=turn.tool_calls))
        if not turn.wants_tools:
            result.final_text = turn.text
            break

        for tc in turn.tool_calls:
            emit({"type": "agent.tool_call", "stage_key": stage_key, "agent": agent,
                  "payload": {"call_id": tc.id, "tool": tc.name, "args_summary": dumps_args(tc.args)}})
            ok, obs = call_tool(tools, tc.name, tc.args)
            if ok and tc.name == "write_file" and isinstance(tc.args.get("path"), str):
                result.written_paths.append(tc.args["path"])
            emit({"type": "agent.tool_result", "stage_key": stage_key, "agent": agent,
                  "payload": {"call_id": tc.id, "tool": tc.name, "ok": ok, "summary": obs[:200]}})
            messages.append(Message(role="tool", content=obs, tool_call_id=tc.id))
    else:
        result.hit_limit = True
        result.final_text = f"[max_iterations={max_iterations} 도달 — 현재까지의 결과물로 제출]"
        emit({"type": "agent.message", "stage_key": stage_key, "agent": agent, "payload": {"text": result.final_text}})

    result.transcript = messages
    return result
