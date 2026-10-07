package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Authentication is on unless explicitly disabled, so a missing secret stops startup.
func TestLoadRequiresAuthSecretsUnlessDisabled(t *testing.T) {
	for name, tc := range map[string]struct {
		env map[string]string
		ok  bool
	}{
		"no secrets":        {map[string]string{}, false},
		"password only":     {map[string]string{"DEVSQUAD_ADMIN_PASSWORD": "pw"}, false},
		"short jwt secret":  {map[string]string{"DEVSQUAD_ADMIN_PASSWORD": "pw", "DEVSQUAD_JWT_SECRET": "short"}, false},
		"configured":        {map[string]string{"DEVSQUAD_ADMIN_PASSWORD": "pw", "DEVSQUAD_JWT_SECRET": strings.Repeat("s", 32)}, true},
		"explicit disabled": {map[string]string{"DEVSQUAD_AUTH_DISABLED": "true"}, true},
		"bad ttl":           {map[string]string{"DEVSQUAD_ADMIN_PASSWORD": "pw", "DEVSQUAD_JWT_SECRET": strings.Repeat("s", 32), "DEVSQUAD_JWT_TTL": "0s"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			for _, k := range []string{"DEVSQUAD_AUTH_DISABLED", "DEVSQUAD_ADMIN_PASSWORD", "DEVSQUAD_JWT_SECRET", "DEVSQUAD_JWT_TTL"} {
				t.Setenv(k, "")
				os.Unsetenv(k)
				if v, ok := tc.env[k]; ok {
					t.Setenv(k, v)
				}
			}
			c, e := Load()
			if (e == nil) != tc.ok {
				t.Fatalf("err=%v", e)
			}
			if name == "configured" && c.JWTTTL != 720*time.Hour {
				t.Fatalf("default ttl=%v", c.JWTTTL)
			}
		})
	}
}
