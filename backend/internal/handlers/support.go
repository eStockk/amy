package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"amy/minecraft-server/internal/observability"
)

type SupportHandler struct {
	db         *sql.DB
	webhookURL string
}

type ticketRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Subject  string `json:"subject"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

func NewSupportHandler(db *sql.DB, webhookURL string) *SupportHandler {
	return &SupportHandler{
		db:         db,
		webhookURL: webhookURL,
	}
}

func (h *SupportHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload ticketRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Subject = strings.TrimSpace(payload.Subject)
	payload.Category = strings.TrimSpace(payload.Category)
	payload.Message = strings.TrimSpace(payload.Message)

	if payload.Name == "" || payload.Email == "" || payload.Subject == "" || payload.Message == "" {
		writeError(w, http.StatusBadRequest, "name, email, subject, message required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ticket := models.Ticket{
		Name:      payload.Name,
		Email:     payload.Email,
		Subject:   payload.Subject,
		Category:  payload.Category,
		Message:   payload.Message,
		CreatedAt: time.Now().UTC(),
	}

	err := h.db.QueryRowContext(
		ctx,
		`INSERT INTO support_tickets (name, email, subject, category, message, created_at)
         VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id`,
		ticket.Name,
		ticket.Email,
		ticket.Subject,
		ticket.Category,
		ticket.Message,
		ticket.CreatedAt,
	).Scan(&ticket.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	h.sendDiscordWebhook(payload)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *SupportHandler) sendDiscordWebhook(payload ticketRequest) {
	if h.webhookURL == "" {
		return
	}

	embed := map[string]any{
		"title": payload.Subject,
		"fields": []map[string]string{
			{"name": "Name", "value": payload.Name},
			{"name": "Email", "value": payload.Email},
			{"name": "Category", "value": payload.Category},
			{"name": "Message", "value": payload.Message},
		},
	}

	body := map[string]any{
		"content": "New support ticket",
		"embeds":  []any{embed},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, h.webhookURL, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
		_ = resp.Body.Close()
	}
	observability.ObserveDiscordOutbound("support_ticket", startedAt, statusCode, err)
}
