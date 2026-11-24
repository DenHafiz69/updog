package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/DenHafiz69/updog/internal/alerts"
	"github.com/DenHafiz69/updog/internal/api"
	"github.com/DenHafiz69/updog/internal/checker"
	"github.com/DenHafiz69/updog/internal/scheduler"
	"github.com/DenHafiz69/updog/internal/sse"
	"github.com/DenHafiz69/updog/internal/store"
	"github.com/DenHafiz69/updog/web"
)

func main() {
	addr := flag.String("addr", envOrDefault("UPDOG_ADDR", ":8080"), "HTTP listen address")
	dbPath := flag.String("db", envOrDefault("UPDOG_DB", "updog.db"), "SQLite database file path")
	retentionDays := flag.Int("retention", envIntOrDefault("UPDOG_RETENTION_DAYS", 30), "Check history retention in days")
	password := flag.String("password", os.Getenv("UPDOG_PASSWORD"), "Admin password (recommended when exposed publicly; supports any length)")
	flag.Parse()

	if *password == "" {
		log.Println("WARNING: UPDOG_PASSWORD is not set; dashboard & API are unauthenticated. Set UPDOG_PASSWORD before exposing publicly.")
	} else {
		log.Println("Admin password authentication enabled.")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("failed to open sqlite store: %v", err)
	}
	defer st.Close()

	hub := sse.NewHub()
	chk := checker.New()
	dispatcher := alerts.NewDispatcher(st)
	sched := scheduler.New(st, chk, dispatcher, hub, *retentionDays)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go sched.Start(ctx)

	srv := api.NewServer(st, sched, dispatcher, hub, web.FS(), *password)
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Updog listening on http://localhost%s (db: %s)", *addr, *dbPath)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down updog...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
