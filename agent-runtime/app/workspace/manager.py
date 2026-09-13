"""Task 별 작업 디렉토리 — 14-Backend 설계 B.2 workspace.

Phase 1: `project.local_path` 가 주어지면 그 디렉토리를 `<root>/<task_id>/` 로 복사한다(원본 불변).
Phase 2: GitHub installation token 으로 `git clone` (worktree) — 같은 인터페이스로 교체.

`.devsquad/` 컨텍스트 디렉토리는 workspace 안의 것을 쓴다 (사용자 저장소 안에 산다는 원칙).
"""
from __future__ import annotations

import shutil
from dataclasses import dataclass
from pathlib import Path

IGNORE = shutil.ignore_patterns(".git", "node_modules", ".venv", "__pycache__", ".next", "build", "target", ".gradle")


@dataclass
class Workspace:
    task_id: str
    path: Path
    context_dir: Path  # <path>/.devsquad


class WorkspaceManager:
    def __init__(self, root: Path):
        self.root = root
        self.root.mkdir(parents=True, exist_ok=True)

    def path_for(self, task_id: str) -> Path:
        return self.root / task_id

    def prepare(self, task_id: str, local_path: str | None, context_path: str = ".devsquad") -> Workspace:
        dest = self.path_for(task_id)
        if not dest.exists():
            if local_path:
                src = Path(local_path).expanduser().resolve()
                if not src.is_dir():
                    raise FileNotFoundError(f"project.local_path 가 디렉토리가 아닙니다: {src}")
                if dest.resolve() == src or src in dest.resolve().parents:
                    raise ValueError(f"workspace root({self.root}) 가 project.local_path({src}) 안에 있으면 안 됩니다")
                shutil.copytree(src, dest, ignore=IGNORE, dirs_exist_ok=True)
            else:
                dest.mkdir(parents=True)  # Phase 2: git clone 자리
        ctx = dest / context_path
        if not (ctx / "pipeline.yaml").exists():
            raise FileNotFoundError(f"{context_path}/pipeline.yaml 이 없습니다 (workspace={dest})")
        return Workspace(task_id=task_id, path=dest, context_dir=ctx)

    def exists(self, task_id: str) -> bool:
        return self.path_for(task_id).is_dir()

    def cleanup(self, task_id: str) -> None:
        shutil.rmtree(self.path_for(task_id), ignore_errors=True)
