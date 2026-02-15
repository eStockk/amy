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

	if hasAccepted, err := h.hasAcceptedApplication(ctx, user.DiscordID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check current applications")
		return
	} else if hasAccepted {
		writeError(w, http.StatusConflict, "application is already accepted")
		return
	}

	if hasPending, err := h.hasPendingApplication(ctx, user.DiscordID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check current applications")
		return
	} else if hasPending {
		writeError(w, http.StatusConflict, "pending rp application already exists")
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

	var current rpApplicationDoc
	err = h.rpCollection.FindOne(ctx, bson.M{"_id": applicationID}).Decode(&current)
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

	currentStatus := normalizedStatus(current.Status)
	nextStatus, allowed := nextStatusByAction(currentStatus, action)
	if !allowed {
		h.writeModerationHTML(w, current, "already-processed")
		return
	}

	now := time.Now().UTC()
	newToken := current.ModerationToken
	setPayload := bson.M{
		"status":    nextStatus,
		"updatedAt": now,
	}
	if nextStatus == "pending" {
		newToken = randomHex(20)
		setPayload["moderationToken"] = newToken
		setPayload["moderatedAt"] = nil
		current.ModeratedAt = nil
	} else {
		setPayload["moderatedAt"] = now
		current.ModeratedAt = &now
	}

	_, err = h.rpCollection.UpdateByID(ctx, applicationID, bson.M{"$set": setPayload})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to moderate application")
		return
	}

	current.Status = nextStatus
	current.UpdatedAt = now
	current.ModerationToken = newToken

	if updateErr := h.updateRPApplicationDiscordMessage(current); updateErr != nil {
		writeError(w, http.StatusBadGateway, "failed to update discord ticket")
		return
	}

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

	if normalizedStatus(application.Status) == "accepted" {
		writeError(w, http.StatusConflict, "accepted application cannot be deleted")
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

func (h *DiscordAuthHandler) hasPendingApplication(ctx context.Context, discordID string) (bool, error) {
	count, err := h.rpCollection.CountDocuments(ctx, bson.M{"discordId": discordID, "status": "pending"})
	return count > 0, err
}

func (h *DiscordAuthHandler) hasAcceptedApplication(ctx context.Context, discordID string) (bool, error) {
	count, err := h.rpCollection.CountDocuments(ctx, bson.M{
		"discordId": discordID,
		"status":    bson.M{"$in": []string{"accepted", "approved"}},
	})
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
	case "accept", "cancel", "reconsider":
		return action
	default:
		return ""
	}
}

