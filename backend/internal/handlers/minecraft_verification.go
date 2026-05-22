package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type minecraftVerificationCodeDoc struct {
	Code          string
	DiscordID     string
	Nickname      string
	ApplicationID string
	Used          bool
	ExpiresAt     time.Time
	CreatedAt     time.Time
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
		writeError(w, http.StatusNotFound, "approved or accepted rp application not found")
		return
	}

	existingUser, err := h.loadDiscordUser(ctx, application.DiscordID)
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
	err = h.db.QueryRowContext(
		ctx,
		`SELECT code, discord_id, nickname, application_id, used, expires_at, created_at
		 FROM minecraft_verification_codes
		 WHERE discord_id = $1 AND nickname = $2 AND used = FALSE AND expires_at > $3
		 ORDER BY created_at DESC
		 LIMIT 1`,
		application.DiscordID,
		payload.Nickname,
		now,
	).Scan(
		&existingCode.Code,
		&existingCode.DiscordID,
		&existingCode.Nickname,
		&existingCode.ApplicationID,
		&existingCode.Used,
		&existingCode.ExpiresAt,
		&existingCode.CreatedAt,
	)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"code":      existingCode.Code,
			"expiresAt": existingCode.ExpiresAt,
		})
		return
	}
	if err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to load verification code")
		return
	}

	expiresAt := now.Add(15 * time.Minute)
	newCode := minecraftVerificationCodeDoc{
		Code:          randomVerificationCode(8),
		DiscordID:     application.DiscordID,
		Nickname:      payload.Nickname,
		ApplicationID: application.ID,
		Used:          false,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
	}

	_, err = h.db.ExecContext(
		ctx,
		`INSERT INTO minecraft_verification_codes (code, discord_id, nickname, application_id, used, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		newCode.Code,
		newCode.DiscordID,
		newCode.Nickname,
		newCode.ApplicationID,
		newCode.Used,
		newCode.ExpiresAt,
		newCode.CreatedAt,
	)
	if err != nil {
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
	err = h.db.QueryRowContext(
		ctx,
		`SELECT code, discord_id, nickname, application_id, used, expires_at, created_at
		 FROM minecraft_verification_codes
		 WHERE code = $1 AND used = FALSE AND expires_at > $2`,
		payload.Code,
		time.Now().UTC(),
	).Scan(
		&codeDoc.Code,
		&codeDoc.DiscordID,
		&codeDoc.Nickname,
		&codeDoc.ApplicationID,
		&codeDoc.Used,
		&codeDoc.ExpiresAt,
		&codeDoc.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "code is invalid or expired")
		return
	}

	if codeDoc.DiscordID != user.DiscordID {
		writeError(w, http.StatusForbidden, "code belongs to another account")
		return
	}

	now := time.Now().UTC()
	_, err = h.db.ExecContext(
		ctx,
		`UPDATE discord_users
		 SET linked_minecraft = $1, minecraft_verified_at = $2, updated_at = $2
		 WHERE discord_id = $3`,
		codeDoc.Nickname,
		now,
		user.DiscordID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link account")
		return
	}

	_, _ = h.db.ExecContext(ctx, `UPDATE minecraft_verification_codes SET used = TRUE WHERE code = $1`, codeDoc.Code)

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
	result, err := h.db.ExecContext(
		ctx,
		`UPDATE discord_users
		 SET rp_first_name = $1, rp_last_name = $2, updated_at = $3
		 WHERE linked_minecraft = $4`,
		payload.FirstName,
		payload.LastName,
		now,
		payload.Nickname,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rp name")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeError(w, http.StatusNotFound, "linked user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) requireAuthenticatedUser(r *http.Request) (*discordUserDoc, error) {
	cookie, err := r.Cookie("discord_id")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, sql.ErrNoRows
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	return h.loadDiscordUser(ctx, cookie.Value)
}

func (h *DiscordAuthHandler) findApprovedApplicationByNickname(ctx context.Context, nickname string) (*rpApplicationDoc, error) {
	return scanRPApplication(h.db.QueryRowContext(ctx, rpApplicationSelectSQL+` WHERE nickname = $1 AND status IN ('accepted', 'approved') ORDER BY updated_at DESC LIMIT 1`, nickname))
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
