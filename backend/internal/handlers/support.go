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
	"strconv"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"amy/minecraft-server/internal/observability"
	webpush "github.com/SherClockHolmes/webpush-go"
)

type SupportHandler struct {
	db              *sql.DB
	webhookURL      string
	frontendURL     string
	vapidPublicKey  string
	vapidPrivateKey string
	pushSubject     string
}

type ticketRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	DiscordNick string `json:"discordNick"`
	Subject     string `json:"subject"`
	Category    string `json:"category"`
	Message     string `json:"message"`
}

func NewSupportHandler(db *sql.DB, webhookURL, frontendURL, vapidPublicKey, vapidPrivateKey, pushSubject string) *SupportHandler {
	return &SupportHandler{
		db:              db,
		webhookURL:      webhookURL,
		frontendURL:     frontendURL,
		vapidPublicKey:  strings.TrimSpace(vapidPublicKey),
		vapidPrivateKey: strings.TrimSpace(vapidPrivateKey),
		pushSubject:     strings.TrimSpace(pushSubject),
	}
}

func (h *SupportHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.List(w, r)
		return
	}
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

	ownerDiscordID := currentDiscordIDFromCookie(r)
	ticket := models.Ticket{
		Name:            payload.Name,
		Email:           payload.Email,
		DiscordNick:     payload.DiscordNick,
		OwnerDiscordID:  ownerDiscordID,
		Subject:         payload.Subject,
		Category:        payload.Category,
		Message:         payload.Message,
		Status:          "open",
		ModerationToken: randomHex(20),
		CreatedAt:       time.Now().UTC(),
	}

	err := h.db.QueryRowContext(
		ctx,
		`INSERT INTO support_tickets (name, email, discord_nick, owner_discord_id, subject, category, message, status, moderation_token, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
         RETURNING id`,
		ticket.Name,
		ticket.Email,
		ticket.DiscordNick,
		ticket.OwnerDiscordID,
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

	_, _ = h.db.ExecContext(
		ctx,
		`INSERT INTO support_ticket_messages (ticket_id, author_type, author_name, author_discord_id, message, read_by_user, created_at)
		 VALUES ($1, 'user', $2, $3, $4, TRUE, $5)`,
		ticket.ID,
		ticket.Name,
		ticket.OwnerDiscordID,
		ticket.Message,
		ticket.CreatedAt,
	)

	if messageID, channelID, err := h.sendDiscordWebhook(ticket); err == nil && messageID != "" {
		ticket.DiscordMessageID = messageID
		ticket.DiscordChannelID = channelID
		_, _ = h.db.ExecContext(ctx, `UPDATE support_tickets SET discord_message_id = $1, discord_channel_id = $2 WHERE id = $3`, messageID, ticket.DiscordChannelID, ticket.ID)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "ticket": ticket})
}

func (h *SupportHandler) Moderate(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/notifications") {
		h.Notifications(w, r)
		return
	}
	if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/messages") {
		h.Messages(w, r)
		return
	}
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

