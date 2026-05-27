package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"amy/minecraft-server/internal/observability"
)

type DiscordAuthHandler struct {
	db               *sql.DB
	clientID         string
	clientSecret     string
	redirectURL      string
	frontendURL      string
	ticketWebhookURL string
	rpWebhookURL     string
	rpModeratorIDs   map[string]struct{}
	discordBotToken  string
	discordGuildID   string
	httpClient       *http.Client
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
	RPFirstName          string     `json:"rpFirstName,omitempty"`
	RPLastName           string     `json:"rpLastName,omitempty"`
	AcceptanceStatus     string     `json:"acceptanceStatus"`
	LastSeenAt           *time.Time `json:"lastSeenAt,omitempty"`
	PresenceActive       bool       `json:"presenceActive,omitempty"`
	FirstAuthenticatedAt *time.Time `json:"-"`
	CreatedAt            time.Time  `json:"createdAt,omitempty"`
	UpdatedAt            time.Time  `json:"updatedAt,omitempty"`
	ProfileThemeRoleID   string     `json:"profileThemeRoleId,omitempty"`
}

type discordMeResponse struct {
	Authenticated bool            `json:"authenticated"`
	User          *discordUserOut `json:"user,omitempty"`
}

type discordUserOut struct {
	ID                 string                   `json:"id"`
	Username           string                   `json:"username"`
	DisplayName        string                   `json:"displayName"`
	Email              string                   `json:"email"`
	Avatar             string                   `json:"avatar"`
	AvatarURL          string                   `json:"avatarUrl"`
	RPFirstName        string                   `json:"rpFirstName,omitempty"`
	RPLastName         string                   `json:"rpLastName,omitempty"`
	ProfileURL         string                   `json:"profileUrl"`
	IsOnline           bool                     `json:"isOnline"`
	IsAmyDiscordMember bool                     `json:"isAmyDiscordMember"`
	RPApplication      *rpApplicationSummaryOut `json:"rpApplication,omitempty"`
}

type publicProfileResponse struct {
	Profile *publicProfile `json:"profile"`
}

type publicProfile struct {
	ID                     string              `json:"id"`
	Username               string              `json:"username"`
	DisplayName            string              `json:"displayName"`
	AvatarURL              string              `json:"avatarUrl"`
	RPFirstName            string              `json:"rpFirstName,omitempty"`
	RPLastName             string              `json:"rpLastName,omitempty"`
	RPName                 string              `json:"rpName,omitempty"`
	MinecraftNickname      string              `json:"minecraftNickname,omitempty"`
	Race                   string              `json:"race,omitempty"`
	Gender                 string              `json:"gender,omitempty"`
	BirthDate              string              `json:"birthDate,omitempty"`
	DiscordRoles           []publicDiscordRole `json:"discordRoles,omitempty"`
	ThemeRoleID            string              `json:"themeRoleId,omitempty"`
	ThemeColor             string              `json:"themeColor,omitempty"`
	HasAcceptedApplication bool                `json:"hasAcceptedApplication"`
	JoinedAt               *time.Time          `json:"joinedAt,omitempty"`
	IsOnline               bool                `json:"isOnline"`
}

type publicDiscordRole struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	Position int    `json:"position"`
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
	discordBotToken,
	discordGuildID string,
) *DiscordAuthHandler {
	return &DiscordAuthHandler{
		db:               db,
		clientID:         clientID,
		clientSecret:     clientSecret,
		redirectURL:      redirectURL,
		frontendURL:      frontendURL,
		ticketWebhookURL: ticketWebhookURL,
		rpWebhookURL:     rpWebhookURL,
		rpModeratorIDs:   parseDiscordIDSet(rpModeratorIDsRaw),
		discordBotToken:  strings.TrimSpace(discordBotToken),
		discordGuildID:   strings.TrimSpace(discordGuildID),
		httpClient:       &http.Client{Timeout: 8 * time.Second},
	}
}

