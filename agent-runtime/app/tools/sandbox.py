"""파일 도구 샌드박스 — 14-Backend 설계 B.7, 17-Agent 정의 가이드 §1.3.

모든 경로는 workspace 아래로 정규화되고 상위 탈출을 차단한다. 시크릿 패턴은 읽기·쓰기 모두 거부.
쓰기는 agent 의 write_paths glob 안에서만 허용된다 — 팀원 이름이 아니라 tool_profile 과 write_paths 가
보안 경계다.
"""
from __future__ import annotations

import fnmatch
import re
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath

SECRET_PATTERNS: tuple[str, ...] = (".env", ".env.*", "*.pem", "*.key", "*.p12", "*.jks", "id_rsa*", "id_ed25519*")
SECRET_DIRS: tuple[str, ...] = ("secrets", ".git", ".ssh", ".aws", "node_modules", ".venv")
MAX_READ_BYTES = 200_000
MAX_WRITE_BYTES = 2_000_000


class SandboxViolation(PermissionError):
    """도구 계층에서 거부된 접근. LLM 에게는 Observation 으로 돌려주고, 이벤트로 남긴다."""


@dataclass
class Sandbox:
    root: Path
    write_paths: list[str] = field(default_factory=list)
    read_only: bool = False

    def __post_init__(self) -> None:
        self.root = self.root.resolve()

    # ---- 경로 --------------------------------------------------------------

    def resolve(self, rel: str) -> Path:
        if not rel or rel.startswith(("/", "~")) or "\\" in rel:
            raise SandboxViolation(f"저장소 상대 경로만 허용: {rel!r}")
        p = PurePosixPath(rel)
        if any(part == ".." for part in p.parts):
            raise SandboxViolation(f"상위 디렉토리 탈출 금지: {rel!r}")
        target = (self.root / p).resolve()
        if target != self.root and self.root not in target.parents:
            raise SandboxViolation(f"workspace 밖 경로: {rel!r}")
        self._check_secret(p)
        return target

    def _check_secret(self, p: PurePosixPath) -> None:
        for part in p.parts[:-1]:
            if part in SECRET_DIRS:
                raise SandboxViolation(f"보호된 디렉토리: {p}")
        if p.parts and p.parts[-1] in SECRET_DIRS:
            raise SandboxViolation(f"보호된 디렉토리: {p}")
        name = p.name
        if any(fnmatch.fnmatch(name, pat) for pat in SECRET_PATTERNS):
            raise SandboxViolation(f"시크릿 파일 접근 금지: {p}")

    def can_write(self, rel: str) -> bool:
        if self.read_only:
            return False
        return any(_glob_match(rel, wp) for wp in self.write_paths)

    def assert_writable(self, rel: str) -> None:
        if not self.can_write(rel):
            raise SandboxViolation(f"쓰기 권한 없음: {rel!r} (write_paths={self.write_paths})")

    # ---- 도구 구현 ----------------------------------------------------------

    def read_file(self, path: str, max_bytes: int = MAX_READ_BYTES) -> str:
        target = self.resolve(path)
        if not target.is_file():
            raise FileNotFoundError(f"파일 없음: {path}")
        data = target.read_bytes()
        truncated = len(data) > max_bytes
        text = data[:max_bytes].decode("utf-8", errors="replace")
        return text + (f"\n\n[... {len(data) - max_bytes} bytes truncated]" if truncated else "")

    def write_file(self, path: str, content: str) -> str:
        self.assert_writable(path)
        target = self.resolve(path)
        if len(content.encode("utf-8")) > MAX_WRITE_BYTES:
            raise SandboxViolation(f"파일이 너무 큽니다 (> {MAX_WRITE_BYTES} bytes): {path}")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")
        return f"wrote {len(content)} chars → {path}"

    def list_dir(self, path: str = ".", max_entries: int = 500) -> list[str]:
        target = self.resolve(path) if path not in (".", "") else self.root
        if not target.is_dir():
            raise FileNotFoundError(f"디렉토리 없음: {path}")
        out: list[str] = []
        for p in sorted(target.iterdir()):
            rel = p.relative_to(self.root).as_posix()
            if p.name in SECRET_DIRS or any(fnmatch.fnmatch(p.name, pat) for pat in SECRET_PATTERNS):
                continue
            out.append(rel + ("/" if p.is_dir() else ""))
            if len(out) >= max_entries:
                out.append(f"[... more than {max_entries} entries]")
                break
        return out

    def search(self, pattern: str, glob: str = "**/*", max_hits: int = 100) -> list[str]:
        """ripgrep 대용 — 정규식으로 파일 내용을 검색. 결과는 'path:line: text'."""
        rx = re.compile(pattern)
        hits: list[str] = []
        for f in sorted(self.root.glob(glob)):
            if not f.is_file():
                continue
            rel = f.relative_to(self.root)
            try:
                self._check_secret(PurePosixPath(rel.as_posix()))
            except SandboxViolation:
                continue
            try:
                for i, line in enumerate(f.read_text(encoding="utf-8", errors="ignore").splitlines(), 1):
                    if rx.search(line):
                        hits.append(f"{rel.as_posix()}:{i}: {line.strip()[:200]}")
                        if len(hits) >= max_hits:
                            return hits
            except OSError:
                continue
        return hits


def _glob_match(rel: str, pattern: str) -> bool:
    """`docs/spec/**` 같은 패턴. `**` 는 0개 이상의 디렉토리, `*` 는 한 세그먼트 안에서만."""
    rel_parts = PurePosixPath(rel).parts
    pat_parts = PurePosixPath(pattern).parts
    return _match_parts(rel_parts, pat_parts)


def _match_parts(rel: tuple[str, ...], pat: tuple[str, ...]) -> bool:
    if not pat:
        return not rel
    head, rest = pat[0], pat[1:]
    if head == "**":
        # ** 가 마지막이면 나머지 전부 매치 (파일 하나 이상)
        if not rest:
            return len(rel) >= 1
        for i in range(len(rel) + 1):
            if _match_parts(rel[i:], rest):
                return True
        return False
    if not rel:
        return False
    return fnmatch.fnmatchcase(rel[0], head) and _match_parts(rel[1:], rest)
