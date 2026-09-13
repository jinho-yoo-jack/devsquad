import pytest

from app.tools.registry import call_tool, tools_for
from app.tools.sandbox import Sandbox, SandboxViolation, _glob_match


@pytest.fixture
def ws(tmp_path):
    (tmp_path / "docs" / "spec").mkdir(parents=True)
    (tmp_path / "server" / "src").mkdir(parents=True)
    (tmp_path / "docs" / "spec" / "a.md").write_text("# A\nhello world\n", encoding="utf-8")
    (tmp_path / ".env").write_text("SECRET=1", encoding="utf-8")
    (tmp_path / "secrets").mkdir()
    (tmp_path / "secrets" / "k.txt").write_text("x", encoding="utf-8")
    (tmp_path / "server" / "src" / "App.java").write_text("class App {}", encoding="utf-8")
    return tmp_path


def test_glob_match_semantics():
    assert _glob_match("docs/spec/a.md", "docs/spec/**")
    assert _glob_match("docs/spec/deep/a.md", "docs/spec/**")
    assert not _glob_match("docs/design/a.md", "docs/spec/**")
    assert _glob_match("web/app/page.tsx", "web/**")
    assert not _glob_match("server/x.java", "web/**")
    assert _glob_match("docs/api-usage/x.md", "docs/api-usage/**")
    assert _glob_match("README.md", "*.md") and not _glob_match("docs/README.md", "*.md")


def test_read_within_root_and_secret_blocks(ws):
    sb = Sandbox(ws, write_paths=["docs/spec/**"])
    assert "hello" in sb.read_file("docs/spec/a.md")
    for bad in [".env", "secrets/k.txt", "../outside.txt", "/etc/passwd", "docs/../../x"]:
        with pytest.raises(SandboxViolation):
            sb.read_file(bad)
    assert ".env" not in sb.list_dir(".") and "secrets/" not in sb.list_dir(".")


def test_write_only_inside_write_paths(ws):
    sb = Sandbox(ws, write_paths=["docs/spec/**"])
    assert "wrote" in sb.write_file("docs/spec/new.md", "x")
    assert (ws / "docs/spec/new.md").exists()
    with pytest.raises(SandboxViolation):
        sb.write_file("server/src/Evil.java", "x")
    with pytest.raises(SandboxViolation):
        sb.write_file("docs/spec/.env", "x")  # write_paths 안이어도 시크릿 패턴은 거부


def test_search_skips_secrets(ws):
    sb = Sandbox(ws, write_paths=[])
    hits = sb.search("SECRET|hello")
    assert any("docs/spec/a.md:2" in h for h in hits)
    assert not any(".env" in h for h in hits)


def test_tool_registry_by_profile(ws):
    docs = tools_for("docs-writer", Sandbox(ws, write_paths=["docs/spec/**"]))
    assert {t.name for t in docs} == {"read_file", "list_dir", "search", "write_file"}
    code = tools_for("code-writer", Sandbox(ws, write_paths=["server/**"]), test_command="echo ok")
    assert "run_tests" in {t.name for t in code}
    reader = tools_for("reader", Sandbox(ws, write_paths=[]))
    assert "write_file" not in {t.name for t in reader}
    pub = tools_for("publisher", Sandbox(ws))
    assert {t.name for t in pub} == {"read_file", "list_dir", "search"}


def test_call_tool_returns_observations_not_exceptions(ws):
    tools = tools_for("docs-writer", Sandbox(ws, write_paths=["docs/spec/**"]))
    ok, obs = call_tool(tools, "write_file", {"path": "server/x.java", "content": "x"})
    assert not ok and obs.startswith("[denied]")
    ok, obs = call_tool(tools, "run_tests", {})
    assert not ok and "허용되지 않은 도구" in obs
    ok, obs = call_tool(tools, "read_file", {"path": "docs/spec/none.md"})
    assert not ok and obs.startswith("[not found]")
    ok, obs = call_tool(tools, "read_file", {"path": "docs/spec/a.md"})
    assert ok and "hello" in obs


def test_run_tests_tool_executes_command(ws):
    tools = tools_for("code-writer", Sandbox(ws, write_paths=["server/**"]), test_command="echo tests-passed")
    ok, obs = call_tool(tools, "run_tests", {})
    assert ok and "exit=0" in obs and "tests-passed" in obs
