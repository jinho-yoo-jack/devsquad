# agent-runtime

```bash
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
uvicorn app.main:app --port 8100 --reload
pytest
```

구조는 `docs/14-Backend-설계.md` Part B 를 따른다. 골격 단계에서는 `app/main.py`, `app/settings.py` 만 실체가 있고 나머지 패키지는 자리만 있다.
