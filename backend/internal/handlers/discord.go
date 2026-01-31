package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DiscordAuthHandler struct {
	clientID     string
	clientSecret string
	redirectURL  string
	frontendURL  string
	collection   *mongo.Collection
}

type discordTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

type discordUserDoc struct {
	DiscordID string `bson:"discordId" json:"id"`
	Username  string `bson:"username" json:"username"`
	Email     string `bson:"email" json:"email"`
	Avatar    string `bson:"avatar" json:"avatar"`
}

type discordMeResponse struct {
	Authenticated bool            `json:"authenticated"`
	User          *discordUserOut `json:"user,omitempty"`
}

type discordUserOut struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar"`
	AvatarURL string `json:"avatarUrl"`
}

func NewDiscordAuthHandler(db *mongo.Database, clientID, clientSecret, redirectURL, frontendURL string) *DiscordAuthHandler {
	return &DiscordAuthHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		frontendURL:  frontendURL,
		collection:   db.Collection("discord_users"),
	}
}

func (h *DiscordAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	if h.clientID == "" || h.redirectURL == "" {
		writeError(w, http.StatusInternalServerError, "discord oauth not configured")
		return
	}

	authorizeURL := "https://discord.com/api/oauth2/authorize?" + url.Values{
		"client_id":     {h.clientID},
		"redirect_uri":  {h.redirectURL},
		"response_type": {"code"},
		"scope":         {"identify email"},
	}.Encode()

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *DiscordAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}

	if h.clientID == "" || h.clientSecret == "" || h.redirectURL == "" {
		writeError(w, http.StatusInternalServerError, "discord oauth not configured")
		return
	}

	form := url.Values{}
	form.Set("client_id", h.clientID)
	form.Set("client_secret", h.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.redirectURL)

	resp, err := http.PostForm("https://discord.com/api/oauth2/token", form)
	if err != nil {
		writeError(w, http.StatusBadGateway, "discord token exchange failed")
		return
	}
	defer resp.Body.Close()

	var token discordTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		writeError(w, http.StatusBadGateway, "invalid discord token response")
		return
	}
	if token.AccessToken == "" {
		writeError(w, http.StatusBadGateway, "missing discord access token")
		return
	}

	userReq, _ := http.NewRequest(http.MethodGet, "https://discord.com/api/users/@me", nil)
	userReq.Header.Set("Authorization", "Bearer "+token.AccessToken)

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "discord user fetch failed")
		return
	}
	defer userResp.Body.Close()

	var user discordUser
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		writeError(w, http.StatusBadGateway, "invalid discord user response")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, _ = h.collection.UpdateOne(
		ctx,
		bson.M{"discordId": user.ID},
		bson.M{
			"$set": bson.M{
				"discordId": user.ID,
				"username":  user.Username,
				"email":     user.Email,
				"avatar":    user.Avatar,
				"updatedAt": time.Now().UTC(),
			},
			"$setOnInsert": bson.M{
				"createdAt": time.Now().UTC(),
			},
		},
		options.Update().SetUpsert(true),
	)

	http.SetCookie(w, &http.Cookie{
		Name:     "discord_id",
		Value:    user.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})

	redirectTo := h.frontendURL
	if redirectTo == "" {
		redirectTo = "http://localhost:3000"
	}

	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *DiscordAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("discord_id")
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusOK, discordMeResponse{Authenticated: false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user discordUserDoc
	if err := h.collection.FindOne(ctx, bson.M{"discordId": cookie.Value}).Decode(&user); err != nil {
		writeJSON(w, http.StatusOK, discordMeResponse{Authenticated: false})
		return
	}

	avatarURL := ""
	if user.Avatar != "" {
		avatarURL = "https://cdn.discordapp.com/avatars/" + user.DiscordID + "/" + user.Avatar + ".png?size=128"
	}

	writeJSON(w, http.StatusOK, discordMeResponse{
		Authenticated: true,
		User: &discordUserOut{
			ID:        user.DiscordID,
			Username:  user.Username,
			Email:     user.Email,
			Avatar:    user.Avatar,
			AvatarURL: avatarURL,
		},
	})
}
