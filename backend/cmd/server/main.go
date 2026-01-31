package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"amy/minecraft-server/internal/config"
	"amy/minecraft-server/internal/db"
	"amy/minecraft-server/internal/handlers"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := db.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to connect to mongo: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(shutdownCtx)
	}()

	database := client.Database(cfg.MongoDB)

	healthHandler := handlers.NewHealthHandler(database)
	playerHandler := handlers.NewPlayerHandler(database)
	newsHandler := handlers.NewNewsHandler(database)
	authHandler := handlers.NewAuthHandler(database)
	supportHandler := handlers.NewSupportHandler(database, cfg.DiscordTicketWebhook)
	discordHandler := handlers.NewDiscordAuthHandler(
		database,
		cfg.DiscordClientID,
		cfg.DiscordClientSecret,
		cfg.DiscordRedirectURL,
		cfg.FrontendURL,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler.Handle)
	mux.HandleFunc("/api/players", playerHandler.List)
	mux.HandleFunc("/api/players/register", playerHandler.Register)
	mux.HandleFunc("/api/news", newsHandler.List)
	mux.HandleFunc("/api/auth/register", authHandler.Register)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/discord/start", discordHandler.Start)
	mux.HandleFunc("/api/auth/discord/callback", discordHandler.Callback)
	mux.HandleFunc("/api/auth/me", discordHandler.Me)
	mux.HandleFunc("/api/support/tickets", supportHandler.Create)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(cfg.FrontendURL, withLogging(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowedOrigin == "" {
			allowedOrigin = "*"
		}
		if r.Header.Get("Origin") == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