func (h *DiscordAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	if h.clientID == "" || h.redirectURL == "" {
		writeError(w, http.StatusInternalServerError, "discord oauth not configured")
		return
	}

	values := url.Values{
		"client_id":     {h.clientID},
		"redirect_uri":  {h.redirectURL},
		"response_type": {"code"},
		"scope":         {"identify email"},
	}
	if redirectPath := safeInternalRedirectPath(r.URL.Query().Get("redirect")); redirectPath != "" {
		values.Set("state", redirectPath)
	}

	authorizeURL := "https://discord.com/api/oauth2/authorize?" + values.Encode()

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

	startedAt := time.Now()
	resp, err := http.PostForm("https://discord.com/api/oauth2/token", form)
	if err != nil {
		observability.ObserveDiscordOutbound("oauth_token", startedAt, 0, err)
		writeError(w, http.StatusBadGateway, "discord token exchange failed")
		return
	}
	defer resp.Body.Close()
	observability.ObserveDiscordOutbound("oauth_token", startedAt, resp.StatusCode, nil)

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

	startedAt = time.Now()
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		observability.ObserveDiscordOutbound("oauth_user", startedAt, 0, err)
		writeError(w, http.StatusBadGateway, "discord user fetch failed")
		return
	}
	defer userResp.Body.Close()
	observability.ObserveDiscordOutbound("oauth_user", startedAt, userResp.StatusCode, nil)

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

	redirectTo := h.redirectTargetFromState(r.URL.Query().Get("state"))

	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *DiscordAuthHandler) redirectTargetFromState(state string) string {
	base := strings.TrimRight(h.frontendURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	if redirectPath := safeInternalRedirectPath(state); redirectPath != "" {
		return base + redirectPath
	}
	return base
}

func safeInternalRedirectPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return ""
	}
	return parsed.RequestURI()
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
	isAmyDiscordMember, _ := h.isAmyDiscordMember(ctx, user.DiscordID)

	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, discordMeResponse{
		Authenticated: true,
		User: &discordUserOut{
			ID:                 user.DiscordID,
			Username:           user.Username,
			DisplayName:        displayNameFor(*user),
			Email:              user.Email,
			Avatar:             user.Avatar,
			AvatarURL:          avatarURLFor(user.DiscordID, user.Avatar),
			RPFirstName:        user.RPFirstName,
			RPLastName:         user.RPLastName,
			ProfileURL:         buildProfileURL(h.frontendURL, user.DiscordID),
			IsOnline:           isUserOnline(*user, now),
			IsAmyDiscordMember: isAmyDiscordMember,
			RPApplication:      summary,
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

	profile := toPublicProfile(*user, time.Now().UTC())
	_ = h.enrichPublicProfile(ctx, profile)
	writeJSON(w, http.StatusOK, publicProfileResponse{Profile: profile})
}

func (h *DiscordAuthHandler) publicProfileFromRPApplication(ctx context.Context, discordID string) (*publicProfile, error) {
	latest, err := h.loadAcceptedApplicationForDiscord(ctx, discordID)
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
		displayName = "Профиль"
	}

	username := "user_" + discordID
	if len(discordID) > 6 {
		username = "user_" + discordID[len(discordID)-6:]
	}

	profile := &publicProfile{
		ID:          discordID,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   "",
		IsOnline:    false,
	}

	applyRPApplicationToProfile(profile, latest)

	return profile, nil
}

func (h *DiscordAuthHandler) loadDiscordUser(ctx context.Context, discordID string) (*discordUserDoc, error) {
	var user discordUserDoc
	var lastSeenAt sql.NullTime
	var firstAuthenticatedAt sql.NullTime

	err := h.db.QueryRowContext(
		ctx,
		`SELECT discord_id, username, global_name, email, avatar,
		        rp_first_name, rp_last_name, acceptance_status,
		        last_seen_at, presence_active, first_authenticated_at, created_at, updated_at, profile_theme_role_id
		 FROM discord_users
		 WHERE discord_id = $1`,
		discordID,
	).Scan(
		&user.DiscordID,
		&user.Username,
		&user.GlobalName,
		&user.Email,
		&user.Avatar,
		&user.RPFirstName,
		&user.RPLastName,
		&user.AcceptanceStatus,
		&lastSeenAt,
		&user.PresenceActive,
		&firstAuthenticatedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ProfileThemeRoleID,
	)
	if err != nil {
		return nil, err
	}
	if lastSeenAt.Valid {
		user.LastSeenAt = &lastSeenAt.Time
	}
	if firstAuthenticatedAt.Valid {
		user.FirstAuthenticatedAt = &firstAuthenticatedAt.Time
	}
	return &user, nil
}

