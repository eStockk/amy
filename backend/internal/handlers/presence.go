package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type presencePingRequest struct {
	Active *bool `json:"active"`
}

func (h *DiscordAuthHandler) PresencePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var payload presencePingRequest
	_ = json.NewDecoder(r.Body).Decode(&payload)

	active := true
	if payload.Active != nil {
		active = *payload.Active
	}

	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = h.db.ExecContext(
		ctx,
		`UPDATE discord_users SET presence_active = $1, last_seen_at = $2, updated_at = $2 WHERE discord_id = $3`,
		active,
		now,
		user.DiscordID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update presence")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"active": active,
	})
}
