package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"amy/minecraft-server/internal/config"
	"amy/minecraft-server/internal/db"
	"amy/minecraft-server/internal/handlers"
	"amy/minecraft-server/internal/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	postgres, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer postgres.Close()

	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.Migrate(migrationCtx, postgres); err != nil {
		migrationCancel()
		log.Fatalf("failed to migrate postgres: %v", err)
	}
	migrationCancel()
	observability.RegisterDatabaseUp(postgres)
	observability.SetDiscordConfig(
		cfg.DiscordClientID != "" && cfg.DiscordClientSecret != "" && cfg.DiscordRedirectURL != "",
		map[string]bool{
			"oauth":          cfg.DiscordClientID != "" && cfg.DiscordClientSecret != "" && cfg.DiscordRedirectURL != "",
			"rp_application": cfg.DiscordRPWebhook != "",
			"support_ticket": cfg.DiscordTicketWebhook != "",
			"news_fetch":     cfg.DiscordBotToken != "" && cfg.DiscordNewsChannelID != "",
		},
	)

	healthHandler := handlers.NewHealthHandler(postgres)
	playerHandler := handlers.NewPlayerHandler(postgres)
	newsHandler := handlers.NewNewsHandler(postgres, cfg.TelegramNewsChannel, cfg.DiscordBotToken, cfg.DiscordNewsChannelID)
	discordMemberSync := handlers.NewDiscordMemberSync(postgres, cfg.DiscordBotToken, cfg.DiscordGuildID)
	serverStatusHandler := handlers.NewServerStatusHandler(cfg.MinecraftServerAddr)
	supportHandler := handlers.NewSupportHandler(postgres, cfg.DiscordTicketWebhook, cfg.FrontendURL)
	discordHandler := handlers.NewDiscordAuthHandler(
		postgres,
		cfg.DiscordClientID,
		cfg.DiscordClientSecret,
		cfg.DiscordRedirectURL,
		cfg.FrontendURL,
		cfg.DiscordTicketWebhook,
		cfg.DiscordRPWebhook,
		cfg.RPModeratorIDs,
		cfg.MinecraftServerToken,
		cfg.MinecraftServerAddr,
	)

	syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := discordHandler.RunMigrations(syncCtx); err != nil {
		log.Printf("discord migrations failed: %v", err)
	}
	syncCancel()
	discordMemberSync.Start(ctx)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/health", healthHandler.Handle)
	mux.HandleFunc("/api/players", playerHandler.List)
	mux.HandleFunc("/api/players/register", playerHandler.Register)
	mux.HandleFunc("/api/news", newsHandler.List)
	mux.HandleFunc("/api/server/status", serverStatusHandler.Handle)
	mux.HandleFunc("/api/auth/discord/start", discordHandler.Start)
	mux.HandleFunc("/api/auth/discord/callback", discordHandler.Callback)
	mux.HandleFunc("/api/auth/me", discordHandler.Me)
	mux.HandleFunc("/api/auth/logout", discordHandler.Logout)
	mux.HandleFunc("/api/auth/presence", discordHandler.PresencePing)
	mux.HandleFunc("/api/auth/verify-minecraft", discordHandler.VerifyMinecraftCode)
	mux.HandleFunc("/api/profiles/", discordHandler.PublicProfile)
	mux.HandleFunc("/api/rp/applications", discordHandler.SubmitRPApplication)
	mux.HandleFunc("/api/rp/applications/", discordHandler.ModerateRPApplication)
	mux.HandleFunc("/api/minecraft/verification-code", discordHandler.RequestMinecraftVerificationCode)
	mux.HandleFunc("/api/minecraft/rp-name", discordHandler.UpdateMineRPName)
	mux.HandleFunc("/api/support/tickets", supportHandler.Create)
	mux.HandleFunc("/api/support/tickets/", supportHandler.Moderate)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(cfg.FrontendURL, withMetrics(withLogging(mux))),
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		observability.HTTPRequestsInFlight.Inc()
		defer observability.HTTPRequestsInFlight.Dec()

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		status := strconv.Itoa(recorder.status)
		route := r.URL.Path
		observability.HTTPRequestsTotal.WithLabelValues(r.Method, route, status, observability.ClientIP(r)).Inc()
		observability.HTTPRequestDuration.WithLabelValues(r.Method, route, status).Observe(time.Since(start).Seconds())
	})
}

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowedOrigin == "" {
			allowedOrigin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Server-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
