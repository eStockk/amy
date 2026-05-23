package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"amy/minecraft-server/internal/observability"
)

type SupportHandler struct {
	db          *sql.DB
	webhookURL  string
	frontendURL string
}

type ticketRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	DiscordNick string `json:"discordNick"`
	Subject     string `json:"subject"`
	Category    string `json:"category"`
	Message     string `json:"message"`
}

func NewSupportHandler(db *sql.DB, webhookURL, frontendURL string) *SupportHandler {
	return &SupportHandler{
		db:          db,
		webhookURL:  webhookURL,
		frontendURL: frontendURL,
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
	payload.DiscordNick = strings.TrimSpace(payload.DiscordNick)
	payload.Subject = strings.TrimSpace(payload.Subject)
	payload.Category = strings.TrimSpace(payload.Category)
	payload.Message = strings.TrimSpace(payload.Message)

	if payload.Name == "" || payload.DiscordNick == "" || payload.Subject == "" || payload.Message == "" {
		writeError(w, http.StatusBadRequest, "name, discordNick, subject, message required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ticket := models.Ticket{
		Name:            payload.Name,
		Email:           payload.Email,
		DiscordNick:     payload.DiscordNick,
		Subject:         payload.Subject,
		Category:        payload.Category,
		Message:         payload.Message,
		Status:          "open",
		ModerationToken: randomHex(20),
		CreatedAt:       time.Now().UTC(),
	}

	err := h.db.QueryRowContext(
		ctx,
		`INSERT INTO support_tickets (name, email, discord_nick, subject, category, message, status, moderation_token, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id`,
		ticket.Name,
		ticket.Email,
		ticket.DiscordNick,
		ticket.Subject,
		ticket.Category,
		ticket.Message,
		ticket.Status,
		ticket.ModerationToken,
		ticket.CreatedAt,
	).Scan(&ticket.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	if messageID, err := h.sendDiscordWebhook(ticket); err == nil && messageID != "" {
		_, _ = h.db.ExecContext(ctx, `UPDATE support_tickets SET discord_message_id = $1 WHERE id = $2`, messageID, ticket.ID)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *SupportHandler) Moderate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ticketID, ok := parseSupportModerationIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action != "resolve" && action != "reconsider" {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	ticket, err := h.loadTicket(ctx, ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load ticket")
		return
	}
	if token != ticket.ModerationToken {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	now := time.Now().UTC()
	nextStatus := "resolved"
	var resolvedAt any = now
	if action == "reconsider" {
		nextStatus = "open"
		resolvedAt = nil
		ticket.ResolvedAt = nil
	} else {
		ticket.ResolvedAt = &now
	}

	_, err = h.db.ExecContext(
		ctx,
		`UPDATE support_tickets SET status = $1, resolved_at = $2 WHERE id = $3`,
		nextStatus,
		resolvedAt,
		ticket.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update ticket")
		return
	}

	ticket.Status = nextStatus
	if err := h.updateDiscordTicketMessage(ticket); err != nil {
		writeError(w, http.StatusBadGateway, "failed to update discord ticket")
		return
	}

	h.writeTicketModerationHTML(w, ticket, action)
}

func (h *SupportHandler) sendDiscordWebhook(ticket models.Ticket) (string, error) {
	if h.webhookURL == "" {
		return "", nil
	}

	body := h.buildDiscordTicketPayload(ticket)
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	requestURL, err := webhookURLWithWait(h.webhookURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
		defer resp.Body.Close()
	}
	observability.ObserveDiscordOutbound("support_ticket", startedAt, statusCode, err)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("discord webhook error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}

	var message discordWebhookMessage
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return "", nil
	}
	return strings.TrimSpace(message.ID), nil
}

func (h *SupportHandler) buildDiscordTicketPayload(ticket models.Ticket) map[string]any {
	statusText := "Открыт"
	if normalizedTicketStatus(ticket.Status) == "resolved" {
		statusText = "Решён"
	}

	embed := map[string]any{
		"title":       ticket.Subject,
		"description": "Статус тикета: " + statusText,
		"fields": []map[string]string{
			{"name": "Name", "value": safeValue(ticket.Name)},
			{"name": "Discord", "value": safeValue(ticket.DiscordNick)},
			{"name": "Category", "value": safeValue(ticket.Category)},
			{"name": "Message", "value": trimForDiscord(ticket.Message)},
		},
	}

	body := map[string]any{
		"content": fmt.Sprintf("Support ticket #%d: %s", ticket.ID, ticket.Subject),
		"embeds":  []any{embed},
	}
	if components := h.ticketDiscordComponents(ticket); len(components) > 0 {
		body["components"] = components
	}
	return body
}

func (h *SupportHandler) updateDiscordTicketMessage(ticket models.Ticket) error {
	if h.webhookURL == "" || strings.TrimSpace(ticket.DiscordMessageID) == "" {
		return nil
	}

	payload := h.buildDiscordTicketPayload(ticket)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	base, err := webhookBaseURL(h.webhookURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, base+"/messages/"+url.PathEscape(ticket.DiscordMessageID), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
		_ = resp.Body.Close()
	}
	observability.ObserveDiscordOutbound("support_ticket_update", startedAt, statusCode, err)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("discord webhook edit error: status=%d", resp.StatusCode)
	}
	return nil
}

func (h *SupportHandler) loadTicket(ctx context.Context, ticketID int64) (models.Ticket, error) {
	var ticket models.Ticket
	var resolvedAt sql.NullTime
	err := h.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, discord_nick, subject, category, message, status,
		        moderation_token, discord_message_id, resolved_at, created_at
		 FROM support_tickets
		 WHERE id = $1`,
		ticketID,
	).Scan(
		&ticket.ID,
		&ticket.Name,
		&ticket.Email,
		&ticket.DiscordNick,
		&ticket.Subject,
		&ticket.Category,
		&ticket.Message,
		&ticket.Status,
		&ticket.ModerationToken,
		&ticket.DiscordMessageID,
		&resolvedAt,
		&ticket.CreatedAt,
	)
	if resolvedAt.Valid {
		ticket.ResolvedAt = &resolvedAt.Time
	}
	return ticket, err
}

func (h *SupportHandler) ticketDiscordComponents(ticket models.Ticket) []any {
	label := "Решён"
	action := "resolve"
	if normalizedTicketStatus(ticket.Status) == "resolved" {
		label = "На перерассмотр"
		action = "reconsider"
	}

	return []any{map[string]any{"type": 1, "components": []any{
		map[string]any{
			"type":  2,
			"style": 5,
			"label": label,
			"url":   h.ticketModerationURL(ticket.ID, action, ticket.ModerationToken),
		},
	}}}
}

func (h *SupportHandler) ticketModerationURL(ticketID int64, action, token string) string {
	base := strings.TrimRight(h.frontendURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/api/support/tickets/%d/moderate?action=%s&token=%s", base, ticketID, action, url.QueryEscape(token))
}

func normalizedTicketStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "resolved" {
		return "resolved"
	}
	return "open"
}

func parseSupportModerationIDFromPath(path string) (int64, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "support" || parts[2] != "tickets" || parts[4] != "moderate" {
		return 0, false
	}
	var id int64
	if _, err := fmt.Sscanf(parts[3], "%d", &id); err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (h *SupportHandler) writeTicketModerationHTML(w http.ResponseWriter, ticket models.Ticket, action string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title := "Тикет возвращён на рассмотрение"
	if normalizedTicketStatus(ticket.Status) == "resolved" {
		title = "Тикет отмечен решённым"
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <title>%s</title>
    <style>
      body{font-family:system-ui;background:#0f1118;color:#fff;margin:0;padding:40px}
      .card{max-width:760px;margin:0 auto;padding:24px;border-radius:14px;background:#171a26;border:1px solid rgba(255,255,255,.12)}
      .muted{color:#b4b6c7}
    </style>
  </head>
  <body>
    <div class="card">
      <h1>%s</h1>
      <p class="muted">Тикет #%d: %s</p>
      <p class="muted">Статус: %s</p>
      <p class="muted">Действие: %s</p>
    </div>
  </body>
</html>`, title, title, ticket.ID, ticket.Subject, ticket.Status, action)

	_, _ = w.Write([]byte(html))
}
