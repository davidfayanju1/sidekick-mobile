// Command api is the server entrypoint: env-scoped config, migrations,
// Phase 1 identity routes and a health probe.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sidekick/backend/internal/auth"
	"github.com/sidekick/backend/internal/avatar"
	"github.com/sidekick/backend/internal/config"
	"github.com/sidekick/backend/internal/db"
	"github.com/sidekick/backend/internal/httpapi"
	"github.com/sidekick/backend/internal/sandbox"
	"github.com/sidekick/backend/internal/storage"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	issuer, err := auth.NewIssuer(cfg.JWTSecret, string(cfg.Env))
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	signer := storage.NewStore(os.Getenv("JWT_SECRET_" + string(cfg.Env)) + "-storage")
	api := httpapi.New(pool, issuer, sandbox.Verifier{}, &sandbox.Mailer{},
		&sandbox.SMSSender{}, signer, avatar.NewMemoryBlobStore())

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"env":"` + string(cfg.Env) + `"}`))
	})
	mux.Handle("/", api.Handler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("sidekick api env=%s port=%s", cfg.Env, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
