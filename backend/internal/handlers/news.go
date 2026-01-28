package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"amy/minecraft-server/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NewsHandler struct {
	collection *mongo.Collection
}

func NewNewsHandler(db *mongo.Database) *NewsHandler {
	return &NewsHandler{collection: db.Collection("news")}
}

func (h *NewsHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := int64(3)
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit)
	cursor, err := h.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch news")
		return
	}
	defer cursor.Close(ctx)

	items := make([]models.News, 0)
	if err := cursor.All(ctx, &items); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read news")
		return
	}

	writeJSON(w, http.StatusOK, items)
}