func nextStatusByAction(currentStatus, action string) (string, bool) {
	switch action {
	case "accept":
		if currentStatus != "pending" {
			return "", false
		}
		return "accepted", true
	case "cancel":
		if currentStatus != "pending" {
			return "", false
		}
		return "canceled", true
	case "reconsider":
		if currentStatus != "accepted" {
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
		"pending":  "\u041d\u0430 \u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440\u0435\u043d\u0438\u0438",
		"accepted": "\u041f\u0440\u0438\u043d\u044f\u0442\u0430",
		"canceled": "\u041e\u0442\u043c\u0435\u043d\u0435\u043d\u0430",
	}[status]
	if statusText == "" {
		statusText = doc.Status
	}

	discordAccount := doc.DiscordID
	if user != nil {
		discordAccount = user.Username + " (" + user.DiscordID + ")"
	}

	embed := map[string]any{
		"title":       "\u0052\u0050-\u0437\u0430\u044f\u0432\u043a\u0430: " + doc.Nickname,
		"description": "\u0421\u0442\u0430\u0442\u0443\u0441 \u0437\u0430\u044f\u0432\u043a\u0438: " + statusText,
		"color":       14901048,
		"fields": []map[string]string{
			{"name": "Discord \u0430\u043a\u043a\u0430\u0443\u043d\u0442", "value": safeValue(discordAccount)},
			{"name": "\u041d\u0438\u043a \u0432 \u0438\u0433\u0440\u0435", "value": safeValue(doc.Nickname)},
			{"name": "\u041e\u0442\u043a\u0443\u0434\u0430 \u0443\u0437\u043d\u0430\u043b \u0438 \u043e \u0441\u0435\u0440\u0432\u0435\u0440\u0435", "value": safeValue(doc.Source)},
			{"name": "\u0418\u043c\u044f \u0438 \u0444\u0430\u043c\u0438\u043b\u0438\u044f", "value": safeValue(doc.RPName)},
			{"name": "\u0414\u0430\u0442\u0430 \u0440\u043e\u0436\u0434\u0435\u043d\u0438\u044f", "value": safeValue(doc.BirthDate)},
			{"name": "\u0420\u0430\u0441\u0430", "value": safeValue(doc.Race)},
			{"name": "\u041f\u043e\u043b", "value": safeValue(doc.Gender)},
			{"name": "\u041a\u043b\u044e\u0447\u0435\u0432\u044b\u0435 \u043d\u0430\u0432\u044b\u043a\u0438", "value": trimForDiscord(doc.Skills)},
			{"name": "\u041f\u043b\u0430\u043d \u0440\u0430\u0437\u0432\u0438\u0442\u0438\u044f", "value": trimForDiscord(doc.Plan)},
			{"name": "\u0411\u0438\u043e\u0433\u0440\u0430\u0444\u0438\u044f", "value": trimForDiscord(doc.Biography)},
			{"name": "\u0421\u0441\u044b\u043b\u043a\u0430 \u043d\u0430 \u0441\u043a\u0438\u043d", "value": safeValue(doc.SkinURL)},
		},
	}

	payload := map[string]any{
		"content": "\u0052\u0050-\u0442\u0438\u043a\u0435\u0442 \u0438\u0433\u0440\u043e\u043a\u0430 " + doc.Nickname,
		"embeds":  []any{embed},
	}

	components := h.rpDiscordComponents(doc)
	if len(components) > 0 {
		payload["components"] = components
	}

	return payload
}

func (h *DiscordAuthHandler) rpModerationLinks(doc rpApplicationDoc) string {
	status := normalizedStatus(doc.Status)
	switch status {
	case "pending":
		acceptURL := h.moderationURL(doc.ID.Hex(), "accept", doc.ModerationToken)
		cancelURL := h.moderationURL(doc.ID.Hex(), "cancel", doc.ModerationToken)
		return "\u041f\u0440\u0438\u043d\u044f\u0442\u044c: " + acceptURL + "\n\u041e\u0442\u043c\u0435\u043d\u0438\u0442\u044c: " + cancelURL
	case "accepted":
		reconsiderURL := h.moderationURL(doc.ID.Hex(), "reconsider", doc.ModerationToken)
		return "\u041f\u0435\u0440\u0435\u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440: " + reconsiderURL
	default:
		return ""
	}
}

func (h *DiscordAuthHandler) rpDiscordComponents(doc rpApplicationDoc) []any {
	status := normalizedStatus(doc.Status)

	switch status {
	case "pending":
		acceptURL := h.moderationURL(doc.ID.Hex(), "accept", doc.ModerationToken)
		cancelURL := h.moderationURL(doc.ID.Hex(), "cancel", doc.ModerationToken)
		return []any{
			map[string]any{
				"type": 1,
				"components": []any{
					map[string]any{"type": 2, "style": 5, "label": "\u041f\u0440\u0438\u043d\u044f\u0442\u044c", "url": acceptURL},
					map[string]any{"type": 2, "style": 5, "label": "\u041e\u0442\u043c\u0435\u043d\u0438\u0442\u044c", "url": cancelURL},
				},
			},
		}
	case "accepted":
		reconsiderURL := h.moderationURL(doc.ID.Hex(), "reconsider", doc.ModerationToken)
		return []any{
			map[string]any{
				"type": 1,
				"components": []any{
					map[string]any{"type": 2, "style": 5, "label": "\u041f\u0435\u0440\u0435\u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440", "url": reconsiderURL},
				},
			},
		}
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

	var owner discordUserDoc
	_ = h.userCollection.FindOne(ctx, bson.M{"discordId": app.DiscordID}).Decode(&owner)
	var ownerPtr *discordUserDoc
	if strings.TrimSpace(owner.DiscordID) != "" {
		ownerPtr = &owner
	}
	payload := h.buildRPApplicationDiscordPayload(app, ownerPtr)

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

	statusText := "\u0417\u0430\u044f\u0432\u043a\u0430 \u0443\u0436\u0435 \u043e\u0431\u0440\u0430\u0431\u043e\u0442\u0430\u043d\u0430"
	switch normalizedStatus(app.Status) {
	case "accepted":
		statusText = "\u0417\u0430\u044f\u0432\u043a\u0430 \u043f\u0440\u0438\u043d\u044f\u0442\u0430"
	case "canceled":
		statusText = "\u0417\u0430\u044f\u0432\u043a\u0430 \u043e\u0442\u043c\u0435\u043d\u0435\u043d\u0430"
	case "pending":
		statusText = "\u0417\u0430\u044f\u0432\u043a\u0430 \u0432\u043e\u0437\u0432\u0440\u0430\u0449\u0435\u043d\u0430 \u043d\u0430 \u0440\u0430\u0441\u0441\u043c\u043e\u0442\u0440\u0435\u043d\u0438\u0435"
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
      <p class="muted">\u041d\u0438\u043a: %s</p>
      <p class="muted">\u0421\u0442\u0430\u0442\u0443\u0441: %s</p>
      <p class="muted">\u0414\u0435\u0439\u0441\u0442\u0432\u0438\u0435: %s</p>
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
	if len(v) <= 900 {
		return v
	}
	return v[:900] + "..."
}
