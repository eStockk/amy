package handlers

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type HealthHandler struct {
	db *mongo.Database
}

func NewHealthHandler(db *mongo.Database) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Client().Ping(ctx, nil); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}