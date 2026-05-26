package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	storageDir      string
}

type ticketRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	DiscordNick string `json:"discordNick"`
	Subject     string `json:"subject"`
	Category    string `json:"category"`
	Message     string `json:"message"`
}

func NewSupportHandler(db *sql.DB, webhookURL, frontendURL, vapidPublicKey, vapidPrivateKey, pushSubject, storageDir string) *SupportHandler {
	storageDir = strings.TrimSpace(storageDir)
	if storageDir == "" {
		storageDir = "data/support"
	}
	return &SupportHandler{
		db:              db,
		webhookURL:      webhookURL,
		frontendURL:     frontendURL,
		vapidPublicKey:  strings.TrimSpace(vapidPublicKey),
		vapidPrivateKey: strings.TrimSpace(vapidPrivateKey),
		pushSubject:     strings.TrimSpace(pushSubject),
		storageDir:      storageDir,
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

	if payload.DiscordNick == "" || payload.Subject == "" || payload.Message == "" {
		writeError(w, http.StatusBadRequest, "discordNick, subject, message required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ownerDiscordID := currentDiscordIDFromCookie(r)
	discordID, displayName, err := h.resolveDiscordNick(ctx, payload.DiscordNick, ownerDiscordID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "discord nick is not found on server")
		return
	}
	if ownerDiscordID == "" {
		ownerDiscordID = discordID
	}
	if payload.Name == "" {
		payload.Name = displayName
	}
	payload.DiscordNick = displayName

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

	err = h.db.QueryRowContext(
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
	_ = h.writeTicketHistoryHTML(ctx, ticket)

	if messageID, channelID, err := h.sendDiscordWebhook(ticket); err == nil && messageID != "" {
		ticket.DiscordMessageID = messageID
		ticket.DiscordChannelID = channelID
		_, _ = h.db.ExecContext(ctx, `UPDATE support_tickets SET discord_message_id = $1, discord_channel_id = $2 WHERE id = $3`, messageID, ticket.DiscordChannelID, ticket.ID)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "ticket": ticket})
}

func (h *SupportHandler) Moderate(w http.ResponseWriter, r *http.Request) {
	if attachmentID, ok := parseSupportAttachmentIDFromPath(r.URL.Path); ok {
		h.Attachment(w, r, attachmentID)
		return
	}
	if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/notifications") {
		h.Notifications(w, r)
		return
	}
	if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/messages") {
		h.Messages(w, r)
		return
	}

	ticketID, ok := parseSupportModerationIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action != "resolve" && action != "reconsider" && action != "archive" && action != "unarchive" && action != "delete" && action != "unarchive_prompt" && action != "reply_prompt" && action != "reply" {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}
	if r.Method != http.MethodGet && !(r.Method == http.MethodPost && action == "reply") {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	if action == "reply_prompt" {
		h.writeTicketReplyHTML(w, ticket, "")
		return
	}
	if action == "reply" {
		if err := r.ParseForm(); err != nil {
			h.writeTicketReplyHTML(w, ticket, "Не удалось прочитать форму ответа.")
			return
		}
		message := strings.TrimSpace(r.FormValue("message"))
		if message == "" {
			h.writeTicketReplyHTML(w, ticket, "Введите текст ответа.")
			return
		}
		if len([]rune(message)) > 2000 {
			h.writeTicketReplyHTML(w, ticket, "Ответ слишком длинный, максимум 2000 символов.")
			return
		}
		if err := h.saveAdminTicketReply(ctx, ticket, "Техподдержка", message, ""); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save reply")
			return
		}
		h.writeTicketReplySentHTML(w, ticket)
		return
	}
	if action == "unarchive_prompt" {
		h.writeTicketDeleteConfirmHTML(w, ticket)
		return
	}
	if action == "delete" {
		if err := h.deleteTicket(ctx, ticket); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete ticket")
			return
		}
		h.writeTicketModerationHTML(w, ticket, action)
		return
	}

	now := time.Now().UTC()
	nextStatus := "resolved"
	var resolvedAt any = now
	var archivedAt any = nil
	if action == "reconsider" {
		nextStatus = "open"
		resolvedAt = nil
		archivedAt = nil
		ticket.ResolvedAt = nil
		ticket.ArchivedAt = nil
	} else if action == "archive" {
		nextStatus = "archived"
		if ticket.ResolvedAt == nil {
			ticket.ResolvedAt = &now
		}
		resolvedAt = ticket.ResolvedAt
		archivedAt = now
		ticket.ArchivedAt = &now
	} else if action == "unarchive" {
		nextStatus = "resolved"
		if ticket.ResolvedAt == nil {
			ticket.ResolvedAt = &now
		}
		resolvedAt = ticket.ResolvedAt
		archivedAt = nil
		ticket.ArchivedAt = nil
	} else {
		ticket.ResolvedAt = &now
	}

	_, err = h.db.ExecContext(
		ctx,
		`UPDATE support_tickets SET status = $1, resolved_at = $2, archived_at = $3 WHERE id = $4`,
		nextStatus,
		resolvedAt,
		archivedAt,
		ticket.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update ticket")
		return
	}

	ticket.Status = nextStatus
	_ = h.writeTicketHistoryHTML(ctx, ticket)
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
	if normalizedTicketStatus(ticket.Status) == "archived" {
		statusText = "Архив"
	}

	embed := map[string]any{
		"title":       ticket.Subject,
		"description": "Статус тикета: " + statusText + "\nОтветить пользователю можно кнопкой ниже.",
		"fields": []map[string]string{
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
		        resolved_at, archived_at, created_at
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
		        resolved_at, archived_at, created_at
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
		message, files, err := h.readMessagePayload(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if message == "" && len(files) == 0 {
			writeError(w, http.StatusBadRequest, "message or image required")
			return
		}
		if len([]rune(message)) > 2000 {
			writeError(w, http.StatusBadRequest, "message is too long")
			return
		}

		var messageID int64
		err = h.db.QueryRowContext(
			ctx,
			`INSERT INTO support_ticket_messages (ticket_id, author_type, author_name, author_discord_id, message, read_by_user, created_at)
			 VALUES ($1, 'user', $2, $3, $4, TRUE, $5)
			 RETURNING id`,
			ticket.ID,
			ticket.Name,
			ownerDiscordID,
			message,
			time.Now().UTC(),
		).Scan(&messageID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save message")
			return
		}
		for _, file := range files {
			if err := h.saveTicketAttachment(ctx, ticket.ID, messageID, file); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		_ = h.writeTicketHistoryHTML(ctx, ticket)
		discordMessageID, err := h.sendDiscordTicketChatMessage(ticket, message, files)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to send message to discord")
			return
		}
		if discordMessageID != "" {
			_, _ = h.db.ExecContext(ctx, `UPDATE support_ticket_messages SET discord_message_id = $1 WHERE id = $2`, discordMessageID, messageID)
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

func (h *SupportHandler) sendDiscordTicketChatMessage(ticket models.Ticket, message string, files []supportUpload) (string, error) {
	if h.webhookURL == "" {
		return "", nil
	}
	discordText := strings.TrimSpace(message)
	if discordText == "" && len(files) > 0 {
		discordText = fmt.Sprintf("[изображений: %d]", len(files))
	}
	payload := map[string]any{
		"content":          fmt.Sprintf("Ответ пользователя по тикету #%d (%s):\n%s", ticket.ID, ticket.Subject, trimForDiscord(discordText)),
		"allowed_mentions": map[string]any{"parse": []string{}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	requestURL, err := webhookURLWithWait(h.webhookURL)
	if err != nil {
		return "", err
	}

	var body io.Reader
	contentType := "application/json"
	if len(files) == 0 {
		body = bytes.NewReader(raw)
	} else {
		var form bytes.Buffer
		writer := multipart.NewWriter(&form)
		payloadField, err := writer.CreateFormField("payload_json")
		if err != nil {
			return "", err
		}
		if _, err := payloadField.Write(raw); err != nil {
			return "", err
		}
		for index, file := range files {
			part, err := writer.CreateFormFile(fmt.Sprintf("files[%d]", index), file.Name)
			if err != nil {
				return "", err
			}
			if _, err := part.Write(file.Bytes); err != nil {
				return "", err
			}
		}
		if err := writer.Close(); err != nil {
			return "", err
		}
		body = &form
		contentType = writer.FormDataContentType()
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("discord webhook error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}
	var webhookMessage discordWebhookMessage
	if err := json.NewDecoder(resp.Body).Decode(&webhookMessage); err != nil {
		return "", nil
	}
	return strings.TrimSpace(webhookMessage.ID), nil
}

func (h *SupportHandler) saveAdminTicketReply(ctx context.Context, ticket models.Ticket, authorName, message, discordMessageID string) error {
	authorName = strings.TrimSpace(authorName)
	if authorName == "" {
		authorName = "Техподдержка"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("empty reply")
	}
	_, err := h.db.ExecContext(
		ctx,
		`INSERT INTO support_ticket_messages
		 (ticket_id, author_type, author_name, author_discord_id, author_discord_status, message, discord_message_id, read_by_user, created_at)
		 VALUES ($1, 'admin', $2, '', 'unknown', $3, $4, FALSE, $5)
		 ON CONFLICT (discord_message_id) WHERE discord_message_id <> '' DO NOTHING`,
		ticket.ID,
		authorName,
		message,
		strings.TrimSpace(discordMessageID),
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	_ = h.writeTicketHistoryHTML(ctx, ticket)
	if h.vapidPublicKey != "" && h.vapidPrivateKey != "" {
		h.sendTicketReplyPush(ctx, ticket, authorName, message)
	}
	return nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return messages, nil
	}
	attachments, err := h.loadTicketAttachments(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	byMessage := make(map[int64][]models.TicketAttachment)
	for _, attachment := range attachments {
		byMessage[attachment.MessageID] = append(byMessage[attachment.MessageID], attachment)
	}
	for i := range messages {
		messages[i].Attachments = byMessage[messages[i].ID]
	}
	return messages, nil
}

func (h *SupportHandler) ticketDiscordComponents(ticket models.Ticket) []any {
	status := normalizedTicketStatus(ticket.Status)
	if status == "resolved" {
		return []any{map[string]any{"type": 1, "components": []any{
			map[string]any{"type": 2, "style": 5, "label": "Ответить пользователю", "url": h.ticketModerationURL(ticket.ID, "reply_prompt", ticket.ModerationToken)},
			map[string]any{"type": 2, "style": 5, "label": "На пересмотр", "url": h.ticketModerationURL(ticket.ID, "reconsider", ticket.ModerationToken)},
			map[string]any{"type": 2, "style": 5, "label": "Архивировать", "url": h.ticketModerationURL(ticket.ID, "archive", ticket.ModerationToken)},
			map[string]any{"type": 2, "style": 5, "label": "Удалить сразу", "url": h.ticketModerationURL(ticket.ID, "delete", ticket.ModerationToken)},
		}}}
	}
	if status == "archived" {
		return []any{map[string]any{"type": 1, "components": []any{
			map[string]any{"type": 2, "style": 5, "label": "Ответить пользователю", "url": h.ticketModerationURL(ticket.ID, "reply_prompt", ticket.ModerationToken)},
			map[string]any{"type": 2, "style": 5, "label": "Разархивировать/удалить сразу", "url": h.ticketModerationURL(ticket.ID, "unarchive_prompt", ticket.ModerationToken)},
		}}}
	}
	return []any{map[string]any{"type": 1, "components": []any{
		map[string]any{
			"type":  2,
			"style": 5,
			"label": "Ответить пользователю",
			"url":   h.ticketModerationURL(ticket.ID, "reply_prompt", ticket.ModerationToken),
		},
		map[string]any{
			"type":  2,
			"style": 5,
			"label": "Решён",
			"url":   h.ticketModerationURL(ticket.ID, "resolve", ticket.ModerationToken),
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
	switch status {
	case "resolved", "archived":
		return status
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

func parseSupportAttachmentIDFromPath(path string) (int64, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "support" || parts[2] != "tickets" || parts[4] != "attachments" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[5], 10, 64)
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
	var archivedAt sql.NullTime
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
		&archivedAt,
		&ticket.CreatedAt,
	)
	if resolvedAt.Valid {
		ticket.ResolvedAt = &resolvedAt.Time
	}
	if archivedAt.Valid {
		ticket.ArchivedAt = &archivedAt.Time
	}
	return ticket, err
}

type supportUpload struct {
	Name     string
	MimeType string
	Bytes    []byte
}

const maxSupportImageBytes = 10 * 1024 * 1024

var supportTicketPrefix = regexp.MustCompile(`(?i)^\s*(?:#|ticket\s*#?|тикет\s*#?)(\d+)\s*[:\-–]?\s*(.*)$`)

func (h *SupportHandler) resolveDiscordNick(ctx context.Context, nick, ownerDiscordID string) (string, string, error) {
	nick = strings.TrimSpace(nick)
	if nick == "" {
		return "", "", fmt.Errorf("empty nick")
	}

	var discordID, username, globalName, memberNick string
	if ownerDiscordID != "" {
		err := h.db.QueryRowContext(
			ctx,
			`SELECT discord_id, username, global_name, nick
			 FROM discord_member_states
			 WHERE discord_id = $1
			   AND (LOWER(username) = LOWER($2) OR LOWER(global_name) = LOWER($2) OR LOWER(nick) = LOWER($2))
			 LIMIT 1`,
			ownerDiscordID,
			nick,
		).Scan(&discordID, &username, &globalName, &memberNick)
		if err == nil {
			return discordID, bestDiscordDisplayName(memberNick, globalName, username, nick), nil
		}
		return "", "", err
	}

	err := h.db.QueryRowContext(
		ctx,
		`SELECT discord_id, username, global_name, nick
		 FROM discord_member_states
		 WHERE LOWER(username) = LOWER($1) OR LOWER(global_name) = LOWER($1) OR LOWER(nick) = LOWER($1)
		 ORDER BY synced_at DESC
		 LIMIT 1`,
		nick,
	).Scan(&discordID, &username, &globalName, &memberNick)
	if err != nil {
		return "", "", err
	}
	return discordID, bestDiscordDisplayName(memberNick, globalName, username, nick), nil
}

func bestDiscordDisplayName(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "Discord user"
}

func (h *SupportHandler) readMessagePayload(r *http.Request) (string, []supportUpload, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxSupportImageBytes + 1024*1024); err != nil {
			return "", nil, fmt.Errorf("invalid multipart form")
		}
		message := strings.TrimSpace(r.FormValue("message"))
		files := make([]supportUpload, 0)
		for _, headers := range r.MultipartForm.File {
			for _, header := range headers {
				if header.Size > maxSupportImageBytes {
					return "", nil, fmt.Errorf("image must be 10mb or smaller")
				}
				file, err := header.Open()
				if err != nil {
					return "", nil, fmt.Errorf("failed to read image")
				}
				raw, err := io.ReadAll(io.LimitReader(file, maxSupportImageBytes+1))
				_ = file.Close()
				if err != nil {
					return "", nil, fmt.Errorf("failed to read image")
				}
				if int64(len(raw)) > maxSupportImageBytes {
					return "", nil, fmt.Errorf("image must be 10mb or smaller")
				}
				mimeType := http.DetectContentType(raw)
				if !strings.HasPrefix(mimeType, "image/") {
					return "", nil, fmt.Errorf("only images are allowed")
				}
				files = append(files, supportUpload{Name: sanitizeSupportFileName(header.Filename, mimeType), MimeType: mimeType, Bytes: raw})
			}
		}
		return message, files, nil
	}

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return "", nil, fmt.Errorf("invalid json")
	}
	return strings.TrimSpace(payload.Message), nil, nil
}

func sanitizeSupportFileName(name, mimeType string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if name == "" || name == "." {
		exts, _ := mime.ExtensionsByType(mimeType)
		ext := ".png"
		if len(exts) > 0 {
			ext = exts[0]
		}
		name = "image" + ext
	}
	return name
}

func (h *SupportHandler) saveTicketAttachment(ctx context.Context, ticketID, messageID int64, upload supportUpload) error {
	dir := filepath.Join(h.storageDir, "tickets", strconv.FormatInt(ticketID, 10), "attachments")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to prepare attachment storage")
	}
	fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), upload.Name)
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, upload.Bytes, 0640); err != nil {
		return fmt.Errorf("failed to save image")
	}
	_, err := h.db.ExecContext(
		ctx,
		`INSERT INTO support_ticket_attachments (ticket_id, message_id, file_name, mime_type, size_bytes, storage_path, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		ticketID,
		messageID,
		upload.Name,
		upload.MimeType,
		len(upload.Bytes),
		path,
		time.Now().UTC(),
	)
	return err
}

func (h *SupportHandler) loadTicketAttachments(ctx context.Context, ticketID int64) ([]models.TicketAttachment, error) {
	rows, err := h.db.QueryContext(
		ctx,
		`SELECT id, ticket_id, message_id, file_name, mime_type, size_bytes, storage_path, created_at
		 FROM support_ticket_attachments
		 WHERE ticket_id = $1
		 ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]models.TicketAttachment, 0)
	for rows.Next() {
		var attachment models.TicketAttachment
		if err := rows.Scan(&attachment.ID, &attachment.TicketID, &attachment.MessageID, &attachment.FileName, &attachment.MimeType, &attachment.SizeBytes, &attachment.StoragePath, &attachment.CreatedAt); err != nil {
			return nil, err
		}
		attachment.URL = fmt.Sprintf("/support/tickets/%d/attachments/%d", attachment.TicketID, attachment.ID)
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (h *SupportHandler) Attachment(w http.ResponseWriter, r *http.Request, attachmentID int64) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ownerDiscordID := currentDiscordIDFromCookie(r)
	if ownerDiscordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var ticketID int64
	var mimeType, path string
	err := h.db.QueryRowContext(
		ctx,
		`SELECT a.ticket_id, a.mime_type, a.storage_path
		 FROM support_ticket_attachments a
		 JOIN support_tickets t ON t.id = a.ticket_id
		 WHERE a.id = $1 AND t.owner_discord_id = $2`,
		attachmentID,
		ownerDiscordID,
	).Scan(&ticketID, &mimeType, &path)
	_ = ticketID
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load attachment")
		return
	}
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, path)
}

func (h *SupportHandler) writeTicketHistoryHTML(ctx context.Context, ticket models.Ticket) error {
	messages, err := h.loadTicketMessages(ctx, ticket.ID)
	if err != nil {
		return err
	}
	dir := filepath.Join(h.storageDir, "tickets", strconv.FormatInt(ticket.ID, 10))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Support ticket ")
	b.WriteString(strconv.FormatInt(ticket.ID, 10))
	b.WriteString("</title><style>body{font-family:Arial,sans-serif;background:#f4f4f4;color:#111}.message{background:#fff;margin:12px 0;padding:12px;border-radius:8px}.meta{color:#666;font-size:12px}.mine{border-left:4px solid #f7c948}.admin{border-left:4px solid #5865f2}img{max-width:420px;border-radius:6px;display:block;margin-top:8px}</style></head><body>")
	b.WriteString("<h1>Ticket #")
	b.WriteString(strconv.FormatInt(ticket.ID, 10))
	b.WriteString(": ")
	b.WriteString(html.EscapeString(ticket.Subject))
	b.WriteString("</h1>")
	for _, message := range messages {
		className := "message " + html.EscapeString(message.AuthorType)
		if message.AuthorType == "user" {
			className = "message mine"
		}
		b.WriteString("<div class=\"" + className + "\"><div class=\"meta\">")
		b.WriteString(html.EscapeString(message.AuthorName))
		b.WriteString(" · ")
		b.WriteString(html.EscapeString(message.CreatedAt.Format(time.RFC3339)))
		b.WriteString("</div><p>")
		b.WriteString(html.EscapeString(message.Message))
		b.WriteString("</p>")
		for _, attachment := range message.Attachments {
			b.WriteString("<a href=\"attachments/")
			b.WriteString(html.EscapeString(filepath.Base(attachment.StoragePath)))
			b.WriteString("\">")
			b.WriteString(html.EscapeString(attachment.FileName))
			b.WriteString("</a>")
		}
		b.WriteString("</div>")
	}
	b.WriteString("</body></html>")
	return os.WriteFile(filepath.Join(dir, "history.html"), []byte(b.String()), 0640)
}

func (h *SupportHandler) deleteTicket(ctx context.Context, ticket models.Ticket) error {
	_, err := h.db.ExecContext(ctx, `DELETE FROM support_tickets WHERE id = $1`, ticket.ID)
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(h.storageDir, "tickets", strconv.FormatInt(ticket.ID, 10)))
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

func (h *SupportHandler) writeTicketDeleteConfirmHTML(w http.ResponseWriter, ticket models.Ticket) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	htmlBody := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <title>Подтверждение</title>
    <style>
      body{font-family:system-ui;background:#0f1118;color:#fff;margin:0;padding:40px}
      .card{max-width:760px;margin:0 auto;padding:24px;border-radius:14px;background:#171a26;border:1px solid rgba(255,255,255,.12)}
      a{display:inline-block;margin-right:10px;margin-top:12px;padding:10px 14px;border-radius:10px;background:#f7c948;color:#0b0b0f;text-decoration:none;font-weight:700}
      a.secondary{background:rgba(255,255,255,.12);color:#fff}
    </style>
  </head>
  <body>
    <div class="card">
      <h1>Удалить архивный тикет #%d?</h1>
      <p>Будут удалены история переписки HTML и вложения с носителя.</p>
      <a href="%s">Да, уверен</a>
      <a class="secondary" href="%s">Назад</a>
    </div>
  </body>
</html>`, ticket.ID, h.ticketModerationURL(ticket.ID, "delete", ticket.ModerationToken), h.ticketModerationURL(ticket.ID, "archive", ticket.ModerationToken))
	_, _ = w.Write([]byte(htmlBody))
}

func (h *SupportHandler) writeTicketReplyHTML(w http.ResponseWriter, ticket models.Ticket, errorText string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errorBlock := ""
	if strings.TrimSpace(errorText) != "" {
		errorBlock = `<p class="error">` + html.EscapeString(errorText) + `</p>`
	}
	htmlBody := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <title>Ответить пользователю</title>
    <style>
      body{font-family:system-ui;background:#0f1118;color:#fff;margin:0;padding:40px}
      .card{max-width:760px;margin:0 auto;padding:24px;border-radius:14px;background:#171a26;border:1px solid rgba(255,255,255,.12)}
      .muted{color:#b4b6c7}.error{color:#ff8f8f}
      textarea{box-sizing:border-box;width:100%%;min-height:170px;resize:vertical;border-radius:12px;border:1px solid rgba(255,255,255,.16);background:#10131d;color:#fff;padding:14px;font:inherit}
      button,a{display:inline-flex;margin-right:10px;margin-top:12px;padding:10px 14px;border-radius:10px;border:0;background:#f7c948;color:#0b0b0f;text-decoration:none;font-weight:700;cursor:pointer}
      a.secondary{background:rgba(255,255,255,.12);color:#fff}
    </style>
  </head>
  <body>
    <div class="card">
      <h1>Ответ пользователю по тикету #%d</h1>
      <p class="muted">%s · %s</p>
      %s
      <form method="post" action="%s">
        <textarea name="message" maxlength="2000" required placeholder="Напишите ответ пользователю..."></textarea>
        <div>
          <button type="submit">Отправить пользователю</button>
          <a class="secondary" href="%s">Назад</a>
        </div>
      </form>
    </div>
  </body>
</html>`,
		ticket.ID,
		html.EscapeString(ticket.Subject),
		html.EscapeString(ticket.DiscordNick),
		errorBlock,
		h.ticketModerationURL(ticket.ID, "reply", ticket.ModerationToken),
		h.ticketModerationURL(ticket.ID, "reply_prompt", ticket.ModerationToken),
	)
	_, _ = w.Write([]byte(htmlBody))
}

func (h *SupportHandler) writeTicketReplySentHTML(w http.ResponseWriter, ticket models.Ticket) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	htmlBody := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <title>Ответ отправлен</title>
    <style>
      body{font-family:system-ui;background:#0f1118;color:#fff;margin:0;padding:40px}
      .card{max-width:760px;margin:0 auto;padding:24px;border-radius:14px;background:#171a26;border:1px solid rgba(255,255,255,.12)}
      .muted{color:#b4b6c7}
      a{display:inline-flex;margin-right:10px;margin-top:12px;padding:10px 14px;border-radius:10px;background:#f7c948;color:#0b0b0f;text-decoration:none;font-weight:700}
    </style>
  </head>
  <body>
    <div class="card">
      <h1>Ответ отправлен</h1>
      <p class="muted">Сообщение добавлено в чат тикета #%d. Если у пользователя включены уведомления, ему придёт push.</p>
      <a href="%s">Написать ещё</a>
    </div>
  </body>
</html>`, ticket.ID, h.ticketModerationURL(ticket.ID, "reply_prompt", ticket.ModerationToken))
	_, _ = w.Write([]byte(htmlBody))
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