func (h *DiscordAuthHandler) requireAuthenticatedUser(r *http.Request) (*discordUserDoc, error) {
	cookie, err := r.Cookie("discord_id")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, sql.ErrNoRows
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	return h.loadDiscordUser(ctx, strings.TrimSpace(cookie.Value))
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

func (h *DiscordAuthHandler) isAmyDiscordMember(ctx context.Context, discordID string) (bool, error) {
	discordID = strings.TrimSpace(discordID)
	if discordID == "" {
		return false, nil
	}

	var exists bool
	err := h.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM discord_member_states WHERE discord_id = $1)`, discordID).Scan(&exists)
	return exists, err
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
		ID:          user.DiscordID,
		Username:    user.Username,
		DisplayName: displayNameFor(user),
		AvatarURL:   avatarURLFor(user.DiscordID, user.Avatar),
		RPFirstName: user.RPFirstName,
		RPLastName:  user.RPLastName,
		IsOnline:    isUserOnline(user, now),
		ThemeRoleID: strings.TrimSpace(user.ProfileThemeRoleID),
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

func (h *DiscordAuthHandler) enrichPublicProfile(ctx context.Context, profile *publicProfile) error {
	if profile == nil || strings.TrimSpace(profile.ID) == "" {
		return nil
	}

	if app, err := h.loadAcceptedApplicationForDiscord(ctx, profile.ID); err == nil {
		applyRPApplicationToProfile(profile, app)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var rawRoles string
	var rawRoleIDs string
	err := h.db.QueryRowContext(ctx, `SELECT array_to_string(roles, E'\n'), array_to_string(role_ids, E'\n') FROM discord_member_states WHERE discord_id = $1`, profile.ID).Scan(&rawRoles, &rawRoleIDs)
	if err == nil {
		roles := splitPostgresTextArray(rawRoles)
		roleIDs := splitPostgresTextArray(rawRoleIDs)
		profile.DiscordRoles = h.publicDiscordRoles(ctx, roles, roleIDs)
		if profile.ThemeRoleID == "" || !hasPublicRoleID(profile.DiscordRoles, profile.ThemeRoleID) {
			profile.ThemeRoleID = highestPublicRole(profile.DiscordRoles).ID
		}
		profile.ThemeColor = roleColorByID(profile.DiscordRoles, profile.ThemeRoleID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func (h *DiscordAuthHandler) UpdateProfileTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var payload struct {
		RoleID string `json:"roleId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8*1024)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	roleID := strings.TrimSpace(payload.RoleID)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var rawRoleIDs string
	if err := h.db.QueryRowContext(ctx, `SELECT array_to_string(role_ids, E'\n') FROM discord_member_states WHERE discord_id = $1`, user.DiscordID).Scan(&rawRoleIDs); err != nil {
		writeError(w, http.StatusForbidden, "discord roles are not synced")
		return
	}
	roleIDs := splitPostgresTextArray(rawRoleIDs)
	if roleID != "" && !stringSliceContains(roleIDs, roleID) {
		writeError(w, http.StatusForbidden, "role is not available")
		return
	}
	if _, err := h.db.ExecContext(ctx, `UPDATE discord_users SET profile_theme_role_id = $1, updated_at = $2 WHERE discord_id = $3`, roleID, time.Now().UTC(), user.DiscordID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save theme")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "roleId": roleID})
}

func (h *DiscordAuthHandler) loadAcceptedApplicationForDiscord(ctx context.Context, discordID string) (*rpApplicationDoc, error) {
	return scanRPApplication(h.db.QueryRowContext(ctx, rpApplicationSelectSQL+` WHERE discord_id = $1 AND status IN ('accepted', 'approved') ORDER BY updated_at DESC, created_at DESC LIMIT 1`, discordID))
}

func (h *DiscordAuthHandler) publicDiscordRoles(ctx context.Context, roleNames, roleIDs []string) []publicDiscordRole {
	meta := map[string]discordGuildRole{}
	if h.discordBotToken != "" && h.discordGuildID != "" {
		if roles, err := h.fetchGuildRoles(ctx); err == nil {
			for _, role := range roles {
				meta[role.ID] = role
			}
		}
	}

	result := make([]publicDiscordRole, 0, len(roleIDs)+len(roleNames))
	seen := map[string]struct{}{}
	for index, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		role := meta[roleID]
		name := strings.TrimSpace(role.Name)
		if name == "" && index < len(roleNames) {
			name = strings.TrimSpace(roleNames[index])
		}
		if name == "" || strings.EqualFold(name, "@everyone") {
			continue
		}
		seen[roleID] = struct{}{}
		seen[strings.ToLower(name)] = struct{}{}
		result = append(result, publicDiscordRole{
			ID:       roleID,
			Name:     name,
			Color:    discordColorHex(role.Color),
			Position: role.Position,
		})
	}
	for _, name := range filterPublicDiscordRoles(roleNames) {
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		result = append(result, publicDiscordRole{ID: key, Name: name})
		seen[key] = struct{}{}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Position > result[j].Position
	})
	return result
}

