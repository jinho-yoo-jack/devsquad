package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Authenticator implements the single-user login of 13-Frontend-설계 §8: the
// admin password issues a long-lived HS256 JWT, kept in an HttpOnly cookie or
// sent as a bearer token.
type Authenticator struct {
	Password string
	Secret   []byte
	TTL      time.Duration
	Now      func() time.Time
}

const (
	Cookie  = "devsquad_token"
	Subject = "admin"
	Issuer  = "devsquad"
)

var ErrBadPassword = errors.New("invalid password")

func (a *Authenticator) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
func (a *Authenticator) Login(password string) (string, time.Time, error) {
	want, got := sha256.Sum256([]byte(a.Password)), sha256.Sum256([]byte(password))
	if a.Password == "" || subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return "", time.Time{}, ErrBadPassword
	}
	// JWT dates have second precision.
	now := a.now().Truncate(time.Second)
	expires := now.Add(a.TTL)
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: Subject, Issuer: Issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}).SignedString(a.Secret)
	return token, expires, e
}
func (a *Authenticator) Verify(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, e := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return a.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(Issuer), jwt.WithExpirationRequired(), jwt.WithTimeFunc(a.now))
	if e != nil {
		return "", e
	}
	if claims.Subject != Subject {
		return "", errors.New("unknown subject")
	}
	return claims.Subject, nil
}

type userKey struct{}

func WithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}
func User(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userKey{}).(string)
	return user, ok
}

var public = map[string]bool{"/api/v1/health": true, "/api/v1/auth/login": true, "/api/v1/auth/logout": true}

func protected(path string) bool {
	return path == "/ws" || (strings.HasPrefix(path, "/api/") && !public[path])
}

// Middleware guards the API and WebSocket. Browsers cannot set headers on a
// WebSocket, so /ws also accepts the token as a query parameter.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if v, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			token = v
		} else if c, e := r.Cookie(Cookie); e == nil {
			token = c.Value
		} else if r.URL.Path == "/ws" {
			token = r.URL.Query().Get("token")
		}
		if user, e := a.Verify(token); e == nil {
			r = r.WithContext(WithUser(r.Context(), user))
		} else if protected(r.URL.Path) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"UNAUTHORIZED","message":"authentication required","details":{}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
