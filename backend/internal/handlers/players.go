package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type PlayerHandler struct {
	db *sql.DB
}

type registerPlayerRequest struct {
	Name string `json:"name"`
}

func NewPlayerHandler(db *sql.DB) *PlayerHandler {
	return &PlayerHandler{db: db}
}

func (h *PlayerHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.db.QueryContext(ctx, `SELECT id, name, created_at FROM players ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query players")
		return
	}
	defer rows.Close()

	players := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var name string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read players")
			return
		}
		players = append(players, map[string]any{"id": id, "name": name, "createdAt": createdAt})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read players")
		return
	}

	writeJSON(w, http.StatusOK, players)
}

func (h *PlayerHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload registerPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var id int64
	if err := h.db.QueryRowContext(ctx, `INSERT INTO players (name, created_at) VALUES ($1, $2) RETURNING id`, payload.Name, time.Now().UTC()).Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register player")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}
