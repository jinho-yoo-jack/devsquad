package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func authenticator(now time.Time) *Authenticator {
	return &Authenticator{Password: "correct horse", Secret: secret, TTL: time.Hour, Now: func() time.Time { return now }}
}

func TestLoginIssuesVerifiableToken(t *testing.T) {
	now := time.Now()
	a := authenticator(now)
	if _, _, e := a.Login("wrong"); !errors.Is(e, ErrBadPassword) {
		t.Fatalf("wrong password: %v", e)
	}
	if _, _, e := a.Login(""); !errors.Is(e, ErrBadPassword) {
		t.Fatalf("empty password: %v", e)
	}
	token, expires, e := a.Login("correct horse")
	if e != nil {
		t.Fatal(e)
	}
	if !expires.Equal(now.Add(time.Hour).Truncate(time.Second)) {
		t.Fatalf("expires=%v", expires)
	}
	if user, e := a.Verify(token); e != nil || user != Subject {
		t.Fatalf("verify: %q %v", user, e)
	}
}

func TestVerifyRejectsForgedOrStaleTokens(t *testing.T) {
	now := time.Now()
	a := authenticator(now)
	valid, _, _ := a.Login("correct horse")
	expired, _, _ := authenticator(now.Add(-2 * time.Hour)).Login("correct horse")
	other, _, _ := (&Authenticator{Password: "x", Secret: []byte("another-secret-another-secret-00"), TTL: time.Hour}).Login("x")
	foreign, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: Subject, Issuer: "elsewhere", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}).SignedString(secret)
	noExpiry, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: Subject, Issuer: Issuer}).SignedString(secret)
	enc := base64.RawURLEncoding.EncodeToString
	none := enc([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + enc([]byte(`{"sub":"admin","iss":"devsquad","exp":9999999999}`)) + "."
	parts := strings.Split(valid, ".")
	tampered := parts[0] + "." + enc([]byte(`{"sub":"root","iss":"devsquad","exp":9999999999}`)) + "." + parts[2]
	for name, token := range map[string]string{"expired": expired, "other secret": other, "foreign issuer": foreign, "no expiry": noExpiry, "alg none": none, "tampered": tampered, "garbage": "x.y.z", "empty": ""} {
		if user, e := a.Verify(token); e == nil {
			t.Errorf("%s accepted as %q", name, user)
		}
	}
}

func TestMiddlewareGuardsAPIAndWebSocket(t *testing.T) {
	a := authenticator(time.Now())
	token, _, _ := a.Login("correct horse")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := User(r.Context())
		_, _ = w.Write([]byte(user))
	})
	h := a.Middleware(next)
	for _, tc := range []struct {
		name, method, path, bearer, cookie string
		want                               int
	}{
		{"api without token", "GET", "/api/v1/tasks", "", "", 401},
		{"api with bearer", "GET", "/api/v1/tasks", token, "", 200},
		{"api with cookie", "POST", "/api/v1/approvals/x/decide", "", token, 200},
		{"api with bad token", "GET", "/api/v1/tasks", "nope", "", 401},
		{"ws without token", "GET", "/ws", "", "", 401},
		{"ws with query token", "GET", "/ws?token=" + token, "", "", 200},
		{"ws with cookie", "GET", "/ws", "", token, 200},
		{"query token only for ws", "GET", "/api/v1/tasks?token=" + token, "", "", 401},
		{"health", "GET", "/api/v1/health", "", "", 200},
		{"legacy health", "GET", "/actuator/health", "", "", 200},
		{"login", "POST", "/api/v1/auth/login", "", "", 200},
		{"logout", "POST", "/api/v1/auth/logout", "", "", 200},
		{"openapi", "GET", "/openapi.json", "", "", 200},
		{"metrics", "GET", "/metrics", "", "", 200},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+tc.bearer)
		}
		if tc.cookie != "" {
			req.AddCookie(&http.Cookie{Name: Cookie, Value: tc.cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: %d want %d", tc.name, rec.Code, tc.want)
			continue
		}
		if tc.want == 401 {
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") || !strings.Contains(rec.Body.String(), `"code":"UNAUTHORIZED"`) || !strings.Contains(rec.Body.String(), `"details":{}`) {
				t.Errorf("%s: error body %s %q", tc.name, ct, rec.Body.String())
			}
		} else if (tc.bearer != "" || tc.cookie != "" || strings.Contains(tc.path, "token=")) && rec.Body.String() != Subject {
			t.Errorf("%s: user not in context: %q", tc.name, rec.Body.String())
		}
	}
}
