package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PlayerHandler struct {
	collection *mongo.Collection
}

type registerPlayerRequest struct {
	Name string `json:"name"`
}

func NewPlayerHandler(db *mongo.Database) *PlayerHandler {
	return &PlayerHandler{collection: db.Collection("players")}
}

func (h *PlayerHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cursor, err := h.collection.Find(ctx, bson.M{}, options.Find().SetLimit(50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query players")
		return
	}
	defer cursor.Close(ctx)

	players := make([]bson.M, 0)
	if err := cursor.All(ctx, &players); err != nil {
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

	result, err := h.collection.InsertOne(ctx, bson.M{
		"name":      payload.Name,
		"createdAt": time.Now().UTC(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register player")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": result.InsertedID})
}