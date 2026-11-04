package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dituonline/internal/api"
	"dituonline/internal/auth"
	"dituonline/internal/store"
	"dituonline/internal/world"
	"dituonline/web"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func num(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return d
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if it := num("PBKDF2_ITER", 0); it > 0 {
		auth.Iterations = it
	}
	var st store.Store
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pg, err := store.OpenPostgres(ctx, url)
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		st = pg
		log.Println("database: PostgreSQL connected, migrations applied")
	} else {
		st = store.NewMemory()
		log.Println("WARNING: DATABASE_URL is not set — using the in-memory store; accounts are lost on restart")
	}
	defer st.Close()

	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		secret = auth.RandomSecret()
		log.Println("WARNING: AUTH_SECRET is not set — using a random one; logins will be invalidated on restart")
	}
	var origins []string
	if o := os.Getenv("ALLOWED_ORIGINS"); o != "" {
		origins = strings.Split(o, ",")
	}
	cfg := api.Config{
		MaxPlayers:        num("MAX_PLAYERS_PER_PRISON", 24),
		MaxInstances:      num("MAX_INSTANCES", 200),
		DaySeconds:        num("GAME_DAY_SECONDS", 600),
		AdminKey:          os.Getenv("ADMIN_KEY"),
		TrustProxy:        env("TRUST_PROXY", "") == "1",
		DisableRateLimits: env("DISABLE_RATE_LIMITS", "") == "1",
		AllowOrigins:      origins,
	}
	if cfg.DisableRateLimits {
		log.Println("WARNING: DISABLE_RATE_LIMITS=1 — HTTP rate limits are off (load testing only)")
	}
	hub := api.NewHub(cfg, st, world.Build())
	srv := api.NewServer(hub, st, auth.NewSigner(secret), cfg, web.Static())
	httpSrv := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}

	go func() {
		log.Printf("listening on %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down: saving players…")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	httpSrv.Shutdown(sctx)
	hub.Shutdown()
	log.Println("bye")
}
