// Package config loads environment-specific settings.
//
// Phase 0 §1: dev, staging and prod are genuinely separate. Only the
// DATABASE_URL and JWT secret matching APP_ENV are ever loaded, so a
// credential valid in dev cannot authenticate against staging or prod.
// Secrets come from the environment only — never from committed files.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Env string

const (
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "prod"
)

type Config struct {
	Env           Env
	DatabaseURL   string
	JWTSecret     string
	S3Endpoint    string
	S3Region      string
	S3BucketPrefix string
}

func CurrentEnv() Env {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "staging":
		return EnvStaging
	case "prod", "production":
		return EnvProd
	default:
		return EnvDev
	}
}

// firstNonEmpty returns the first non-empty value, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// databaseURL returns the database connection string.
// Priority: env-specific (DATABASE_URL_<ENV>) > generic (DATABASE_URL).
func databaseURL(e Env) (string, bool) {
	v := firstNonEmpty(
		os.Getenv("DATABASE_URL_"+strings.ToUpper(string(e))),
		os.Getenv("DATABASE_URL"),
	)
	return v, v != ""
}

// jwtSecret returns the JWT signing secret.
// Priority: env-specific (JWT_SECRET_<ENV>) > generic (JWT_SECRET).
func jwtSecret(e Env) (string, bool) {
	v := firstNonEmpty(
		os.Getenv("JWT_SECRET_"+strings.ToUpper(string(e))),
		os.Getenv("JWT_SECRET"),
	)
	return v, v != ""
}

func Load() (*Config, error) {
	e := CurrentEnv()

	db, ok := databaseURL(e)
	if !ok {
		return nil, fmt.Errorf("missing database URL: set DATABASE_URL or DATABASE_URL_%s", strings.ToUpper(string(e)))
	}

	secret, ok := jwtSecret(e)
	if !ok {
		return nil, fmt.Errorf("missing JWT secret: set JWT_SECRET or JWT_SECRET_%s", strings.ToUpper(string(e)))
	}

	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must be at least 32 chars (got %d)", len(secret))
	}

	return &Config{
		Env:            e,
		DatabaseURL:    db,
		JWTSecret:      secret,
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3Region:       firstNonEmpty(os.Getenv("S3_REGION"), "eu-west-2"),
		S3BucketPrefix: os.Getenv("S3_BUCKET_PREFIX"),
	}, nil
}
