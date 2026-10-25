// Package config loads and validates all runtime configuration from the
// environment exactly once at startup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the typed application configuration. It is built once in main and
// passed by pointer to the components that need it.
type Config struct {
	Env      string
	Port     string
	Timezone string
	CORS     []string

	DatabaseURL string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	BcryptCost       int
	InviteTTL        time.Duration

	RateLimitPerMinute int

	AIProvider string
	AIAPIKey   string
	AIModel    string
	AITimeout  time.Duration

	StorageEndpoint  string
	StorageBucket    string
	StorageAccessKey string
	StorageSecretKey string
	SignedURLTTL     time.Duration
	// LocalStorageDir is used when no S3-compatible storage is configured.
	// Exports land on disk and are served through the API.
	LocalStorageDir string

	ChromeBin            string
	ExportTimeout        time.Duration
	ExportBaseURL        string
	TemplateDir          string
	SentryDSN            string
	LogLevel             string
	SeedDemoUsers        bool
	DefaultUserPassword  string
	FrontendDistDir      string
	DisableRateLimit     bool
	MaxExportConcurrency int
}

// Load reads configuration from the environment, applying the documented
// defaults from PRD Section 6.5.
func Load() (*Config, error) {
	c := &Config{
		Env:      env("APP_ENV", "development"),
		Port:     env("APP_PORT", "8080"),
		Timezone: env("APP_TIMEZONE", "Asia/Jakarta"),
		CORS:     splitCSV(env("CORS_ORIGINS", "http://localhost:9000,http://localhost:8080")),

		DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/legacy_intercessor?sslmode=disable"),

		JWTAccessSecret:  env("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret: env("JWT_REFRESH_SECRET", ""),
		JWTAccessTTL:     envDuration("JWT_ACCESS_TTL", time.Hour),
		JWTRefreshTTL:    envDuration("JWT_REFRESH_TTL", 168*time.Hour),
		BcryptCost:       envInt("BCRYPT_COST", 12),
		InviteTTL:        envDuration("INVITE_TTL", 72*time.Hour),

		RateLimitPerMinute: envInt("RATE_LIMIT_PER_MINUTE", 100),

		AIProvider: strings.ToLower(env("AI_PROVIDER", "")),
		AIAPIKey:   env("AI_API_KEY", ""),
		AIModel:    env("AI_MODEL", ""),
		AITimeout:  envDuration("AI_TIMEOUT_SECONDS", 15*time.Second),

		StorageEndpoint:  env("STORAGE_ENDPOINT", ""),
		StorageBucket:    env("STORAGE_BUCKET", ""),
		StorageAccessKey: env("STORAGE_ACCESS_KEY", ""),
		StorageSecretKey: env("STORAGE_SECRET_KEY", ""),
		SignedURLTTL:     envDuration("SIGNED_URL_TTL", 10*time.Minute),
		LocalStorageDir:  env("LOCAL_STORAGE_DIR", "./var/exports"),

		ChromeBin:            env("CHROME_BIN", ""),
		ExportTimeout:        envDuration("EXPORT_TIMEOUT_SECONDS", 30*time.Second),
		ExportBaseURL:        env("EXPORT_BASE_URL", ""),
		TemplateDir:          env("TEMPLATE_DIR", "./templates"),
		SentryDSN:            env("SENTRY_DSN", ""),
		LogLevel:             env("LOG_LEVEL", "info"),
		SeedDemoUsers:        envBool("SEED_DEMO_USERS", false),
		DefaultUserPassword:  env("DEFAULT_USER_PASSWORD", "LegacyIntercessor#2026"),
		FrontendDistDir:      env("FRONTEND_DIST_DIR", ""),
		DisableRateLimit:     envBool("DISABLE_RATE_LIMIT", false),
		MaxExportConcurrency: envInt("MAX_EXPORT_CONCURRENCY", 1),
	}

	if c.Env != "production" && c.JWTAccessSecret == "" {
		c.JWTAccessSecret = "dev-access-secret-change-me"
	}
	if c.Env != "production" && c.JWTRefreshSecret == "" {
		c.JWTRefreshSecret = "dev-refresh-secret-change-me"
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.Env == "production" {
		if c.JWTAccessSecret == "" || c.JWTRefreshSecret == "" {
			return fmt.Errorf("config: JWT_ACCESS_SECRET and JWT_REFRESH_SECRET are required in production")
		}
		if len(c.JWTAccessSecret) < 32 || len(c.JWTRefreshSecret) < 32 {
			return fmt.Errorf("config: JWT secrets must be at least 32 characters in production")
		}
		if strings.Contains(c.DatabaseURL, "postgres:postgres@") {
			return fmt.Errorf("config: default database credentials are not allowed in production")
		}
	}
	if c.BcryptCost < 10 || c.BcryptCost > 15 {
		return fmt.Errorf("config: BCRYPT_COST must be between 10 and 15")
	}
	if c.RateLimitPerMinute < 1 {
		return fmt.Errorf("config: RATE_LIMIT_PER_MINUTE must be positive")
	}
	return nil
}

// IsProduction reports whether the app runs with production safeguards.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// envDuration accepts either a Go duration ("1h", "90s") or a bare number of
// seconds ("15"), because the PRD writes AI_TIMEOUT_SECONDS=15.
func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
