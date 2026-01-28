package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
)

type SupportHandler struct {
	collection *mongo.Collection
}

type ticketRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Subject  string `json:"subject"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

func NewSupportHandler(db *mongo.Database) *SupportHandler {
	return &SupportHandler{collection: db.Collection("tickets")}
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
	payload.Subject = strings.TrimSpace(payload.Subject)
	payload.Category = strings.TrimSpace(payload.Category)
	payload.Message = strings.TrimSpace(payload.Message)

	if payload.Name == "" || payload.Email == "" || payload.Subject == "" || payload.Message == "" {
		writeError(w, http.StatusBadRequest, "name, email, subject, message required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ticket := models.Ticket{
		Name:      payload.Name,
		Email:     payload.Email,
		Subject:   payload.Subject,
		Category:  payload.Category,
		Message:   payload.Message,
		CreatedAt: time.Now().UTC(),
	}

	if _, err := h.collection.InsertOne(ctx, ticket); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}
