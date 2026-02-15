package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type rpApplicationDoc struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	DiscordID        string             `bson:"discordId"`
	Nickname         string             `bson:"nickname"`
	Source           string             `bson:"source,omitempty"`
	RPName           string             `bson:"rpName,omitempty"`
	BirthDate        string             `bson:"birthDate"`
	Race             string             `bson:"race"`
	Gender           string             `bson:"gender"`
	Skills           string             `bson:"skills"`
	Plan             string             `bson:"plan"`
	Biography        string             `bson:"biography"`
	SkinURL          string             `bson:"skinUrl"`
	Status           string             `bson:"status"`
	ModerationToken  string             `bson:"moderationToken"`
	DiscordMessageID string             `bson:"discordMessageId,omitempty"`
	ModeratedAt      *time.Time         `bson:"moderatedAt,omitempty"`
	CreatedAt        time.Time          `bson:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt"`
}

type rpApplicationRequest struct {
	Nickname  string `json:"nickname"`
	Source    string `json:"source"`
	RPName    string `json:"rpName"`
	BirthDate string `json:"birthDate"`
	Race      string `json:"race"`
	Gender    string `json:"gender"`
	Skills    string `json:"skills"`
	Plan      string `json:"plan"`
	Biography string `json:"biography"`
	SkinURL   string `json:"skinUrl"`
}

