package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	DBURL            string        `env:"DEVSQUAD_DB_URL" envDefault:"postgres://devsquad:devsquad@localhost:5432/devsquad"`
	HTTPAddr         string        `env:"DEVSQUAD_HTTP_ADDR" envDefault:":8080"`
	WorkspaceRoot    string        `env:"DEVSQUAD_WORKSPACE_ROOT" envDefault:"/tmp/devsquad-workspace"`
	FakeLLM          bool          `env:"DEVSQUAD_FAKE_LLM" envDefault:"false"`
	RecoveryInterval time.Duration `env:"DEVSQUAD_RECOVERY_INTERVAL" envDefault:"5m"`
	TestTimeout      time.Duration `env:"DEVSQUAD_EXECUTE_TEST_TIMEOUT" envDefault:"600s"`
	OllamaURL        string        `env:"OLLAMA_BASE_URL" envDefault:"http://localhost:11434/v1"`
	AuthDisabled     bool          `env:"DEVSQUAD_AUTH_DISABLED" envDefault:"false"`
	AdminPassword    string        `env:"DEVSQUAD_ADMIN_PASSWORD"`
	JWTSecret        string        `env:"DEVSQUAD_JWT_SECRET"`
	JWTTTL           time.Duration `env:"DEVSQUAD_JWT_TTL" envDefault:"720h"`
}

func Load() (Config, error) {
	_ = godotenv.Load()
	c, e := env.ParseAs[Config]()
	if e != nil {
		return c, e
	}
	if c.RecoveryInterval <= 0 || c.TestTimeout <= 0 {
		return c, fmt.Errorf("recovery interval and test timeout must be positive")
	}
	// Authentication is on by default; only an explicit opt-out skips it.
	if !c.AuthDisabled && (c.AdminPassword == "" || len(c.JWTSecret) < 32 || c.JWTTTL <= 0) {
		return c, fmt.Errorf("set DEVSQUAD_ADMIN_PASSWORD, DEVSQUAD_JWT_SECRET (32+ characters) and a positive DEVSQUAD_JWT_TTL, or DEVSQUAD_AUTH_DISABLED=true for local development")
	}
	return c, nil
}
