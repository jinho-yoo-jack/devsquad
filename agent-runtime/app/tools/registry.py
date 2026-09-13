"""tool_profile → 도구 집합 바인딩 — 17-Agent 정의 가이드 §1.3.

| profile      | read | write(write_paths) | run_tests(test_command) | git | github |
| docs-writer  |  ✓   |        ✓           |            -            |  -  |   -    |
| code-writer  |  ✓   |        ✓           |            ✓            | Phase 2 | - |
| reader       |  ✓   |   docs/** 만        |            -            | diff(Phase 3) | - |
| publisher    |  ✓   |        -           |            -            |  -  | create_pr(Phase 3) |

도구는 LLM 에 노출되는 JSON 스키마(`spec`)와 실제 callable(`fn`) 의 쌍이다. 어떤 profile 도
이 파일에 없는 도구를 얻을 수 없다 — 사용자가 conventions 에 무엇을 적어도 여기서 막힌다.
"""
from __future__ import annotations

import shlex
import subprocess
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from app.tools.sandbox import Sandbox, SandboxViolation

ToolFn = Callable[..., Any]


@dataclass(frozen=True)
class Tool:
    name: str
    description: str
    parameters: dict[str, Any]  # JSON Schema (object)
    fn: ToolFn

    def spec(self) -> dict[str, Any]:
        return {"name": self.name, "description": self.description, "input_schema": self.parameters}


def _read_tools(sb: Sandbox) -> list[Tool]:
    return [
        Tool("read_file", "저장소 파일을 읽는다. 경로는 저장소 루트 기준 상대 경로.",
             {"type": "object", "properties": {"path": {"type": "string"}}, "required": ["path"]},
             lambda path: sb.read_file(path)),
        Tool("list_dir", "디렉토리 목록. 기본은 저장소 루트.",
             {"type": "object", "properties": {"path": {"type": "string", "default": "."}}},
             lambda path=".": "\n".join(sb.list_dir(path))),
        Tool("search", "정규식으로 파일 내용을 검색한다 (path:line: text).",
             {"type": "object", "properties": {"pattern": {"type": "string"}, "glob": {"type": "string", "default": "**/*"}},
              "required": ["pattern"]},
             lambda pattern, glob="**/*": "\n".join(sb.search(pattern, glob)) or "(no matches)"),
    ]


def _write_tool(sb: Sandbox) -> Tool:
    return Tool("write_file", f"파일을 쓴다(덮어쓰기). 허용 경로: {sb.write_paths}",
                {"type": "object", "properties": {"path": {"type": "string"}, "content": {"type": "string"}},
                 "required": ["path", "content"]},
                lambda path, content: sb.write_file(path, content))


def _test_tool(root: Path, test_command: str, timeout_s: int = 600) -> Tool:
    def run_tests() -> str:
        try:
            proc = subprocess.run(
                shlex.split(test_command) if "&&" not in test_command else ["bash", "-lc", test_command],
                cwd=root, capture_output=True, text=True, timeout=timeout_s, check=False,
            )
        except subprocess.TimeoutExpired:
            return f"[timeout after {timeout_s}s]"
        out = (proc.stdout + "\n" + proc.stderr).strip()
        if len(out) > 20_000:
            out = out[:10_000] + "\n[... truncated ...]\n" + out[-10_000:]
        return f"exit={proc.returncode}\n{out}"

    return Tool("run_tests", f"테스트 실행: `{test_command}`", {"type": "object", "properties": {}}, run_tests)


def tools_for(profile: str, sandbox: Sandbox, test_command: str | None = None) -> list[Tool]:
    if profile == "docs-writer":
        return [*_read_tools(sandbox), _write_tool(sandbox)]
    if profile == "code-writer":
        tools = [*_read_tools(sandbox), _write_tool(sandbox)]
        if test_command:
            tools.append(_test_tool(sandbox.root, test_command))
        return tools
    if profile == "reader":
        # docs/** 이외 write_paths 는 loader 가 이미 거절했다
        return [*_read_tools(sandbox), *([_write_tool(sandbox)] if sandbox.write_paths else [])]
    if profile == "publisher":
        return _read_tools(sandbox)
    raise ValueError(f"unknown tool_profile: {profile}")


def call_tool(tools: list[Tool], name: str, args: dict[str, Any]) -> tuple[bool, str]:
    """(ok, observation). 화이트리스트 밖 도구·샌드박스 위반은 예외가 아니라 Observation 으로 돌려준다 —
    LLM 이 다음 스텝에서 경로를 고칠 수 있게."""
    tool = next((t for t in tools if t.name == name), None)
    if tool is None:
        return False, f"[denied] 이 팀원에게 허용되지 않은 도구: {name}"
    try:
        return True, str(tool.fn(**args))
    except SandboxViolation as e:
        return False, f"[denied] {e}"
    except FileNotFoundError as e:
        return False, f"[not found] {e}"
    except TypeError as e:
        return False, f"[bad args] {e}"
