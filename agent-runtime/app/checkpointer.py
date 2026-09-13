"""체크포인터 팩토리 — memory(테스트/로컬) 또는 postgres(운영, `langgraph` 스키마).

그래프는 워커 스레드에서 **동기** `graph.stream()` 으로 실행한다(stream_bridge 참고). 따라서 동기
PostgresSaver 를 쓴다. (AsyncPostgresSaver 는 동기 메서드를 제공하지 않는다.)
"""
from __future__ import annotations

from collections.abc import Iterator
from contextlib import contextmanager
from typing import Any


@contextmanager
def open_checkpointer(db_url: str | None) -> Iterator[Any]:
    if not db_url or db_url.startswith("memory"):
        from langgraph.checkpoint.memory import InMemorySaver

        yield InMemorySaver()
        return

    from langgraph.checkpoint.postgres import PostgresSaver

    # 체크포인트 테이블은 langgraph 스키마에 (infra/init.sql 이 스키마를 만든다)
    url = db_url if "options=" in db_url else db_url + ("&" if "?" in db_url else "?") + "options=-csearch_path%3Dlanggraph"
    with PostgresSaver.from_conn_string(url) as saver:
        saver.setup()
        yield saver
