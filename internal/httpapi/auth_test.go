package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jinho-yoo-jack/devsquad/internal/app"
	"github.com/jinho-yoo-jack/devsquad/internal/auth"
	"github.com/jinho-yoo-jack/devsquad/internal/httpapi"
)

func server(t *testing.T, authn *auth.Authenticator) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(httpapi.Handler(&app.ProjectService{}, &app.TaskService{}, &app.ApprovalService{}, nil, nil, authn))
	t.Cleanup(s.Close)
	return s
}
func call(t *testing.T, s *httptest.Server, method, path string, body any, header http.Header) (*http.Response, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, s.URL+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}
func session(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == auth.Cookie {
			return c
		}
	}
	return nil
}

// 13-Frontend-설계 §8: a long-lived JWT kept in an HttpOnly cookie.
func TestLoginLogoutAndMe(t *testing.T) {
	s := server(t, &auth.Authenticator{Password: "pw", Secret: []byte(strings.Repeat("k", 32)), TTL: 24 * time.Hour})
	res, body := call(t, s, "POST", "/api/v1/auth/login", map[string]any{"password": "nope"}, nil)
	if res.StatusCode != 401 || body["code"] != "UNAUTHORIZED" || session(res) != nil {
		t.Fatalf("wrong password: %d %v", res.StatusCode, body)
	}
	res, body = call(t, s, "POST", "/api/v1/auth/login", map[string]any{"password": "pw"}, nil)
	c := session(res)
	if res.StatusCode != 200 || c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure || c.MaxAge != 86400 {
		t.Fatalf("login: %d %+v", res.StatusCode, c)
	}
	if body["user"] != auth.Subject || body["token"] != c.Value || body["expires_at"] == nil {
		t.Fatalf("login body: %v", body)
	}
	if res, body = call(t, s, "GET", "/api/v1/me", nil, nil); res.StatusCode != 401 {
		t.Fatalf("me without session: %d %v", res.StatusCode, body)
	}
	if res, body = call(t, s, "GET", "/api/v1/me", nil, http.Header{"Cookie": {c.String()}}); res.StatusCode != 200 || body["user"] != auth.Subject || body["auth_enabled"] != true {
		t.Fatalf("me: %d %v", res.StatusCode, body)
	}
	if res, body = call(t, s, "GET", "/api/v1/me", nil, http.Header{"Authorization": {"Bearer " + c.Value}}); res.StatusCode != 200 {
		t.Fatalf("bearer: %d %v", res.StatusCode, body)
	}
	if res, _ = call(t, s, "GET", "/api/v1/projects", nil, nil); res.StatusCode != 401 {
		t.Fatalf("projects without session: %d", res.StatusCode)
	}
	res, _ = call(t, s, "POST", "/api/v1/auth/logout", nil, nil)
	if c = session(res); res.StatusCode != 204 || c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("logout: %d %+v", res.StatusCode, c)
	}
	res, _ = call(t, s, "POST", "/api/v1/auth/login", map[string]any{"password": "pw"}, http.Header{"X-Forwarded-Proto": {"https"}})
	if c = session(res); c == nil || !c.Secure {
		t.Fatalf("https cookie: %+v", c)
	}
}

func TestMeWithoutAuthentication(t *testing.T) {
	s := server(t, nil)
	res, body := call(t, s, "GET", "/api/v1/me", nil, http.Header{"X-User": {"dev"}})
	if res.StatusCode != 200 || body["user"] != "dev" || body["auth_enabled"] != false {
		t.Fatalf("me: %d %v", res.StatusCode, body)
	}
	if res, _ = call(t, s, "POST", "/api/v1/auth/login", map[string]any{"password": "x"}, nil); res.StatusCode != 404 {
		t.Fatalf("login exists without authentication: %d", res.StatusCode)
	}
}
