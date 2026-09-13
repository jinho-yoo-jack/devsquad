from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_health_ok():
    r = client.get("/health")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok"
    assert body["service"] == "agent-runtime"


def test_runs_requires_internal_token():
    r = client.post("/runs", json={"task_id": "t1", "command": "x", "project": {}})
    assert r.status_code == 401
