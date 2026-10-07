package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jinho-yoo-jack/devsquad/internal/auth"
	"github.com/jinho-yoo-jack/devsquad/internal/domain"
)

type AuthController struct{ Auth *auth.Authenticator }
type Actor struct {
	User string `header:"X-User" default:"local-user"`
}
type Login struct {
	Proto string `header:"X-Forwarded-Proto"`
	Body  struct {
		Password string `json:"password"`
	}
}
type Logout struct {
	Proto string `header:"X-Forwarded-Proto"`
}
type Session struct {
	User      string    `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Me struct {
	User        string `json:"user"`
	AuthEnabled bool   `json:"auth_enabled"`
}
type withCookie struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      *Session
}

// actor is the authenticated user. X-User names the actor only when
// authentication is disabled for local development.
func actor(ctx context.Context, header string) string {
	if user, ok := auth.User(ctx); ok {
		return user
	}
	return header
}
func sessionCookie(value, proto string, maxAge int) http.Cookie {
	return http.Cookie{Name: auth.Cookie, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: proto == "https"}
}
func registerMe(api huma.API) {
	register(api, "me", "GET", "/api/v1/me", 200, func(ctx context.Context, in *Actor) (Me, error) {
		if user, ok := auth.User(ctx); ok {
			return Me{User: user, AuthEnabled: true}, nil
		}
		return Me{User: in.User}, nil
	})
}
func (c AuthController) Register(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "auth-login", Method: "POST", Path: "/api/v1/auth/login", MaxBodyBytes: 1 << 10, Errors: []int{400, 401}}, func(ctx context.Context, in *Login) (*withCookie, error) {
		token, expires, e := c.Auth.Login(in.Body.Password)
		if e != nil {
			return nil, domain.Fault(401, "UNAUTHORIZED", "invalid password")
		}
		return &withCookie{sessionCookie(token, in.Proto, int(c.Auth.TTL.Seconds())), &Session{auth.Subject, token, expires}}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "auth-logout", Method: "POST", Path: "/api/v1/auth/logout", DefaultStatus: 204}, func(ctx context.Context, in *Logout) (*withCookie, error) {
		return &withCookie{SetCookie: sessionCookie("", in.Proto, -1)}, nil
	})
}
