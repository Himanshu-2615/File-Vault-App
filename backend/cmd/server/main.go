package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"

	"github.com/himanshu/file-vault-app/backend/internal/config"
	"github.com/himanshu/file-vault-app/backend/internal/graph"
	"github.com/himanshu/file-vault-app/backend/internal/httpext"
	"github.com/himanshu/file-vault-app/backend/internal/rate"
	"github.com/himanshu/file-vault-app/backend/internal/repo"
	"github.com/himanshu/file-vault-app/backend/internal/storage"
)

func main() {
	addr := getenv("PORT", "8081")

	cfg := config.FromEnv()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	repository := repo.New(pool)
	// run migrations on startup (idempotent)
	if err := repo.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	store := storage.New(cfg.StorageDir)
	limiter := rate.NewLimiter(cfg.RateLimitRPS)

	// simple user identity via header for now
	getUser := func(r *http.Request) string { return r.Header.Get("X-User-ID") }

	r.Group(func(gr chi.Router) {
		gr.Use(limiter.Middleware(func(r *http.Request) string { return getUser(r) }))
		httpext.RegisterUploadRoutes(gr, httpext.UploadDeps{Storage: store, Repo: repository, MaxFormMemory: 32 << 20, GetUserID: getUser})
	})

	// Public downloads
	httpext.RegisterPublicRoutes(r, httpext.PublicDeps{Repo: repository})

	// GraphQL
	r.Handle("/graphql", graph.NewHandler(graph.Deps{Repo: repository, GetUserID: getUser}))

	handler := cors.AllowAll().Handler(r)
	server := &http.Server{Addr: ":" + addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Printf("server listening on :%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
