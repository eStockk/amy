package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type rpApplicationDoc struct {
	ID               string
	DiscordID        string
	Nickname         string
	Source           string
	RPName           string
	BirthDate        string
	Race             string
	Gender           string
	HeightCm         int
	Skills           string
	Plan             string
	Biography        string
	PrisonReason     string
	SkinURL          string
	Status           string
	ModerationToken  string
	DiscordMessageID string
	ModeratedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type rpApplicationRequest struct {
	Nickname     string `json:"nickname"`
	Source       string `json:"source"`
	RPName       string `json:"rpName"`
	BirthDate    string `json:"birthDate"`
	Race         string `json:"race"`
	Gender       string `json:"gender"`
	HeightCm     int    `json:"heightCm"`
	Skills       string `json:"skills"`
	Plan         string `json:"plan"`
	Biography    string `json:"biography"`
	PrisonReason string `json:"prisonReason"`
	SkinURL      string `json:"skinUrl"`
}

type discordWebhookMessage struct {
	ID string `json:"id"`
}

type sqlScanner interface {
	Scan(dest ...any) error
}

var imageExtRe = regexp.MustCompile(`(?i)\.(png|jpe?g|webp)$`)

func (h *DiscordAuthHandler) SubmitRPApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var payload rpApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	normalizeRPRequest(&payload)
	if validationError := validateRPRequest(payload); validationError != "" {
		writeError(w, http.StatusBadRequest, validationError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	if hasAccepted, err := h.hasAcceptedApplication(ctx, user.DiscordID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check current applications")
		return
	} else if hasAccepted {
		writeError(w, http.StatusConflict, "application is already accepted")
		return
	}

	if hasLocked, err := h.hasLockedApplication(ctx, user.DiscordID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check current applications")
		return
	} else if hasLocked {
		writeError(w, http.StatusConflict, "pending rp application already exists")
		return
	}

	now := time.Now().UTC()
	doc := rpApplicationDoc{
		ID:              randomHex(12),
		DiscordID:       user.DiscordID,
		Nickname:        payload.Nickname,
		Source:          payload.Source,
		RPName:          payload.RPName,
		BirthDate:       payload.BirthDate,
		Race:            payload.Race,
		Gender:          payload.Gender,
		HeightCm:        payload.HeightCm,
		Skills:          payload.Skills,
		Plan:            payload.Plan,
		Biography:       payload.Biography,
		PrisonReason:    payload.PrisonReason,
		SkinURL:         payload.SkinURL,
		Status:          "pending",
		ModerationToken: randomHex(20),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err = h.db.ExecContext(
		ctx,
		`INSERT INTO rp_applications
		 (id, discord_id, nickname, source, rp_name, birth_date, race, gender, height_cm,
		  skills, plan, biography, prison_reason, skin_url, status, moderation_token, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		doc.ID,
		doc.DiscordID,
		doc.Nickname,
		doc.Source,
		doc.RPName,
		doc.BirthDate,
		doc.Race,
		doc.Gender,
		doc.HeightCm,
		doc.Skills,
		doc.Plan,
		doc.Biography,
		doc.PrisonReason,
		doc.SkinURL,
		doc.Status,
		doc.ModerationToken,
		doc.CreatedAt,
		doc.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rp application")
		return
	}

	_, _ = h.db.ExecContext(ctx, `UPDATE discord_users SET acceptance_status = 'pending', updated_at = $1 WHERE discord_id = $2`, now, user.DiscordID)

	if h.rpWebhookURL != "" {
		messageID, webhookErr := h.sendRPApplicationWebhook(doc, user)
		if webhookErr != nil {
			_, _ = h.db.ExecContext(ctx, `DELETE FROM rp_applications WHERE id = $1`, doc.ID)
			writeError(w, http.StatusBadGateway, "failed to send rp ticket to discord")
			return
		}

		if messageID != "" {
			doc.DiscordMessageID = messageID
			_, _ = h.db.ExecContext(ctx, `UPDATE rp_applications SET discord_message_id = $1, updated_at = $2 WHERE id = $3`, messageID, time.Now().UTC(), doc.ID)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":        "ok",
		"applicationId": doc.ID,
	})
}

func (h *DiscordAuthHandler) ModerateRPApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		h.DeleteRPApplication(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	applicationID, ok := parseApplicationModerationIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	action := normalizeModerationAction(r.URL.Query().Get("action"))
	if action == "" {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}

	moderator, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "moderator auth required")
		return
	}
	if !h.isRPModerator(moderator.DiscordID) {
		writeError(w, http.StatusForbidden, "moderator access required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	current, err := h.loadApplicationByID(ctx, applicationID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "application not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load application")
		return
	}

	if token != current.ModerationToken {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	currentStatus := normalizedStatus(current.Status)
	nextStatus, allowed := nextStatusByAction(currentStatus, action)
	if !allowed {
		h.writeModerationHTML(w, *current, "already-processed")
		return
	}

	now := time.Now().UTC()
	newToken := current.ModerationToken
	var moderatedAt any = now
	if nextStatus == "pending" {
		newToken = randomHex(20)
		moderatedAt = nil
		current.ModeratedAt = nil
	} else {
		current.ModeratedAt = &now
	}

	_, err = h.db.ExecContext(
		ctx,
		`UPDATE rp_applications
		 SET status = $1, moderation_token = $2, moderated_at = $3, updated_at = $4
		 WHERE id = $5`,
		nextStatus,
		newToken,
		moderatedAt,
		now,
		applicationID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to moderate application")
		return
	}

	_, _ = h.db.ExecContext(ctx, `UPDATE discord_users SET acceptance_status = $1, updated_at = $2 WHERE discord_id = $3`, nextStatus, now, current.DiscordID)

	current.Status = nextStatus
	current.UpdatedAt = now
	current.ModerationToken = newToken

	if updateErr := h.updateRPApplicationDiscordMessage(*current); updateErr != nil {
		writeError(w, http.StatusBadGateway, "failed to update discord ticket")
		return
	}

	h.writeModerationHTML(w, *current, action)
}

func (h *DiscordAuthHandler) DeleteRPApplication(w http.ResponseWriter, r *http.Request) {
	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	applicationID, ok := parseApplicationDeleteIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	application, err := h.loadApplicationByID(ctx, applicationID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "application not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load application")
		return
	}

	if application.DiscordID != user.DiscordID {
		writeError(w, http.StatusForbidden, "you can delete only your own application")
		return
	}

	if normalizedStatus(application.Status) == "accepted" {
		writeError(w, http.StatusConflict, "accepted application cannot be deleted")
		return
	}

	if err := h.deleteRPApplicationDiscordMessage(application.DiscordMessageID); err != nil {
		writeError(w, http.StatusBadGateway, "failed to delete ticket message in discord")
		return
	}

	_, err = h.db.ExecContext(ctx, `DELETE FROM rp_applications WHERE id = $1 AND discord_id = $2`, applicationID, user.DiscordID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}

	_, _ = h.db.ExecContext(ctx, `DELETE FROM minecraft_verification_codes WHERE application_id = $1`, application.ID)
	_, _ = h.db.ExecContext(ctx, `UPDATE discord_users SET acceptance_status = 'pending', updated_at = $1 WHERE discord_id = $2 AND acceptance_status <> 'accepted'`, time.Now().UTC(), user.DiscordID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) latestApplicationSummary(ctx context.Context, discordID string) (*rpApplicationSummaryOut, error) {
	doc, err := h.loadLatestApplicationForDiscord(ctx, discordID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &rpApplicationSummaryOut{
		ID:           doc.ID,
		Status:       doc.Status,
		Nickname:     doc.Nickname,
		RPName:       doc.RPName,
		Race:         doc.Race,
		Gender:       doc.Gender,
		HeightCm:     doc.HeightCm,
		BirthDate:    doc.BirthDate,
		PrisonReason: doc.PrisonReason,
		CreatedAt:    &doc.CreatedAt,
		UpdatedAt:    &doc.UpdatedAt,
		ModeratedAt:  doc.ModeratedAt,
	}, nil
}

func (h *DiscordAuthHandler) loadApplicationByID(ctx context.Context, id string) (*rpApplicationDoc, error) {
	return scanRPApplication(h.db.QueryRowContext(ctx, rpApplicationSelectSQL+` WHERE id = $1`, id))
}

func (h *DiscordAuthHandler) loadLatestApplicationForDiscord(ctx context.Context, discordID string) (*rpApplicationDoc, error) {
	return scanRPApplication(h.db.QueryRowContext(ctx, rpApplicationSelectSQL+` WHERE discord_id = $1 ORDER BY created_at DESC LIMIT 1`, discordID))
}

const rpApplicationSelectSQL = `SELECT id, discord_id, nickname, source, rp_name, birth_date, race, gender, height_cm,
       skills, plan, biography, prison_reason, skin_url, status, moderation_token,
       discord_message_id, moderated_at, created_at, updated_at
FROM rp_applications`

func scanRPApplication(scanner sqlScanner) (*rpApplicationDoc, error) {
	var app rpApplicationDoc
	var moderatedAt sql.NullTime
	err := scanner.Scan(
		&app.ID,
		&app.DiscordID,
		&app.Nickname,
		&app.Source,
		&app.RPName,
		&app.BirthDate,
		&app.Race,
		&app.Gender,
		&app.HeightCm,
		&app.Skills,
		&app.Plan,
		&app.Biography,
		&app.PrisonReason,
		&app.SkinURL,
		&app.Status,
		&app.ModerationToken,
		&app.DiscordMessageID,
		&moderatedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if moderatedAt.Valid {
		app.ModeratedAt = &moderatedAt.Time
	}
	return &app, nil
}

func (h *DiscordAuthHandler) sendRPApplicationWebhook(doc rpApplicationDoc, user *discordUserDoc) (string, error) {
	if h.rpWebhookURL == "" {
		return "", nil
	}

	payload := h.buildRPApplicationDiscordPayload(doc, user)
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	requestURL, err := webhookURLWithWait(h.rpWebhookURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

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

func (h *DiscordAuthHandler) deleteRPApplicationDiscordMessage(messageID string) error {
	messageID = strings.TrimSpace(messageID)
	if h.rpWebhookURL == "" || messageID == "" {
		return nil
	}

	parsed, err := url.Parse(h.rpWebhookURL)
	if err != nil {
		return err
	}
	parsed.RawQuery = ""

	base := strings.TrimRight(parsed.String(), "/")
	deleteURL := base + "/messages/" + url.PathEscape(messageID)

	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord webhook delete error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}

	return nil
}

func webhookURLWithWait(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	query.Set("wait", "true")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (h *DiscordAuthHandler) moderationURL(applicationID, action, token string) string {
	base := strings.TrimRight(h.frontendURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/api/rp/applications/%s/moderate?action=%s&token=%s", base, applicationID, action, url.QueryEscape(token))
}

func (h *DiscordAuthHandler) hasLockedApplication(ctx context.Context, discordID string) (bool, error) {
	var count int
	err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rp_applications WHERE discord_id = $1 AND status IN ('pending', 'call')`, discordID).Scan(&count)
	return count > 0, err
}

func (h *DiscordAuthHandler) hasAcceptedApplication(ctx context.Context, discordID string) (bool, error) {
	var count int
	err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rp_applications WHERE discord_id = $1 AND status IN ('accepted', 'approved')`, discordID).Scan(&count)
	return count > 0, err
}

func normalizedStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved":
		return "accepted"
	case "rejected":
		return "canceled"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func normalizeModerationAction(raw string) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "approve":
		return "accept"
	case "reject":
		return "cancel"
	case "accept", "cancel", "call", "reconsider":
		return action
	default:
		return ""
	}
}

func nextStatusByAction(currentStatus, action string) (string, bool) {
	switch action {
	case "accept":
		if currentStatus != "pending" && currentStatus != "call" {
			return "", false
		}
		return "accepted", true
	case "cancel":
		if currentStatus != "pending" && currentStatus != "call" {
			return "", false
		}
		return "canceled", true
	case "call":
		if currentStatus != "pending" {
			return "", false
		}
		return "call", true
	case "reconsider":
		if currentStatus == "pending" {
			return "", false
		}
		return "pending", true
	default:
		return "", false
	}
}

func (h *DiscordAuthHandler) buildRPApplicationDiscordPayload(doc rpApplicationDoc, user *discordUserDoc) map[string]any {
	status := normalizedStatus(doc.Status)
	statusText := map[string]string{
		"pending":  "На рассмотрении",
		"call":     "Позвать на созвон",
		"accepted": "Принята",
		"canceled": "Отменена",
	}[status]
	if statusText == "" {
		statusText = doc.Status
	}

	discordAccount := doc.DiscordID
	if user != nil {
		discordAccount = user.Username + " (" + user.DiscordID + ")"
	}

	fields := []map[string]string{
		{"name": "Discord аккаунт", "value": safeValue(discordAccount)},
		{"name": "Ник в игре", "value": safeValue(doc.Nickname)},
		{"name": "Откуда узнал о сервере", "value": safeValue(doc.Source)},
		{"name": "Имя и фамилия", "value": safeValue(doc.RPName)},
		{"name": "Дата рождения", "value": safeValue(doc.BirthDate)},
		{"name": "Раса", "value": safeValue(doc.Race)},
		{"name": "Пол", "value": safeValue(doc.Gender)},
		{"name": "Рост", "value": fmt.Sprintf("%d см", doc.HeightCm)},
		{"name": "Ключевые навыки", "value": trimForDiscord(doc.Skills)},
		{"name": "План развития", "value": trimForDiscord(doc.Plan)},
		{"name": "Биография", "value": trimForDiscord(doc.Biography)},
		{"name": "Причина ссылки на тюремный остров", "value": trimForDiscord(doc.PrisonReason)},
		{"name": "Ссылка на скин", "value": safeValue(doc.SkinURL)},
	}

	if links := h.rpModerationLinks(doc); links != "" {
		fields = append(fields, map[string]string{"name": "Модерация", "value": links})
	}

	embed := map[string]any{
		"title":       "RP-заявка: " + doc.Nickname,
		"description": "Статус заявки: " + statusText,
		"color":       14901048,
		"fields":      fields,
	}

	content := "RP-тикет игрока " + doc.Nickname
	if links := h.rpModerationLinks(doc); links != "" {
		content += "\n" + links
	}

	payload := map[string]any{
		"content": content,
		"embeds":  []any{embed},
	}

	if components := h.rpDiscordComponents(doc); len(components) > 0 {
		payload["components"] = components
	}

	return payload
}

func (h *DiscordAuthHandler) rpModerationLinks(doc rpApplicationDoc) string {
	status := normalizedStatus(doc.Status)
	switch status {
	case "pending":
		return strings.Join([]string{
			"Принять: " + h.moderationURL(doc.ID, "accept", doc.ModerationToken),
			"Позвать на созвон: " + h.moderationURL(doc.ID, "call", doc.ModerationToken),
			"Отменить: " + h.moderationURL(doc.ID, "cancel", doc.ModerationToken),
		}, "\n")
	case "call":
		return strings.Join([]string{
			"Принять после созвона: " + h.moderationURL(doc.ID, "accept", doc.ModerationToken),
			"Отменить: " + h.moderationURL(doc.ID, "cancel", doc.ModerationToken),
			"Вернуть на рассмотрение: " + h.moderationURL(doc.ID, "reconsider", doc.ModerationToken),
		}, "\n")
	case "accepted", "canceled":
		return "Перерассмотр: " + h.moderationURL(doc.ID, "reconsider", doc.ModerationToken)
	default:
		return ""
	}
}

func (h *DiscordAuthHandler) rpDiscordComponents(doc rpApplicationDoc) []any {
	status := normalizedStatus(doc.Status)
	button := func(label, action string) map[string]any {
		return map[string]any{"type": 2, "style": 5, "label": label, "url": h.moderationURL(doc.ID, action, doc.ModerationToken)}
	}

	switch status {
	case "pending":
		return []any{map[string]any{"type": 1, "components": []any{
			button("Принять", "accept"),
			button("Позвать на созвон", "call"),
			button("Отменить", "cancel"),
		}}}
	case "call":
		return []any{map[string]any{"type": 1, "components": []any{
			button("Принять", "accept"),
			button("Отменить", "cancel"),
			button("На рассмотрение", "reconsider"),
		}}}
	case "accepted", "canceled":
		return []any{map[string]any{"type": 1, "components": []any{button("Перерассмотр", "reconsider")}}}
	default:
		return nil
	}
}

func (h *DiscordAuthHandler) updateRPApplicationDiscordMessage(app rpApplicationDoc) error {
	if h.rpWebhookURL == "" || strings.TrimSpace(app.DiscordMessageID) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	owner, _ := h.loadDiscordUser(ctx, app.DiscordID)
	payload := h.buildRPApplicationDiscordPayload(app, owner)

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	base, err := webhookBaseURL(h.rpWebhookURL)
	if err != nil {
		return err
	}

	editURL := base + "/messages/" + url.PathEscape(app.DiscordMessageID)
	req, err := http.NewRequest(http.MethodPatch, editURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord webhook edit error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}

	return nil
}

func webhookBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	parsed.RawQuery = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func parseApplicationModerationIDFromPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "rp" || parts[2] != "applications" || parts[4] != "moderate" {
		return "", false
	}
	id := strings.TrimSpace(parts[3])
	return id, id != ""
}

func parseApplicationDeleteIDFromPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "rp" || parts[2] != "applications" {
		return "", false
	}
	id := strings.TrimSpace(parts[3])
	return id, id != ""
}

func normalizeRPRequest(req *rpApplicationRequest) {
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Source = strings.TrimSpace(req.Source)
	req.RPName = strings.TrimSpace(req.RPName)
	req.BirthDate = strings.TrimSpace(req.BirthDate)
	req.Race = strings.TrimSpace(req.Race)
	req.Gender = strings.TrimSpace(req.Gender)
	req.Skills = strings.TrimSpace(req.Skills)
	req.Plan = strings.TrimSpace(req.Plan)
	req.Biography = strings.TrimSpace(req.Biography)
	req.PrisonReason = strings.TrimSpace(req.PrisonReason)
	req.SkinURL = strings.TrimSpace(req.SkinURL)
}

func validateRPRequest(payload rpApplicationRequest) string {
	if !minecraftNicknameRe.MatchString(payload.Nickname) {
		return "nickname must be 3-16 chars and contain only latin letters, digits or _"
	}
	if payload.BirthDate == "" || payload.Race == "" || payload.Gender == "" || payload.Skills == "" || payload.Plan == "" || payload.Biography == "" || payload.PrisonReason == "" || payload.SkinURL == "" {
		return "required fields are missing"
	}
	if payload.HeightCm < 120 || payload.HeightCm > 250 {
		return "heightCm must be between 120 and 250"
	}
	if len(payload.Source) > 200 || len(payload.RPName) > 120 || len(payload.Race) > 80 || len(payload.Gender) > 80 {
		return "one of the text fields is too long"
	}
	if len(payload.Skills) > 2200 || len(payload.Plan) > 2200 || len(payload.PrisonReason) > 2200 || len(payload.Biography) > 7000 {
		return "skills, plan, prisonReason or biography is too long"
	}
	if _, err := time.Parse("2006-01-02", payload.BirthDate); err != nil {
		return "birthDate must be in format YYYY-MM-DD"
	}
	if countSentences(payload.Biography) < 5 {
		return "biography must contain at least 5 sentences"
	}
	if !isSafeSkinURL(payload.SkinURL) {
		return "skinUrl is not safe"
	}
	return ""
}

func countSentences(text string) int {
	count := 0
	for _, separator := range []string{".", "!", "?"} {
		count += strings.Count(text, separator)
	}
	return count
}

func isSafeSkinURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
			return false
		}
	}
	if strings.Contains(host, "..") {
		return false
	}
	path := strings.ToLower(parsed.EscapedPath())
	if !imageExtRe.MatchString(path) {
		return false
	}
	return len(raw) <= 300
}

func randomHex(size int) string {
	if size <= 0 {
		size = 16
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("fallback%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func (h *DiscordAuthHandler) writeModerationHTML(w http.ResponseWriter, app rpApplicationDoc, action string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	statusText := "Заявка уже обработана"
	switch normalizedStatus(app.Status) {
	case "accepted":
		statusText = "Заявка принята"
	case "canceled":
		statusText = "Заявка отменена"
	case "call":
		statusText = "Игрок приглашен на созвон"
	case "pending":
		statusText = "Заявка возвращена на рассмотрение"
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
      <p class="muted">Ник: %s</p>
      <p class="muted">Статус: %s</p>
      <p class="muted">Действие: %s</p>
    </div>
  </body>
</html>`, statusText, statusText, app.Nickname, app.Status, action)

	_, _ = w.Write([]byte(html))
}

func safeValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "-"
	}
	return v
}

func trimForDiscord(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "-"
	}
	runes := []rune(v)
	if len(runes) <= 900 {
		return v
	}
	return string(runes[:900]) + "..."
}

func (h *DiscordAuthHandler) syncRPDiscordMessages(ctx context.Context) error {
	if strings.TrimSpace(h.rpWebhookURL) == "" {
		return nil
	}

	rows, err := h.db.QueryContext(ctx, rpApplicationSelectSQL+` WHERE discord_message_id <> '' AND status IN ('pending', 'call', 'accepted')`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		app, err := scanRPApplication(rows)
		if err != nil {
			return err
		}
		if err := h.updateRPApplicationDiscordMessage(*app); err != nil {
			return err
		}
	}

	return rows.Err()
}
