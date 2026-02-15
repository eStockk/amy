package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	_, err = h.userCollection.UpdateOne(ctx, bson.M{"discordId": user.DiscordID}, bson.M{"$set": bson.M{
		"presenceActive": active,
		"lastSeenAt":     now,
		"updatedAt":      now,
	}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update presence")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"active": active,
	})
}
