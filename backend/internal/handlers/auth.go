package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	collection *mongo.Collection
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAuthHandler(db *mongo.Database) *AuthHandler {
	return &AuthHandler{collection: db.Collection("users")}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload registerRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Name = strings.TrimSpace(payload.Name)

	if payload.Email == "" || payload.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	count, err := h.collection.CountDocuments(ctx, bson.M{"email": payload.Email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "user already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := models.User{
		Email:        payload.Email,
		Name:         payload.Name,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	if _, err := h.collection.InsertOne(ctx, user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))

	if payload.Email == "" || payload.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user models.User
	if err := h.collection.FindOne(ctx, bson.M{"email": payload.Email}).Decode(&user); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