type discordWebhookMessage struct {
	ID string `json:"id"`
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

	pendingFilter := bson.M{"discordId": user.DiscordID, "status": "pending"}
	var pendingApplication rpApplicationDoc
	err = h.rpCollection.FindOne(ctx, pendingFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&pendingApplication)
	if err == nil {
		writeError(w, http.StatusConflict, "pending rp application already exists")
		return
	}
	if err != nil && err != mongo.ErrNoDocuments {
		writeError(w, http.StatusInternalServerError, "failed to check current applications")
		return
	}

	now := time.Now().UTC()
	doc := rpApplicationDoc{
		DiscordID:       user.DiscordID,
		Nickname:        payload.Nickname,
		Source:          payload.Source,
		RPName:          payload.RPName,
		BirthDate:       payload.BirthDate,
		Race:            payload.Race,
		Gender:          payload.Gender,
		Skills:          payload.Skills,
		Plan:            payload.Plan,
		Biography:       payload.Biography,
		SkinURL:         payload.SkinURL,
		Status:          "pending",
		ModerationToken: randomHex(20),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	insertResult, err := h.rpCollection.InsertOne(ctx, doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rp application")
		return
	}

	id, _ := insertResult.InsertedID.(primitive.ObjectID)
	doc.ID = id

	if h.rpWebhookURL != "" {
		messageID, webhookErr := h.sendRPApplicationWebhook(doc, user)
		if webhookErr != nil {
			_, _ = h.rpCollection.DeleteOne(ctx, bson.M{"_id": doc.ID})
			writeError(w, http.StatusBadGateway, "failed to send rp ticket to discord")
			return
		}

		if messageID != "" {
			doc.DiscordMessageID = messageID
			_, _ = h.rpCollection.UpdateByID(ctx, doc.ID, bson.M{"$set": bson.M{
				"discordMessageId": messageID,
				"updatedAt":        time.Now().UTC(),
			}})
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":        "ok",
		"applicationId": doc.ID.Hex(),
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

	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action != "approve" && action != "reject" {
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

	var current rpApplicationDoc
	err := h.rpCollection.FindOne(ctx, bson.M{"_id": applicationID}).Decode(&current)
	if err != nil {
		if err == mongo.ErrNoDocuments {
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

	if current.Status != "pending" {
		h.writeModerationHTML(w, current, "already-processed")
		return
	}

	now := time.Now().UTC()
	newStatus := "rejected"
	if action == "approve" {
		newStatus = "approved"
	}

	_, err = h.rpCollection.UpdateByID(ctx, applicationID, bson.M{"$set": bson.M{
		"status":      newStatus,
		"moderatedAt": now,
		"updatedAt":   now,
	}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to moderate application")
		return
	}

	current.Status = newStatus
	current.ModeratedAt = &now
	current.UpdatedAt = now
	h.writeModerationHTML(w, current, action)
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

	var application rpApplicationDoc
	err = h.rpCollection.FindOne(ctx, bson.M{"_id": applicationID}).Decode(&application)
	if err != nil {
		if err == mongo.ErrNoDocuments {
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

	if err := h.deleteRPApplicationDiscordMessage(application.DiscordMessageID); err != nil {
		writeError(w, http.StatusBadGateway, "failed to delete ticket message in discord")
		return
	}

	_, err = h.rpCollection.DeleteOne(ctx, bson.M{"_id": applicationID, "discordId": user.DiscordID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}

	_, _ = h.codeCollection.DeleteMany(ctx, bson.M{"applicationId": application.ID.Hex()})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) latestApplicationSummary(ctx context.Context, discordID string) (*rpApplicationSummaryOut, error) {
	findOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	var doc rpApplicationDoc
	if err := h.rpCollection.FindOne(ctx, bson.M{"discordId": discordID}, findOpts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &rpApplicationSummaryOut{
		ID:          doc.ID.Hex(),
		Status:      doc.Status,
		Nickname:    doc.Nickname,
		RPName:      doc.RPName,
		Race:        doc.Race,
		Gender:      doc.Gender,
		BirthDate:   doc.BirthDate,
		CreatedAt:   &doc.CreatedAt,
		UpdatedAt:   &doc.UpdatedAt,
		ModeratedAt: doc.ModeratedAt,
	}, nil
}

func (h *DiscordAuthHandler) sendRPApplicationWebhook(doc rpApplicationDoc, user *discordUserDoc) (string, error) {
	if h.rpWebhookURL == "" {
		return "", nil
	}

	approveURL := h.moderationURL(doc.ID.Hex(), "approve", doc.ModerationToken)
	rejectURL := h.moderationURL(doc.ID.Hex(), "reject", doc.ModerationToken)

	embed := map[string]any{
		"title":       "Новая RP-заявка: " + doc.Nickname,
		"description": "Игрок отправил RP-анкету через сайт Amy.",
		"color":       14901048,
		"fields": []map[string]string{
			{"name": "Discord аккаунт", "value": user.Username + " (" + user.DiscordID + ")"},
			{"name": "Ник в игре", "value": safeValue(doc.Nickname)},
			{"name": "Откуда узнал о сервере", "value": safeValue(doc.Source)},
			{"name": "Имя и фамилия", "value": safeValue(doc.RPName)},
			{"name": "Дата рождения", "value": safeValue(doc.BirthDate)},
			{"name": "Раса", "value": safeValue(doc.Race)},
			{"name": "Пол", "value": safeValue(doc.Gender)},
			{"name": "Ключевые навыки", "value": trimForDiscord(doc.Skills)},
			{"name": "План развития", "value": trimForDiscord(doc.Plan)},
			{"name": "Биография", "value": trimForDiscord(doc.Biography)},
			{"name": "Ссылка на скин", "value": safeValue(doc.SkinURL)},
		},
	}

	body := map[string]any{
		"content": "Поступила новая RP-заявка. Выберите действие:",
		"embeds":  []any{embed},
		"components": []any{
			map[string]any{
				"type": 1,
				"components": []any{
					map[string]any{"type": 2, "style": 5, "label": "Одобрить", "url": approveURL},
					map[string]any{"type": 2, "style": 5, "label": "Отклонить", "url": rejectURL},
				},
			},
		},
	}

	raw, err := json.Marshal(body)
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

func parseApplicationModerationIDFromPath(path string) (primitive.ObjectID, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 5 {
		return primitive.NilObjectID, false
	}
	if parts[0] != "api" || parts[1] != "rp" || parts[2] != "applications" || parts[4] != "moderate" {
		return primitive.NilObjectID, false
	}
	id, err := primitive.ObjectIDFromHex(parts[3])
	if err != nil {
		return primitive.NilObjectID, false
	}
	return id, true
}

func parseApplicationDeleteIDFromPath(path string) (primitive.ObjectID, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 4 {
		return primitive.NilObjectID, false
	}
	if parts[0] != "api" || parts[1] != "rp" || parts[2] != "applications" {
		return primitive.NilObjectID, false
	}
	id, err := primitive.ObjectIDFromHex(parts[3])
	if err != nil {
		return primitive.NilObjectID, false
	}
	return id, true
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
	req.SkinURL = strings.TrimSpace(req.SkinURL)
}

func validateRPRequest(payload rpApplicationRequest) string {
	if !minecraftNicknameRe.MatchString(payload.Nickname) {
		return "nickname must be 3-16 chars and contain only latin letters, digits or _"
	}
	if payload.BirthDate == "" || payload.Race == "" || payload.Gender == "" || payload.Skills == "" || payload.Plan == "" || payload.Biography == "" || payload.SkinURL == "" {
		return "required fields are missing"
	}
	if len(payload.Source) > 200 || len(payload.RPName) > 120 || len(payload.Race) > 80 || len(payload.Gender) > 80 {
		return "one of the text fields is too long"
	}
	if len(payload.Skills) > 2200 || len(payload.Plan) > 2200 || len(payload.Biography) > 7000 {
		return "skills, plan or biography is too long"
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
	if len(raw) > 300 {
		return false
	}
	return true
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
	if action == "approve" {
		statusText = "Заявка одобрена"
	} else if action == "reject" {
		statusText = "Заявка отклонена"
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
    </div>
  </body>
</html>`, statusText, statusText, app.Nickname, app.Status)

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
	if len(v) <= 900 {
		return v
	}
	return v[:900] + "..."
}
