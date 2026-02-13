package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type minecraftVerificationCodeDoc struct {
	Code          string    `bson:"code"`
	DiscordID     string    `bson:"discordId"`
	Nickname      string    `bson:"nickname"`
	ApplicationID string    `bson:"applicationId"`
	Used          bool      `bson:"used"`
	ExpiresAt     time.Time `bson:"expiresAt"`
	CreatedAt     time.Time `bson:"createdAt"`
}

type verificationCodeRequest struct {
	Nickname string `json:"nickname"`
}

type verifyCodeRequest struct {
	Code string `json:"code"`
}

type mineRPNameRequest struct {
	Nickname  string `json:"nickname"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (h *DiscordAuthHandler) RequestMinecraftVerificationCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.validateServerToken(r) {
		writeError(w, http.StatusUnauthorized, "invalid server token")
		return
	}

	var payload verificationCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Nickname = strings.TrimSpace(payload.Nickname)
	if !minecraftNicknameRe.MatchString(payload.Nickname) {
		writeError(w, http.StatusBadRequest, "invalid nickname")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	application, err := h.findApprovedApplicationByNickname(ctx, payload.Nickname)
	if err != nil {
		writeError(w, http.StatusNotFound, "approved rp application not found")
		return
	}

	var existingUser discordUserDoc
	err = h.userCollection.FindOne(ctx, bson.M{"discordId": application.DiscordID}).Decode(&existingUser)
	if err != nil {
		writeError(w, http.StatusNotFound, "discord account not found")
		return
	}
	if existingUser.LinkedMinecraft != "" && strings.EqualFold(existingUser.LinkedMinecraft, payload.Nickname) {
		writeJSON(w, http.StatusOK, map[string]any{"alreadyVerified": true})
		return
	}

	now := time.Now().UTC()
	var existingCode minecraftVerificationCodeDoc
	codeFilter := bson.M{
		"discordId": application.DiscordID,
		"nickname":  payload.Nickname,
		"used":      false,
		"expiresAt": bson.M{"$gt": now},
	}
	if err := h.codeCollection.FindOne(ctx, codeFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&existingCode); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"code":      existingCode.Code,
			"expiresAt": existingCode.ExpiresAt,
		})
		return
	}

	expiresAt := now.Add(15 * time.Minute)
	newCode := minecraftVerificationCodeDoc{
		Code:          randomVerificationCode(8),
		DiscordID:     application.DiscordID,
		Nickname:      payload.Nickname,
		ApplicationID: application.ID.Hex(),
		Used:          false,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
	}

	if _, err := h.codeCollection.InsertOne(ctx, newCode); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":      newCode.Code,
		"expiresAt": newCode.ExpiresAt,
	})
}

func (h *DiscordAuthHandler) VerifyMinecraftCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var payload verifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Code = strings.ToUpper(strings.TrimSpace(payload.Code))
	if len(payload.Code) < 6 || len(payload.Code) > 12 {
		writeError(w, http.StatusBadRequest, "invalid code")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	var codeDoc minecraftVerificationCodeDoc
	err = h.codeCollection.FindOne(ctx, bson.M{
		"code":      payload.Code,
		"used":      false,
		"expiresAt": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&codeDoc)
	if err != nil {
		writeError(w, http.StatusBadRequest, "code is invalid or expired")
		return
	}

	if codeDoc.DiscordID != user.DiscordID {
		writeError(w, http.StatusForbidden, "code belongs to another account")
		return
	}

	now := time.Now().UTC()
	_, err = h.userCollection.UpdateOne(ctx, bson.M{"discordId": user.DiscordID}, bson.M{"$set": bson.M{
		"linkedMinecraft":     codeDoc.Nickname,
		"minecraftVerifiedAt": now,
		"updatedAt":           now,
	}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link account")
		return
	}

	_, _ = h.codeCollection.UpdateOne(ctx, bson.M{"code": codeDoc.Code}, bson.M{"$set": bson.M{"used": true}})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"nickname": codeDoc.Nickname,
	})
}

func (h *DiscordAuthHandler) UpdateMineRPName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.validateServerToken(r) {
		writeError(w, http.StatusUnauthorized, "invalid server token")
		return
	}

	var payload mineRPNameRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Nickname = strings.TrimSpace(payload.Nickname)
	payload.FirstName = strings.TrimSpace(payload.FirstName)
	payload.LastName = strings.TrimSpace(payload.LastName)

	if !minecraftNicknameRe.MatchString(payload.Nickname) {
		writeError(w, http.StatusBadRequest, "invalid nickname")
		return
	}
	if payload.FirstName == "" && payload.LastName == "" {
		writeError(w, http.StatusBadRequest, "firstName or lastName required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	now := time.Now().UTC()
	result, err := h.userCollection.UpdateOne(ctx, bson.M{"linkedMinecraft": payload.Nickname}, bson.M{"$set": bson.M{
		"rpFirstName": payload.FirstName,
		"rpLastName":  payload.LastName,
		"updatedAt":   now,
	}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rp name")
		return
	}
	if result.MatchedCount == 0 {
		writeError(w, http.StatusNotFound, "linked user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) requireAuthenticatedUser(r *http.Request) (*discordUserDoc, error) {
	cookie, err := r.Cookie("discord_id")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, mongo.ErrNoDocuments
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user discordUserDoc
	if err := h.userCollection.FindOne(ctx, bson.M{"discordId": cookie.Value}).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *DiscordAuthHandler) findApprovedApplicationByNickname(ctx context.Context, nickname string) (*rpApplicationDoc, error) {
	findOpts := options.FindOne().SetSort(bson.D{{Key: "updatedAt", Value: -1}})
	var app rpApplicationDoc
	err := h.rpCollection.FindOne(ctx, bson.M{
		"nickname": nickname,
		"status":   "approved",
	}, findOpts).Decode(&app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (h *DiscordAuthHandler) validateServerToken(r *http.Request) bool {
	token := strings.TrimSpace(r.Header.Get("X-Server-Token"))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if token == "" || h.minecraftServerToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.minecraftServerToken)) == 1
}

func randomVerificationCode(length int) string {
	if length < 6 {
		length = 6
	}
	alphabet := []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	result := make([]rune, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			result[i] = alphabet[0]
			continue
		}
		result[i] = alphabet[n.Int64()]
	}
	return string(result)
}
