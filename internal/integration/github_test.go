package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/scm"
)

type pullRequest struct {
	Number                       int
	URL, Head, Base, Title, Body string
}
type githubAPI struct {
	mu    sync.Mutex
	pulls map[string]*pullRequest
	posts int
	fail  int
}

func (g *githubAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var body map[string]string
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	reply := func(status int, v any) { w.WriteHeader(status); _ = json.NewEncoder(w).Encode(v) }
	switch {
	case r.Header.Get("Authorization") != "Bearer tok":
		reply(401, map[string]string{"message": "Bad credentials"})
	case r.Method == "GET":
		list := []map[string]any{}
		if p := g.pulls[strings.TrimPrefix(r.URL.Query().Get("head"), "o:")]; p != nil {
			list = append(list, map[string]any{"number": p.Number, "html_url": p.URL})
		}
		reply(200, list)
	case r.Method == "POST" && g.fail > 0:
		g.fail--
		reply(500, map[string]string{"message": "Server Error"})
	case r.Method == "POST":
		g.posts++
		p := &pullRequest{Number: len(g.pulls) + 1, Head: body["head"], Base: body["base"], Title: body["title"], Body: body["body"]}
		p.URL = fmt.Sprintf("https://github.example/o/r/pull/%d", p.Number)
		g.pulls[p.Head] = p
		reply(201, map[string]any{"number": p.Number, "html_url": p.URL})
	case r.Method == "PATCH":
		for _, p := range g.pulls {
			if strings.HasSuffix(r.URL.Path, fmt.Sprintf("/%d", p.Number)) {
				p.Title, p.Body = body["title"], body["body"]
			}
		}
		reply(200, map[string]any{})
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	c.Dir = dir
	out, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, out)
	}
	return strings.TrimSpace(string(out))
}

// githubHarness publishes the harness source as github.example/o/r (a local bare repository).
func githubHarness(t *testing.T, pipeline string) (*harness, *githubAPI, string) {
	t.Helper()
	h := newHarness(t, pipeline, nil)
	src := t.TempDir()
	if e := os.CopyFS(src, os.DirFS(h.source)); e != nil {
		t.Fatal(e)
	}
	run(t, src, "init", "--quiet", "--initial-branch=main")
	run(t, src, "add", "--all")
	run(t, src, "commit", "--quiet", "-m", "initial")
	root := t.TempDir()
	bare := filepath.Join(root, "o", "r.git")
	run(t, src, "clone", "--quiet", "--bare", ".", bare)
	api := &githubAPI{pulls: map[string]*pullRequest{}}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	h.scm = &scm.GitHub{Token: "tok", APIURL: server.URL, GitURL: "file://" + root, WebURL: "https://github.example"}
	h.tasks.SCM = h.scm
	h.engine.SCM = h.scm
	return h, api, bare
}
func (h *harness) remoteTask() string {
	h.t.Helper()
	var p app.ProjectEntity
	h.request("POST", "/api/v1/projects", map[string]any{"name": "remote", "github_owner": "o", "github_repo": "r", "default_branch": "main"}, 201, &p)
	var task app.TaskResponse
	h.request("POST", "/api/v1/tasks", map[string]any{"project_id": p.ID, "command": "Build a feature\nwith details"}, 201, &task)
	return task.ID
}
func (h *harness) deliverable(id, key string) string {
	h.t.Helper()
	var content string
	if e := h.db.Pool.QueryRow(context.Background(), "select d.content from deliverable d join stage s on s.id=d.stage_id where d.task_id=$1 and s.stage_key=$2 order by d.created_at desc limit 1", id, key).Scan(&content); e != nil {
		h.t.Fatal(e)
	}
	return content
}

const twoWriters = "version: 1\nstages:\n- {id: a, agent: a, approvals: []}\n- {id: b, agent: b, approvals: []}\n- {id: pr, agent: publisher, depends_on: [a, b], approvals: []}\n"