func (h *DiscordAuthHandler) fetchGuildRoles(ctx context.Context) ([]discordGuildRole, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/guilds/"+url.PathEscape(h.discordGuildID)+"/roles", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+h.discordBotToken)
	req.Header.Set("User-Agent", "amy-world-profile-roles/1.0")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errUnexpectedStatus(resp.StatusCode)
	}
	var roles []discordGuildRole
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func highestPublicRole(roles []publicDiscordRole) publicDiscordRole {
	for _, role := range roles {
		if role.Color != "" {
			return role
		}
	}
	if len(roles) > 0 {
		return roles[0]
	}
	return publicDiscordRole{}
}

func roleColorByID(roles []publicDiscordRole, roleID string) string {
	for _, role := range roles {
		if role.ID == roleID {
			return role.Color
		}
	}
	return ""
}

func hasPublicRoleID(roles []publicDiscordRole, roleID string) bool {
	for _, role := range roles {
		if role.ID == roleID {
			return true
		}
	}
	return false
}

func discordColorHex(color int) string {
	if color <= 0 {
		return ""
	}
	return fmt.Sprintf("#%06x", color)
}

func stringSliceContains(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func splitPostgresTextArray(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func applyRPApplicationToProfile(profile *publicProfile, app *rpApplicationDoc) {
	if profile == nil || app == nil {
		return
	}
	profile.HasAcceptedApplication = app.Status == "accepted" || app.Status == "approved"
	profile.RPName = strings.TrimSpace(app.RPName)
	profile.MinecraftNickname = strings.TrimSpace(app.Nickname)
	profile.Race = strings.TrimSpace(app.Race)
	profile.Gender = strings.TrimSpace(app.Gender)
	profile.BirthDate = strings.TrimSpace(app.BirthDate)
	if profile.RPName != "" {
		parts := strings.Fields(profile.RPName)
		if len(parts) > 0 {
			profile.RPFirstName = parts[0]
		}
		if len(parts) > 1 {
			profile.RPLastName = strings.Join(parts[1:], " ")
		}
	}
}

func filterPublicDiscordRoles(roles []string) []string {
	result := make([]string, 0, len(roles))
	seen := map[string]struct{}{}
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" || strings.EqualFold(role, "@everyone") {
			continue
		}
		key := strings.ToLower(role)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, role)
	}
	return result
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
