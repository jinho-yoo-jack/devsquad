package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/auth"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/httpapi"
	"github.com/jinho-yoo-jack/devsquad/internal/ws"
)

// With authentication on, actors come from the token; X-User cannot impersonate.
func TestAuthenticatedActorsAndWebSocket(t *testing.T) {
	h := newHarness(t, gated, nil)
	authn := &auth.Authenticator{Password: "pw", Secret: []byte(strings.Repeat("k", 32)), TTL: time.Hour}
	hub := ws.New(h.db, h.bus)
	s := httptest.NewServer(httpapi.Handler(h.projects, h.tasks, h.approvals, hub, nil, authn))
	t.Cleanup(func() { hub.Close(); s.Close() })
	token, _, e := authn.Login("pw")
	if e != nil {
		t.Fatal(e)
	}
	send := func(method, path string, body any, want int, out any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, s.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User", "impostor")
		req.AddCookie(&http.Cookie{Name: auth.Cookie, Value: token})
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d", method, path, res.StatusCode, want)
		}
		if out != nil {
			if e = json.NewDecoder(res.Body).Decode(out); e != nil {
				t.Fatal(e)
			}
		}
	}
	var p app.ProjectEntity
	send("POST", "/api/v1/projects", map[string]any{"name": "test", "github_owner": "o", "github_repo": "r", "local_path": h.source}, 201, &p)
	var task app.TaskResponse
	send("POST", "/api/v1/tasks", map[string]any{"project_id": p.ID, "command": "Build"}, 201, &task)
	a := h.pending(task.ID, "a", "plan", 0)
	send("POST", "/api/v1/approvals/"+a.ID+"/decide", map[string]any{"decision": "approve"}, 200, nil)
	stored, _ := h.db.Task(context.Background(), task.ID)
	decided, _ := h.db.Approval(context.Background(), a.ID)
	if stored.CreatedBy != auth.Subject || domain.Text(decided.DecidedBy) != auth.Subject {
		t.Fatalf("created_by=%q decided_by=%q", stored.CreatedBy, domain.Text(decided.DecidedBy))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws"
	if _, res, e := websocket.Dial(ctx, url, nil); e == nil || res == nil || res.StatusCode != 401 {
		t.Fatalf("unauthenticated socket: %v", e)
	}
	conn, _, e := websocket.Dial(ctx, url+"?token="+token, nil)
	if e != nil {
		t.Fatal(e)
	}
	conn.CloseNow()
}
