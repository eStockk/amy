package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type DiscordAuthHandler struct {
	db                   *sql.DB
	clientID             string
	clientSecret         string
	redirectURL          string
	frontendURL          string
	ticketWebhookURL     string
	rpWebhookURL         string
	rpModeratorIDs       map[string]struct{}
	minecraftServerToken string
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
	DiscordID            string     `json:"id"`
	Username             string     `json:"username"`
	GlobalName           string     `json:"globalName"`
	Email                string     `json:"email"`
	Avatar               string     `json:"avatar"`
	LinkedMinecraft      string     `json:"linkedMinecraft,omitempty"`
	RPFirstName          string     `json:"rpFirstName,omitempty"`
	RPLastName           string     `json:"rpLastName,omitempty"`
	AcceptanceStatus     string     `json:"acceptanceStatus"`
	MinecraftVerifiedAt  *time.Time `json:"minecraftVerifiedAt,omitempty"`
	LastSeenAt           *time.Time `json:"lastSeenAt,omitempty"`
	PresenceActive       bool       `json:"presenceActive,omitempty"`
	FirstAuthenticatedAt *time.Time `json:"-"`
	CreatedAt            time.Time  `json:"createdAt,omitempty"`
	UpdatedAt            time.Time  `json:"updatedAt,omitempty"`
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
	IsOnline        bool                     `json:"isOnline"`
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
	IsOnline        bool       `json:"isOnline"`
}

