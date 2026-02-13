package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DiscordAuthHandler struct {
	clientID             string
	clientSecret         string
	redirectURL          string
	frontendURL          string
	ticketWebhookURL     string
	rpWebhookURL         string
	minecraftServerToken string
	userCollection       *mongo.Collection
	rpCollection         *mongo.Collection
	codeCollection       *mongo.Collection
}

type discordTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Email      string `json:"email"`
	Avatar     string `json:"avatar"`
}

type discordUserDoc struct {
	DiscordID           string     `bson:"discordId" json:"id"`
	Username            string     `bson:"username" json:"username"`
	GlobalName          string     `bson:"globalName" json:"globalName"`
	Email               string     `bson:"email" json:"email"`
	Avatar              string     `bson:"avatar" json:"avatar"`
	LinkedMinecraft     string     `bson:"linkedMinecraft,omitempty" json:"linkedMinecraft,omitempty"`
	RPFirstName         string     `bson:"rpFirstName,omitempty" json:"rpFirstName,omitempty"`
	RPLastName          string     `bson:"rpLastName,omitempty" json:"rpLastName,omitempty"`
	MinecraftVerifiedAt *time.Time `bson:"minecraftVerifiedAt,omitempty" json:"minecraftVerifiedAt,omitempty"`
	CreatedAt           time.Time  `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt           time.Time  `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

type discordMeResponse struct {
	Authenticated bool            `json:"authenticated"`
	User          *discordUserOut `json:"user,omitempty"`
}

type discordUserOut struct {
	ID              string                   `json:"id"`
	Username        string                   `json:"username"`
	DisplayName     string                   `json:"displayName"`
	Email           string                   `json:"email"`
	Avatar          string                   `json:"avatar"`
	AvatarURL       string                   `json:"avatarUrl"`
	LinkedMinecraft string                   `json:"linkedMinecraft,omitempty"`
	RPFirstName     string                   `json:"rpFirstName,omitempty"`
	RPLastName      string                   `json:"rpLastName,omitempty"`
	ProfileURL      string                   `json:"profileUrl"`
	RPApplication   *rpApplicationSummaryOut `json:"rpApplication,omitempty"`
}

type publicProfileResponse struct {
	Profile *publicProfile `json:"profile"`
}

type publicProfile struct {
	ID              string     `json:"id"`
	Username        string     `json:"username"`
	DisplayName     string     `json:"displayName"`
	AvatarURL       string     `json:"avatarUrl"`
	LinkedMinecraft string     `json:"linkedMinecraft,omitempty"`
	RPFirstName     string     `json:"rpFirstName,omitempty"`
	RPLastName      string     `json:"rpLastName,omitempty"`
	JoinedAt        *time.Time `json:"joinedAt,omitempty"`
}

type rpApplicationSummaryOut struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Nickname    string     `json:"nickname"`
	RPName      string     `json:"rpName,omitempty"`
	Race        string     `json:"race,omitempty"`
	Gender      string     `json:"gender,omitempty"`
	BirthDate   string     `json:"birthDate,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	ModeratedAt *time.Time `json:"moderatedAt,omitempty"`
}

var minecraftNicknameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)

func NewDiscordAuthHandler(
	db *mongo.Database,
	clientID,
	clientSecret,
	redirectURL,
	frontendURL,
	ticketWebhookURL,
	rpWebhookURL,
	minecraftServerToken string,
) *DiscordAuthHandler {
	return &DiscordAuthHandler{
		clientID:             clientID,
		clientSecret:         clientSecret,
		redirectURL:          redirectURL,
		frontendURL:          frontendURL,
		ticketWebhookURL:     ticketWebhookURL,
		rpWebhookURL:         rpWebhookURL,
		minecraftServerToken: minecraftServerToken,
		userCollection:       db.Collection("discord_users"),
		rpCollection:         db.Collection("rp_applications"),
		codeCollection:       db.Collection("minecraft_verification_codes"),
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

	_, _ = h.userCollection.UpdateOne(
		ctx,
		bson.M{"discordId": user.ID},
		bson.M{
			"$set": bson.M{
				"discordId":  user.ID,
				"username":   user.Username,
				"globalName": user.GlobalName,
				"email":      user.Email,
				"avatar":     user.Avatar,
				"updatedAt":  time.Now().UTC(),
			},
			"$setOnInsert": bson.M{
				"createdAt": time.Now().UTC(),
			},
		},
		options.Update().SetUpsert(true),
	)

	setSessionCookie(w, r, h.frontendURL, user.ID)

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
	if err := h.userCollection.FindOne(ctx, bson.M{"discordId": cookie.Value}).Decode(&user); err != nil {
		writeJSON(w, http.StatusOK, discordMeResponse{Authenticated: false})
		return
	}

	summary, _ := h.latestApplicationSummary(ctx, user.DiscordID)

	writeJSON(w, http.StatusOK, discordMeResponse{
		Authenticated: true,
		User: &discordUserOut{
			ID:              user.DiscordID,
			Username:        user.Username,
			DisplayName:     displayNameFor(user),
			Email:           user.Email,
			Avatar:          user.Avatar,
			AvatarURL:       avatarURLFor(user.DiscordID, user.Avatar),
			LinkedMinecraft: user.LinkedMinecraft,
			RPFirstName:     user.RPFirstName,
			RPLastName:      user.RPLastName,
			ProfileURL:      buildProfileURL(h.frontendURL, user.DiscordID),
			RPApplication:   summary,
		},
	})
}

func (h *DiscordAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clearSessionCookie(w, r, h.frontendURL)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) PublicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	profileID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/profiles/"), "/")
	if profileID == "" || strings.Contains(profileID, "/") {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user discordUserDoc
	err := h.userCollection.FindOne(ctx, bson.M{"discordId": profileID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, publicProfileResponse{Profile: toPublicProfile(user)})
}

func resolveCookieOptions(frontendURL string, r *http.Request) (string, bool) {
	secure := strings.HasPrefix(strings.ToLower(frontendURL), "https://")
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
		secure = true
	}

	parsed, err := url.Parse(frontendURL)
	if err != nil {
		return "", secure
	}

	configuredHost := parsed.Hostname()
	requestHost := r.URL.Hostname()
	if requestHost == "" {
		requestHost = r.Host
	}
	requestHost = strings.Split(requestHost, ":")[0]

	if configuredHost == "" || requestHost == "" || !strings.EqualFold(configuredHost, requestHost) {
		return "", secure
	}

	if configuredHost == "localhost" || net.ParseIP(configuredHost) != nil {
		return "", secure
	}

	return configuredHost, secure
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, frontendURL, value string) {
	cookieDomain, secureCookie := resolveCookieOptions(frontendURL, r)
	cookie := &http.Cookie{
		Name:     "discord_id",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	}
	if cookieDomain != "" {
		cookie.Domain = cookieDomain
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, frontendURL string) {
	cookieDomain, secureCookie := resolveCookieOptions(frontendURL, r)
	cookie := &http.Cookie{
		Name:     "discord_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
	if cookieDomain != "" {
		cookie.Domain = cookieDomain
	}
	http.SetCookie(w, cookie)
}

func displayNameFor(user discordUserDoc) string {
	displayName := strings.TrimSpace(user.GlobalName)
	if displayName == "" {
		displayName = user.Username
	}
	return displayName
}

func avatarURLFor(discordID, avatar string) string {
	if avatar == "" || discordID == "" {
		return ""
	}
	return "https://cdn.discordapp.com/avatars/" + discordID + "/" + avatar + ".png?size=256"
}

func buildProfileURL(frontendURL, discordID string) string {
	if discordID == "" {
		return ""
	}
	if frontendURL == "" {
		return "/u/" + discordID
	}
	return strings.TrimRight(frontendURL, "/") + "/u/" + discordID
}

func toPublicProfile(user discordUserDoc) *publicProfile {
	profile := &publicProfile{
		ID:              user.DiscordID,
		Username:        user.Username,
		DisplayName:     displayNameFor(user),
		AvatarURL:       avatarURLFor(user.DiscordID, user.Avatar),
		LinkedMinecraft: user.LinkedMinecraft,
		RPFirstName:     user.RPFirstName,
		RPLastName:      user.RPLastName,
	}
	if !user.CreatedAt.IsZero() {
		createdAt := user.CreatedAt.UTC()
		profile.JoinedAt = &createdAt
	}
	return profile
}
