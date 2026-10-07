package scm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeAPI struct {
	mu       sync.Mutex
	open     map[string]map[string]any // head -> PR
	requests []string
	failPost int
	next     int
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("Accept") != "application/vnd.github+json" {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		return
	}
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	switch {
	case r.Method == "GET" && r.URL.Path == "/repos/o/r/pulls":
		list := []map[string]any{}
		if pr := f.open[strings.TrimPrefix(r.URL.Query().Get("head"), "o:")]; pr != nil && r.URL.Query().Get("state") == "open" {
			list = append(list, pr)
		}
		_ = json.NewEncoder(w).Encode(list)
	case r.Method == "POST" && r.URL.Path == "/repos/o/r/pulls":
		head := body["head"].(string)
		if f.failPost > 0 {
			f.failPost--
			f.open[head] = map[string]any{"number": 99, "html_url": "https://github.example/o/r/pull/99"}
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":[{"message":"A pull request already exists for o:` + head + `."}]}`))
			return
		}
		f.next++
		pr := map[string]any{"number": f.next, "html_url": "https://github.example/o/r/pull/" + string(rune('0'+f.next)), "title": body["title"], "body": body["body"], "base": body["base"]}
		f.open[head] = pr
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(pr)
	case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/repos/o/r/pulls/"):
		for _, pr := range f.open {
			pr["body"] = body["body"]
			pr["title"] = body["title"]
			_ = json.NewEncoder(w).Encode(pr)
			return
		}
		w.WriteHeader(404)
	default:
		w.WriteHeader(404)
	}
}

func github(t *testing.T) (*GitHub, *fakeAPI) {
	t.Helper()
	f := &fakeAPI{open: map[string]map[string]any{}}
	s := httptest.NewServer(f)
	t.Cleanup(s.Close)
	return &GitHub{Token: "tok", APIURL: s.URL, GitURL: "https://github.example"}, f
}

// 14-Backend-설계: PR creation is idempotent; an open PR for the branch is reused.
func TestOpenCreatesThenReusesPullRequest(t *testing.T) {
	g, f := github(t)
	req := PullRequest{Owner: "o", Repo: "r", Head: "devsquad/t1/backend", Base: "main", Title: "title", Body: "v1"}
	first, e := g.Open(context.Background(), req)
	if e != nil || !first.Created || first.Number != 1 || first.URL == "" {
		t.Fatalf("create: %+v %v", first, e)
	}
	if pr := f.open["devsquad/t1/backend"]; pr["base"] != "main" || pr["body"] != "v1" {
		t.Fatalf("request body: %v", pr)
	}
	req.Body = "v2"
	again, e := g.Open(context.Background(), req)
	if e != nil || again.Created || again.Number != 1 || again.URL != first.URL {
		t.Fatalf("reuse: %+v %v", again, e)
	}
	if f.open["devsquad/t1/backend"]["body"] != "v2" {
		t.Fatal("reused PR body was not refreshed")
	}
	posts := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, "POST") {
			posts++
		}
	}
	if posts != 1 {
		t.Fatalf("duplicate PR requests: %v", f.requests)
	}
}

func TestOpenRecoversFromConcurrentCreation(t *testing.T) {
	g, f := github(t)
	f.failPost = 1
	got, e := g.Open(context.Background(), PullRequest{Owner: "o", Repo: "r", Head: "devsquad/t1/frontend", Base: "main", Title: "t", Body: "b"})
	if e != nil || got.Number != 99 || got.Created {
		t.Fatalf("%+v %v", got, e)
	}
}

func TestOpenReportsAPIErrorsWithoutToken(t *testing.T) {
	g, _ := github(t)
	g.Token = "wrong-secret-token"
	_, e := g.Open(context.Background(), PullRequest{Owner: "o", Repo: "r", Head: "h", Base: "main"})
	if e == nil || !strings.Contains(e.Error(), "401") || !strings.Contains(e.Error(), "Bad credentials") || strings.Contains(e.Error(), "wrong-secret-token") {
		t.Fatalf("error: %v", e)
	}
}

func TestGitTransport(t *testing.T) {
	g := &GitHub{Token: "tok", GitURL: "https://github.example/"}
	if got := g.RemoteURL("o", "r"); got != "https://github.example/o/r.git" {
		t.Fatal(got)
	}
	header := g.GitHeader()
	raw, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Authorization: Basic "))
	if e != nil || string(raw) != "x-access-token:tok" {
		t.Fatalf("header: %q", header)
	}
	if got := g.Web(); got != "https://github.example" {
		t.Fatalf("web defaults to the git host: %q", got)
	}
	if got := (&GitHub{GitURL: "file:///tmp/git", WebURL: "https://github.example/"}).Web(); got != "https://github.example" {
		t.Fatalf("web: %q", got)
	}
}

// 10-PRD FR-42: task summary, approved deliverable links, stage summaries and approval history.
func TestBodyCarriesTaskAndApprovalHistory(t *testing.T) {
	at := time.Date(2026, 10, 7, 1, 2, 3, 0, time.UTC)
	body := Body(Report{
		TaskID: "t1", Command: "회원 탈퇴 기능 추가\n세부 내용", Owner: "o", Repo: "r", WebURL: "https://github.example",
		Stages: []StageReport{
			{Key: "planning", Role: "planner", Branch: "devsquad/t1/planner", Ref: "docs/spec/withdraw.md", Summary: "요구사항 정의서", Approvals: []ApprovalReport{
				{Kind: "plan", RetryNo: 0, Status: "approved", DecidedBy: "admin", DecidedAt: &at},
				{Kind: "deliverable", RetryNo: 0, Status: "rejected", DecidedBy: "admin", DecidedAt: &at, Feedback: "복구 안내 | 모달 필요"},
				{Kind: "deliverable", RetryNo: 1, Status: "edited", DecidedBy: "admin", DecidedAt: &at},
			}},
			{Key: "backend", Role: "backend", Summary: "API 구현"},
		},
	})
	for _, want := range []string{"회원 탈퇴 기능 추가", "세부 내용", "`t1`", "[docs/spec/withdraw.md](https://github.example/o/r/blob/devsquad/t1/planner/docs/spec/withdraw.md)", "요구사항 정의서", "API 구현", "| planning | deliverable | v1 | rejected | admin | 2026-10-07 01:02 UTC | 복구 안내 \\| 모달 필요 |", "| planning | deliverable | v2 | edited |"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if Title("회원 탈퇴 기능 추가\n세부", "backend") != "[DevSquad] 회원 탈퇴 기능 추가 (backend)" || Title("x", "") != "[DevSquad] x" {
		t.Fatal(Title("회원 탈퇴 기능 추가\n세부", "backend"))
	}
	local := Body(Report{TaskID: "t1", Command: "c", Stages: []StageReport{{Key: "a", Role: "a", Ref: "docs/a.md"}}})
	if !strings.Contains(local, "- 산출물: `docs/a.md`") || strings.Contains(local, "](") {
		t.Fatalf("a body without a remote must not link: %s", local)
	}
}