type rpApplicationSummaryOut struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Nickname     string     `json:"nickname"`
	RPName       string     `json:"rpName,omitempty"`
	Race         string     `json:"race,omitempty"`
	Gender       string     `json:"gender,omitempty"`
	HeightCm     int        `json:"heightCm,omitempty"`
	BirthDate    string     `json:"birthDate,omitempty"`
	PrisonReason string     `json:"prisonReason,omitempty"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
	ModeratedAt  *time.Time `json:"moderatedAt,omitempty"`
}

var minecraftNicknameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)

const presenceOnlineWindow = 75 * time.Second

func NewDiscordAuthHandler(
	db *sql.DB,
	clientID,
	clientSecret,
	redirectURL,
	frontendURL,
	ticketWebhookURL,
	rpWebhookURL,
	rpModeratorIDsRaw,
	minecraftServerToken string,
) *DiscordAuthHandler {
	return &DiscordAuthHandler{
		db:                   db,
		clientID:             clientID,
		clientSecret:         clientSecret,
		redirectURL:          redirectURL,
		frontendURL:          frontendURL,
		ticketWebhookURL:     ticketWebhookURL,
		rpWebhookURL:         rpWebhookURL,
		rpModeratorIDs:       parseDiscordIDSet(rpModeratorIDsRaw),
		minecraftServerToken: minecraftServerToken,
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

	now := time.Now().UTC()
	_, err = h.db.ExecContext(
		ctx,
		`INSERT INTO discord_users (discord_id, username, global_name, email, avatar, first_authenticated_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6, $6)
		 ON CONFLICT (discord_id) DO UPDATE SET
		   username = EXCLUDED.username,
		   global_name = EXCLUDED.global_name,
		   email = EXCLUDED.email,
		   avatar = EXCLUDED.avatar,
		   updated_at = EXCLUDED.updated_at`,
		user.ID,
		user.Username,
		user.GlobalName,
		user.Email,
		user.Avatar,
		now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save discord user")
		return
	}

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

	user, err := h.loadDiscordUser(ctx, cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusOK, discordMeResponse{Authenticated: false})
		return
	}

	summary, _ := h.latestApplicationSummary(ctx, user.DiscordID)

	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, discordMeResponse{
		Authenticated: true,
		User: &discordUserOut{
			ID:              user.DiscordID,
			Username:        user.Username,
			DisplayName:     displayNameFor(*user),
			Email:           user.Email,
			Avatar:          user.Avatar,
			AvatarURL:       avatarURLFor(user.DiscordID, user.Avatar),
			LinkedMinecraft: user.LinkedMinecraft,
			RPFirstName:     user.RPFirstName,
			RPLastName:      user.RPLastName,
			ProfileURL:      buildProfileURL(h.frontendURL, user.DiscordID),
			IsOnline:        isUserOnline(*user, now),
			RPApplication:   summary,
		},
	})
}

func (h *DiscordAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if user, err := h.requireAuthenticatedUser(r); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		_, _ = h.db.ExecContext(ctx, `UPDATE discord_users SET presence_active = FALSE, updated_at = $1 WHERE discord_id = $2`, time.Now().UTC(), user.DiscordID)
		cancel()
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

	user, err := h.loadDiscordUser(ctx, profileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fallback, fallbackErr := h.publicProfileFromRPApplication(ctx, profileID)
			if fallbackErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to load profile")
				return
			}
			if fallback == nil {
				writeError(w, http.StatusNotFound, "profile not found")
				return
			}
			writeJSON(w, http.StatusOK, publicProfileResponse{Profile: fallback})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, publicProfileResponse{Profile: toPublicProfile(*user, time.Now().UTC())})
}

func (h *DiscordAuthHandler) publicProfileFromRPApplication(ctx context.Context, discordID string) (*publicProfile, error) {
	latest, err := h.loadLatestApplicationForDiscord(ctx, discordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	displayName := strings.TrimSpace(latest.RPName)
	if displayName == "" {
		displayName = strings.TrimSpace(latest.Nickname)
	}
	if displayName == "" {
		displayName = "?????"
	}

	username := "user_" + discordID
	if len(discordID) > 6 {
		username = "user_" + discordID[len(discordID)-6:]
	}

	profile := &publicProfile{
		ID:              discordID,
		Username:        username,
		DisplayName:     displayName,
		AvatarURL:       "",
		LinkedMinecraft: latest.Nickname,
		IsOnline:        false,
	}

	if strings.TrimSpace(latest.RPName) != "" {
		parts := strings.Fields(latest.RPName)
		if len(parts) > 0 {
			profile.RPFirstName = parts[0]
		}
		if len(parts) > 1 {
			profile.RPLastName = strings.Join(parts[1:], " ")
		}
	}

	return profile, nil
}

func (h *DiscordAuthHandler) loadDiscordUser(ctx context.Context, discordID string) (*discordUserDoc, error) {
	var user discordUserDoc
	var minecraftVerifiedAt sql.NullTime
	var lastSeenAt sql.NullTime
	var firstAuthenticatedAt sql.NullTime

	err := h.db.QueryRowContext(
		ctx,
		`SELECT discord_id, username, global_name, email, avatar, linked_minecraft,
		        rp_first_name, rp_last_name, acceptance_status, minecraft_verified_at,
		        last_seen_at, presence_active, first_authenticated_at, created_at, updated_at
		 FROM discord_users
		 WHERE discord_id = $1`,
		discordID,
	).Scan(
		&user.DiscordID,
		&user.Username,
		&user.GlobalName,
		&user.Email,
		&user.Avatar,
		&user.LinkedMinecraft,
		&user.RPFirstName,
		&user.RPLastName,
		&user.AcceptanceStatus,
		&minecraftVerifiedAt,
		&lastSeenAt,
		&user.PresenceActive,
		&firstAuthenticatedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if minecraftVerifiedAt.Valid {
		user.MinecraftVerifiedAt = &minecraftVerifiedAt.Time
	}
	if lastSeenAt.Valid {
		user.LastSeenAt = &lastSeenAt.Time
	}
	if firstAuthenticatedAt.Valid {
		user.FirstAuthenticatedAt = &firstAuthenticatedAt.Time
	}
	return &user, nil
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
		MaxAge:   30 * 24 * 60 * 60,
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

func isUserOnline(user discordUserDoc, now time.Time) bool {
	if !user.PresenceActive || user.LastSeenAt == nil || user.LastSeenAt.IsZero() {
		return false
	}
	lastSeen := user.LastSeenAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.Sub(lastSeen) <= presenceOnlineWindow
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

func toPublicProfile(user discordUserDoc, now time.Time) *publicProfile {
	profile := &publicProfile{
		ID:              user.DiscordID,
		Username:        user.Username,
		DisplayName:     displayNameFor(user),
		AvatarURL:       avatarURLFor(user.DiscordID, user.Avatar),
		LinkedMinecraft: user.LinkedMinecraft,
		RPFirstName:     user.RPFirstName,
		RPLastName:      user.RPLastName,
		IsOnline:        isUserOnline(user, now),
	}

	if user.FirstAuthenticatedAt != nil && !user.FirstAuthenticatedAt.IsZero() {
		joinedAt := user.FirstAuthenticatedAt.UTC()
		profile.JoinedAt = &joinedAt
	} else if !user.CreatedAt.IsZero() {
		joinedAt := user.CreatedAt.UTC()
		profile.JoinedAt = &joinedAt
	}

	return profile
}

func parseDiscordIDSet(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		result[id] = struct{}{}
	}
	return result
}

func (h *DiscordAuthHandler) isRPModerator(discordID string) bool {
	discordID = strings.TrimSpace(discordID)
	if discordID == "" || len(h.rpModeratorIDs) == 0 {
		return false
	}
	_, ok := h.rpModeratorIDs[discordID]
	return ok
}

func (h *DiscordAuthHandler) RunMigrations(ctx context.Context) error {
	return h.syncRPDiscordMessages(ctx)
}