func (h *SupportHandler) sendDiscordWebhook(ticket models.Ticket) (string, string, error) {
	if h.webhookURL == "" {
		return "", "", nil
	}

	body := h.buildDiscordTicketPayload(ticket)
	raw, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	requestURL, err := webhookURLWithWait(h.webhookURL)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return "", "", err
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
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", "", fmt.Errorf("discord webhook error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}

	var message discordWebhookMessage
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return "", "", nil
	}
	return strings.TrimSpace(message.ID), strings.TrimSpace(message.ChannelID), nil
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

	req, err := http.NewRequest(http.MethodPatch, base+"/messages/"+url.PathEscape(ticket.DiscordMessageID)+"?with_components=true", bytes.NewReader(raw))
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
	return scanSupportTicket(h.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, discord_nick, owner_discord_id, subject, category, message, status,
		        moderation_token, discord_message_id, discord_channel_id,
		        (SELECT COUNT(*) FROM support_ticket_messages m WHERE m.ticket_id = support_tickets.id AND m.author_type = 'admin' AND m.read_by_user = FALSE) AS unread_admin_count,
		        resolved_at, created_at
		 FROM support_tickets
		 WHERE id = $1`,
		ticketID,
	))
}

func (h *SupportHandler) List(w http.ResponseWriter, r *http.Request) {
	ownerDiscordID := currentDiscordIDFromCookie(r)
	if ownerDiscordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.db.QueryContext(
		ctx,
		`SELECT id, name, email, discord_nick, owner_discord_id, subject, category, message, status,
		        moderation_token, discord_message_id, discord_channel_id,
		        (SELECT COUNT(*) FROM support_ticket_messages m WHERE m.ticket_id = support_tickets.id AND m.author_type = 'admin' AND m.read_by_user = FALSE) AS unread_admin_count,
		        resolved_at, created_at
		 FROM support_tickets
		 WHERE owner_discord_id = $1
		 ORDER BY created_at DESC
		 LIMIT 50`,
		ownerDiscordID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tickets")
		return
	}
	defer rows.Close()

	tickets := make([]models.Ticket, 0)
	for rows.Next() {
		ticket, err := scanSupportTicket(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan tickets")
			return
		}
		tickets = append(tickets, ticket)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tickets": tickets})
}

func (h *SupportHandler) Messages(w http.ResponseWriter, r *http.Request) {
	ticketID, ok := parseSupportMessagesIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	ownerDiscordID := currentDiscordIDFromCookie(r)
	if ownerDiscordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
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
	if ticket.OwnerDiscordID != ownerDiscordID {
		writeError(w, http.StatusForbidden, "ticket access denied")
		return
	}

	if r.Method == http.MethodPost {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			writeError(w, http.StatusBadRequest, "message required")
			return
		}
		if len([]rune(message)) > 2000 {
			writeError(w, http.StatusBadRequest, "message is too long")
			return
		}

		_, err = h.db.ExecContext(
			ctx,
			`INSERT INTO support_ticket_messages (ticket_id, author_type, author_name, author_discord_id, message, read_by_user, created_at)
			 VALUES ($1, 'user', $2, $3, $4, TRUE, $5)`,
			ticket.ID,
			ticket.Name,
			ownerDiscordID,
			message,
			time.Now().UTC(),
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save message")
			return
		}
		if err := h.sendDiscordTicketChatMessage(ticket, message); err != nil {
			writeError(w, http.StatusBadGateway, "failed to send message to discord")
			return
		}
	}

	messages, err := h.loadTicketMessages(ctx, ticket.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load messages")
		return
	}
	_, _ = h.db.ExecContext(ctx, `UPDATE support_ticket_messages SET read_by_user = TRUE WHERE ticket_id = $1 AND author_type = 'admin'`, ticket.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticket, "messages": messages})
}

func (h *SupportHandler) Notifications(w http.ResponseWriter, r *http.Request) {
	ownerDiscordID := currentDiscordIDFromCookie(r)
	if ownerDiscordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": h.vapidPublicKey != "" && h.vapidPrivateKey != "",
			"publicKey":  h.vapidPublicKey,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if r.Method == http.MethodDelete {
		endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
		if endpoint == "" {
			_, _ = h.db.ExecContext(ctx, `DELETE FROM support_push_subscriptions WHERE discord_id = $1`, ownerDiscordID)
		} else {
			_, _ = h.db.ExecContext(ctx, `DELETE FROM support_push_subscriptions WHERE discord_id = $1 AND endpoint = $2`, ownerDiscordID, endpoint)
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256DH string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	payload.Endpoint = strings.TrimSpace(payload.Endpoint)
	payload.Keys.P256DH = strings.TrimSpace(payload.Keys.P256DH)
	payload.Keys.Auth = strings.TrimSpace(payload.Keys.Auth)
	if payload.Endpoint == "" || payload.Keys.P256DH == "" || payload.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "subscription keys required")
		return
	}

	_, err := h.db.ExecContext(
		ctx,
		`INSERT INTO support_push_subscriptions (discord_id, endpoint, p256dh, auth, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 ON CONFLICT (discord_id, endpoint) DO UPDATE SET
		   p256dh = EXCLUDED.p256dh,
		   auth = EXCLUDED.auth,
		   updated_at = EXCLUDED.updated_at`,
		ownerDiscordID,
		payload.Endpoint,
		payload.Keys.P256DH,
		payload.Keys.Auth,
		time.Now().UTC(),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SupportHandler) sendDiscordTicketChatMessage(ticket models.Ticket, message string) error {
	if h.webhookURL == "" {
		return nil
	}
	payload := map[string]any{
		"content":          fmt.Sprintf("Ответ пользователя по тикету #%d (%s):\n%s", ticket.ID, ticket.Subject, trimForDiscord(message)),
		"allowed_mentions": map[string]any{"parse": []string{}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, h.webhookURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	return err
}

func (h *SupportHandler) loadTicketMessages(ctx context.Context, ticketID int64) ([]models.TicketMessage, error) {
	rows, err := h.db.QueryContext(
		ctx,
		`SELECT id, ticket_id, author_type, author_name, author_discord_id, author_discord_status, message, read_by_user, created_at
		 FROM support_ticket_messages
		 WHERE ticket_id = $1
		 ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]models.TicketMessage, 0)
	for rows.Next() {
		var message models.TicketMessage
		if err := rows.Scan(
			&message.ID,
			&message.TicketID,
			&message.AuthorType,
			&message.AuthorName,
			&message.AuthorDiscordID,
			&message.AuthorDiscordStatus,
			&message.Message,
			&message.ReadByUser,
			&message.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
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

func parseSupportMessagesIDFromPath(path string) (int64, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "support" || parts[2] != "tickets" || parts[4] != "messages" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[3], 10, 64)
	return id, err == nil && id > 0
}

func currentDiscordIDFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("discord_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func scanSupportTicket(scanner sqlScanner) (models.Ticket, error) {
	var ticket models.Ticket
	var resolvedAt sql.NullTime
	err := scanner.Scan(
		&ticket.ID,
		&ticket.Name,
		&ticket.Email,
		&ticket.DiscordNick,
		&ticket.OwnerDiscordID,
		&ticket.Subject,
		&ticket.Category,
		&ticket.Message,
		&ticket.Status,
		&ticket.ModerationToken,
		&ticket.DiscordMessageID,
		&ticket.DiscordChannelID,
		&ticket.UnreadAdminCount,
		&resolvedAt,
		&ticket.CreatedAt,
	)
	if resolvedAt.Valid {
		ticket.ResolvedAt = &resolvedAt.Time
	}
	return ticket, err
}

func (h *SupportHandler) NotifyTicketReply(ctx context.Context, ticketID int64, authorName, message string) {
	if h.vapidPublicKey == "" || h.vapidPrivateKey == "" {
		return
	}
	ticket, err := h.loadTicket(ctx, ticketID)
	if err != nil || strings.TrimSpace(ticket.OwnerDiscordID) == "" {
		return
	}
	h.sendTicketReplyPush(ctx, ticket, authorName, message)
}

func (h *SupportHandler) sendTicketReplyPush(ctx context.Context, ticket models.Ticket, authorName, message string) {
	rows, err := h.db.QueryContext(ctx, `SELECT endpoint, p256dh, auth FROM support_push_subscriptions WHERE discord_id = $1`, ticket.OwnerDiscordID)
	if err != nil {
		return
	}
	defer rows.Close()

	subject := h.pushSubject
	if subject == "" {
		subject = "mailto:support@amyworld.ru"
	}
	payload, _ := json.Marshal(map[string]any{
		"title":    "Ответ поддержки",
		"body":     fmt.Sprintf("%s: %s", safeValue(authorName), trimForDiscord(message)),
		"url":      fmt.Sprintf("/support?ticket=%d", ticket.ID),
		"ticketId": ticket.ID,
	})

	for rows.Next() {
		var endpoint, p256dh, auth string
		if err := rows.Scan(&endpoint, &p256dh, &auth); err != nil {
			continue
		}
		subscription := &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				P256dh: p256dh,
				Auth:   auth,
			},
		}
		resp, err := webpush.SendNotification(payload, subscription, &webpush.Options{
			Subscriber:      subject,
			VAPIDPublicKey:  h.vapidPublicKey,
			VAPIDPrivateKey: h.vapidPrivateKey,
			TTL:             60,
		})
		if resp != nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
				_, _ = h.db.ExecContext(ctx, `DELETE FROM support_push_subscriptions WHERE endpoint = $1`, endpoint)
			}
		}
		_ = err
	}
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
