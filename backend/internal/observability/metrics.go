package observability

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amy_backend_http_requests_total",
			Help: "Total backend HTTP requests.",
		},
		[]string{"method", "path", "status", "client_ip"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "amy_backend_http_request_duration_seconds",
			Help:    "Backend HTTP request duration.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "amy_backend_http_requests_in_flight",
			Help: "Backend HTTP requests currently in flight.",
		},
	)
	DiscordOutboundRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amy_backend_discord_outbound_requests_total",
			Help: "Discord outbound requests made by the backend.",
		},
		[]string{"kind", "result", "status"},
	)
	DiscordOutboundRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "amy_backend_discord_outbound_request_duration_seconds",
			Help:    "Discord outbound request duration.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"kind", "result"},
	)
	DiscordOutboundLastSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amy_backend_discord_outbound_last_success_timestamp_seconds",
			Help: "Unix timestamp of the latest successful Discord outbound request.",
		},
		[]string{"kind"},
	)
	DiscordIntegrationConfigured = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "amy_backend_discord_integration_configured",
			Help: "Whether a Discord integration is configured. 1 means configured, 0 means missing config.",
		},
		[]string{"kind"},
	)
	DiscordOAuthConfigured = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "amy_backend_discord_oauth_configured",
			Help: "Whether Discord OAuth has client id, client secret, and redirect URL configured.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		DiscordOutboundRequestsTotal,
		DiscordOutboundRequestDuration,
		DiscordOutboundLastSuccess,
		DiscordIntegrationConfigured,
		DiscordOAuthConfigured,
	)
}

func RegisterDatabaseUp(db *sql.DB) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "amy_backend_database_up",
			Help: "Whether the backend can ping PostgreSQL. 1 means up, 0 means down.",
		},
		func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				return 0
			}
			return 1
		},
	))
}

func SetDiscordConfig(oauthConfigured bool, integrations map[string]bool) {
	if oauthConfigured {
		DiscordOAuthConfigured.Set(1)
	} else {
		DiscordOAuthConfigured.Set(0)
	}

	for kind, configured := range integrations {
		if configured {
			DiscordIntegrationConfigured.WithLabelValues(kind).Set(1)
		} else {
			DiscordIntegrationConfigured.WithLabelValues(kind).Set(0)
		}
	}
}

func ObserveDiscordOutbound(kind string, startedAt time.Time, statusCode int, err error) {
	result := "success"
	status := strconv.Itoa(statusCode)
	if statusCode == 0 {
		status = "none"
	}
	if err != nil || statusCode < 200 || statusCode > 299 {
		result = "error"
	}

	DiscordOutboundRequestsTotal.WithLabelValues(kind, result, status).Inc()
	DiscordOutboundRequestDuration.WithLabelValues(kind, result).Observe(time.Since(startedAt).Seconds())
	if result == "success" {
		DiscordOutboundLastSuccess.WithLabelValues(kind).Set(float64(time.Now().Unix()))
	}
}

func ClientIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if parsed := net.ParseIP(value); parsed != nil {
			return parsed.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if parsed := net.ParseIP(host); parsed != nil {
			return parsed.String()
		}
	}
	if parsed := net.ParseIP(r.RemoteAddr); parsed != nil {
		return parsed.String()
	}
	return "unknown"
}
