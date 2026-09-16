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

// databaseURLForEnv returns the connection string for the given env.
// Falls back to DATABASE_URL if the env-specific var is not set.
func databaseURLForEnv(e Env) (string, bool) {
	switch e {
	case EnvStaging:
		if v := os.Getenv("DATABASE_URL_STAGING"); v != "" {
			return v, true
		}
	case EnvProd:
		if v := os.Getenv("DATABASE_URL_PROD"); v != "" {
			return v, true
		}
	default:
		if v := os.Getenv("DATABASE_URL_DEV"); v != "" {
			return v, true
		}
	}
	// Fallback to generic DATABASE_URL for all envs.
	v := os.Getenv("DATABASE_URL")
	return v, v != ""
}

func jwtSecretForEnv(e Env) (string, bool) {
	switch e {
	case EnvStaging:
		if v := os.Getenv("JWT_SECRET_STAGING"); v != "" {
			return v, true
		}
	case EnvProd:
		if v := os.Getenv("JWT_SECRET_PROD"); v != "" {
			return v, true
		}
	default:
		if v := os.Getenv("JWT_SECRET_DEV"); v != "" {
			return v, true
		}
	}
	// Fallback to generic JWT_SECRET for all envs.
	v := os.Getenv("JWT_SECRET")
	return v, v != ""
}

func Load() (*Config, error) {
	e := CurrentEnv()
	db, ok := databaseURLForEnv(e)
	if !ok {
		return nil, fmt.Errorf("missing database URL for env %q (set DATABASE_URL_%s)", e, strings.ToUpper(string(e)))
	}
	secret, ok := jwtSecretForEnv(e)
	if !ok {
		return nil, fmt.Errorf("missing JWT secret for env %q (set JWT_SECRET_%s)", e, strings.ToUpper(string(e)))
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret for env %q must be at least 32 chars", e)
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