// 16-구현-로드맵 Phase 3: approved work becomes devsquad/<task-id>/<role> branches and pull requests.
func TestPublisherOpensPullRequestPerRole(t *testing.T) {
	h, api, bare := githubHarness(t, twoWriters+"policy: {pr: {mode: per-role}}\n")
	id := h.remoteTask()
	h.status(id, "completed")
	if len(api.pulls) != 2 || api.posts != 2 {
		t.Fatalf("pull requests: %+v", api.pulls)
	}
	base := run(t, bare, "rev-parse", "main")
	for _, role := range []string{"a", "b"} {
		branch := "devsquad/" + id + "/" + role
		p := api.pulls[branch]
		if p == nil || p.Base != "main" || p.Title != "[DevSquad] Build a feature ("+role+")" {
			t.Fatalf("%s: %+v", role, p)
		}
		for _, want := range []string{"`" + id + "`", "## 승인 이력", "### " + role + " (" + role + ")", "https://github.example/o/r/blob/" + branch + "/docs/" + role + "/"} {
			if !strings.Contains(p.Body, want) {
				t.Errorf("%s body missing %q:\n%s", role, want, p.Body)
			}
		}
		files := strings.Split(run(t, bare, "ls-tree", "-r", "--name-only", "refs/heads/"+branch), "\n")
		owned := slices.ContainsFunc(files, func(f string) bool { return strings.HasPrefix(f, "docs/"+role+"/") })
		other := slices.ContainsFunc(files, func(f string) bool { return strings.HasPrefix(f, "docs/") && !strings.HasPrefix(f, "docs/"+role+"/") })
		if !owned || other || run(t, bare, "rev-parse", "refs/heads/"+branch+"^") != base {
			t.Fatalf("%s branch files: %v", role, files)
		}
	}
	p := h.payloads(id)
	p.require(t, "pr.opened", "url", "number", "role")
	urls, _ := p["run.completed"][0]["pr_urls"].([]any)
	if len(p["pr.opened"]) != 2 || len(urls) != 2 {
		t.Fatalf("pr.opened=%v run.completed=%v", p["pr.opened"], p["run.completed"])
	}
	if content := h.deliverable(id, "pr"); !strings.Contains(content, api.pulls["devsquad/"+id+"/a"].URL) {
		t.Fatalf("publisher deliverable: %s", content)
	}
}

func TestPublisherOpensSinglePullRequest(t *testing.T) {
	h, api, bare := githubHarness(t, twoWriters+"policy: {pr: {mode: single, base_branch: main}}\n")
	id := h.remoteTask()
	h.status(id, "completed")
	p := api.pulls["devsquad/"+id]
	if len(api.pulls) != 1 || p == nil || p.Title != "[DevSquad] Build a feature" {
		t.Fatalf("pull requests: %+v", api.pulls)
	}
	files := run(t, bare, "ls-tree", "-r", "--name-only", "refs/heads/devsquad/"+id)
	if !strings.Contains(files, "docs/a/") || !strings.Contains(files, "docs/b/") {
		t.Fatalf("single branch: %s", files)
	}
}

// 14-Backend-설계: a push or PR failure is retryable, not run.failed.
func TestPublishFailureBlocksUntilResumed(t *testing.T) {
	h, api, _ := githubHarness(t, twoWriters)
	api.fail = 1
	id := h.remoteTask()
	h.status(id, "blocked")
	p := h.payloads(id)
	if len(p["run.failed"]) != 0 || len(p["stage.blocked"]) != 1 || p["stage.blocked"][0]["reason"] != "publish_failed" || !strings.Contains(fmt.Sprint(p["stage.blocked"][0]["error"]), "500") {
		t.Fatalf("blocked=%v failed=%v", p["stage.blocked"], p["run.failed"])
	}
	h.request("POST", "/api/v1/tasks/"+id+"/resume", nil, 200, nil)
	h.status(id, "completed")
	if len(api.pulls) != 1 {
		t.Fatalf("pull requests after resume: %+v", api.pulls)
	}
}

func TestLocalProjectSkipsPullRequests(t *testing.T) {
	h, api, _ := githubHarness(t, twoWriters)
	id := h.create()
	h.status(id, "completed")
	if api.posts != 0 || len(api.pulls) != 0 {
		t.Fatalf("local project opened pull requests: %+v", api.pulls)
	}
	if content := h.deliverable(id, "pr"); !strings.Contains(content, "PR을 만들지 않았습니다") || !strings.Contains(content, "## 승인 이력") {
		t.Fatalf("publisher deliverable: %s", content)
	}
	p := h.payloads(id)
	events, _ := h.db.Events(context.Background(), id, 0, 1000)
	for _, ev := range events {
		if domain.Text(ev.StageKey) == "pr" && (ev.Type == "usage" || ev.Type == "agent.thinking") {
			t.Fatalf("publisher called a model: %+v", ev)
		}
	}
	if _, ok := p["run.completed"][0]["pr_urls"]; ok {
		t.Fatalf("run.completed: %v", p["run.completed"])
	}
}

func TestRemoteProjectRequiresGitHub(t *testing.T) {
	h := newHarness(t, gated, nil)
	var p app.ProjectEntity
	h.request("POST", "/api/v1/projects", map[string]any{"name": "remote", "github_owner": "o", "github_repo": "r"}, 201, &p)
	h.fails("POST", "/api/v1/tasks", map[string]any{"project_id": p.ID, "command": "c"}, 400, "PROJECT_INVALID")
	g, _, _ := githubHarness(t, gated)
	for _, repo := range []string{"../r", "r --upload-pack=x", ".."} {
		var bad app.ProjectEntity
		g.request("POST", "/api/v1/projects", map[string]any{"name": "bad", "github_owner": "o", "github_repo": repo}, 201, &bad)
		g.fails("POST", "/api/v1/tasks", map[string]any{"project_id": bad.ID, "command": "c"}, 400, "PROJECT_INVALID")
	}
}
