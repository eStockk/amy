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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	rpModeratorIDs       map[string]struct{}
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
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	DiscordID           string             `bson:"discordId" json:"id"`
	Username            string             `bson:"username" json:"username"`
	GlobalName          string             `bson:"globalName" json:"globalName"`
	Email               string             `bson:"email" json:"email"`
	Avatar              string             `bson:"avatar" json:"avatar"`
	LinkedMinecraft     string             `bson:"linkedMinecraft,omitempty" json:"linkedMinecraft,omitempty"`
	RPFirstName         string             `bson:"rpFirstName,omitempty" json:"rpFirstName,omitempty"`
	RPLastName          string             `bson:"rpLastName,omitempty" json:"rpLastName,omitempty"`
	MinecraftVerifiedAt *time.Time         `bson:"minecraftVerifiedAt,omitempty" json:"minecraftVerifiedAt,omitempty"`
	LastSeenAt          *time.Time         `bson:"lastSeenAt,omitempty" json:"lastSeenAt,omitempty"`
	PresenceActive      bool               `bson:"presenceActive,omitempty" json:"presenceActive,omitempty"`
	CreatedAt           time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt           time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
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

const presenceOnlineWindow = 75 * time.Second

func NewDiscordAuthHandler(
	db *mongo.Database,
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
		clientID:             clientID,
		clientSecret:         clientSecret,
		redirectURL:          redirectURL,
		frontendURL:          frontendURL,
		ticketWebhookURL:     ticketWebhookURL,
		rpWebhookURL:         rpWebhookURL,
		rpModeratorIDs:       parseDiscordIDSet(rpModeratorIDsRaw),
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

	now := time.Now().UTC()
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
			IsOnline:        isUserOnline(user, now),
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
		_, _ = h.userCollection.UpdateOne(ctx, bson.M{"discordId": user.DiscordID}, bson.M{"$set": bson.M{
			"presenceActive": false,
			"updatedAt":      time.Now().UTC(),
		}})
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

	var user discordUserDoc
	err := h.userCollection.FindOne(ctx, bson.M{"discordId": profileID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
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

	profile := toPublicProfile(user, time.Now().UTC())
	if earliest := h.earliestKnownJoinAt(ctx, user.DiscordID); earliest != nil {
		if profile.JoinedAt == nil || earliest.Before(*profile.JoinedAt) {
			profile.JoinedAt = earliest
		}
	}

	writeJSON(w, http.StatusOK, publicProfileResponse{Profile: profile})
}

func (h *DiscordAuthHandler) publicProfileFromRPApplication(ctx context.Context, discordID string) (*publicProfile, error) {
	var latest rpApplicationDoc
	err := h.rpCollection.FindOne(
		ctx,
		bson.M{"discordId": discordID},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}),
	).Decode(&latest)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
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

	joinedAt := latest.CreatedAt.UTC()
	if joinedAt.IsZero() {
		joinedAt = latest.UpdatedAt.UTC()
	}
	if !joinedAt.IsZero() {
		profile.JoinedAt = &joinedAt
	}

	return profile, nil
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

	var joinedAt time.Time
	if !user.CreatedAt.IsZero() {
		joinedAt = user.CreatedAt.UTC()
	}
	if !user.ID.IsZero() {
		idCreatedAt := user.ID.Timestamp().UTC()
		if joinedAt.IsZero() || idCreatedAt.Before(joinedAt) {
			joinedAt = idCreatedAt
		}
	}
	if !joinedAt.IsZero() {
		profile.JoinedAt = &joinedAt
	}

	return profile
}

func (h *DiscordAuthHandler) earliestKnownJoinAt(ctx context.Context, discordID string) *time.Time {
	if strings.TrimSpace(discordID) == "" {
		return nil
	}

	type createdAtDoc struct {
		CreatedAt time.Time `bson:"createdAt"`
	}

	findEarliest := func(collection *mongo.Collection, filter bson.M) *time.Time {
		if collection == nil {
			return nil
		}

		var result createdAtDoc
		err := collection.FindOne(
			ctx,
			filter,
			options.FindOne().
				SetSort(bson.D{{Key: "createdAt", Value: 1}}).
				SetProjection(bson.M{"createdAt": 1}),
		).Decode(&result)
		if err != nil || result.CreatedAt.IsZero() {
			return nil
		}

		createdAt := result.CreatedAt.UTC()
		return &createdAt
	}

	candidates := []*time.Time{
		findEarliest(h.rpCollection, bson.M{"discordId": discordID}),
		findEarliest(h.codeCollection, bson.M{"discordId": discordID}),
	}

	var earliest *time.Time
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if earliest == nil || candidate.Before(*earliest) {
			earliest = candidate
		}
	}

	return earliest
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
	if discordID == "" {
		return false
	}
	if len(h.rpModeratorIDs) == 0 {
		return false
	}
	_, ok := h.rpModeratorIDs[discordID]
	return ok
}

func (h *DiscordAuthHandler) RunMigrations(ctx context.Context) error {
	if err := h.ensureMigrationIndexes(ctx); err != nil {
		return err
	}

	if err := h.migrateRPStatuses(ctx); err != nil {
		return err
	}

	if err := h.syncRPDiscordMessages(ctx); err != nil {
		return err
	}

	cursor, err := h.userCollection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"discordId": 1,
		"createdAt": 1,
	}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var user discordUserDoc
		if err := cursor.Decode(&user); err != nil {
			return err
		}

		if strings.TrimSpace(user.DiscordID) == "" {
			continue
		}

		current := user.CreatedAt
		target := current

		if target.IsZero() && !user.ID.IsZero() {
			target = user.ID.Timestamp().UTC()
		}

		if earliest := h.earliestKnownJoinAt(ctx, user.DiscordID); earliest != nil {
			if target.IsZero() || earliest.Before(target) {
				target = *earliest
			}
		}

		if target.IsZero() {
			continue
		}

		if !current.IsZero() && (current.Equal(target) || current.Before(target)) {
			continue
		}

		_, err = h.userCollection.UpdateOne(
			ctx,
			bson.M{"discordId": user.DiscordID},
			bson.M{"$set": bson.M{"createdAt": target}},
		)
		if err != nil {
			return err
		}
	}

	return cursor.Err()
}

func (h *DiscordAuthHandler) ensureMigrationIndexes(ctx context.Context) error {
	if h.userCollection != nil {
		_, err := h.userCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "discordId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("discord_users_discordId_unique"),
		})
		if err := ignoreIndexConflict(err); err != nil {
			return err
		}
	}

	if h.rpCollection != nil {
		_, err := h.rpCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "discordId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("rp_applications_discordId_createdAt"),
		})
		if err := ignoreIndexConflict(err); err != nil {
			return err
		}

		_, err = h.rpCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}},
			Options: options.Index().SetName("rp_applications_status_updatedAt"),
		})
		if err := ignoreIndexConflict(err); err != nil {
			return err
		}
	}

	if h.codeCollection != nil {
		_, err := h.codeCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "discordId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("verification_codes_discordId_createdAt"),
		})
		if err := ignoreIndexConflict(err); err != nil {
			return err
		}

		_, err = h.codeCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "code", Value: 1}},
			Options: options.Index().SetName("verification_codes_code"),
		})
		if err := ignoreIndexConflict(err); err != nil {
			return err
		}
	}

	return nil
}

func ignoreIndexConflict(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "already exists") || strings.Contains(lower, "indexoptionsconflict") || strings.Contains(lower, "index key specs conflict") {
		return nil
	}
	return err
}

func (h *DiscordAuthHandler) migrateRPStatuses(ctx context.Context) error {
	type rpMigrationDoc struct {
		ID              primitive.ObjectID `bson:"_id"`
		Status          string             `bson:"status"`
		ModerationToken string             `bson:"moderationToken"`
		ModeratedAt     *time.Time         `bson:"moderatedAt,omitempty"`
		CreatedAt       time.Time          `bson:"createdAt,omitempty"`
		UpdatedAt       time.Time          `bson:"updatedAt,omitempty"`
	}

	cursor, err := h.rpCollection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"status":          1,
		"moderationToken": 1,
		"moderatedAt":     1,
		"createdAt":       1,
		"updatedAt":       1,
	}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc rpMigrationDoc
		if err := cursor.Decode(&doc); err != nil {
			return err
		}

		nextStatus := normalizedStatus(doc.Status)
		if nextStatus == "" {
			nextStatus = "pending"
		}
		setPayload := bson.M{}

		if nextStatus != doc.Status {
			setPayload["status"] = nextStatus
		}

		if nextStatus == "pending" {
			if strings.TrimSpace(doc.ModerationToken) == "" {
				setPayload["moderationToken"] = randomHex(20)
			}
			if doc.ModeratedAt != nil {
				setPayload["moderatedAt"] = nil
			}
		} else if (nextStatus == "accepted" || nextStatus == "canceled") && doc.ModeratedAt == nil {
			moderatedAt := doc.UpdatedAt.UTC()
			if moderatedAt.IsZero() {
				moderatedAt = doc.CreatedAt.UTC()
			}
			if moderatedAt.IsZero() {
				moderatedAt = time.Now().UTC()
			}
			setPayload["moderatedAt"] = moderatedAt
		}

		if len(setPayload) == 0 {
			continue
		}

		if _, hasUpdatedAt := setPayload["updatedAt"]; !hasUpdatedAt {
			setPayload["updatedAt"] = time.Now().UTC()
		}

		_, err = h.rpCollection.UpdateByID(ctx, doc.ID, bson.M{"$set": setPayload})
		if err != nil {
			return err
		}
	}

	return cursor.Err()
}

func (h *DiscordAuthHandler) syncRPDiscordMessages(ctx context.Context) error {
	if h.rpCollection == nil || strings.TrimSpace(h.rpWebhookURL) == "" {
		return nil
	}

	filter := bson.M{
		"discordMessageId": bson.M{"$exists": true, "$ne": ""},
		"status":           bson.M{"$in": []string{"pending", "accepted", "approved"}},
	}

	cursor, err := h.rpCollection.Find(ctx, filter, options.Find().SetProjection(bson.M{
		"discordId":        1,
		"discordMessageId": 1,
		"moderationToken":  1,
		"status":           1,
		"nickname":         1,
		"source":           1,
		"rpName":           1,
		"birthDate":        1,
		"race":             1,
		"gender":           1,
		"skills":           1,
		"plan":             1,
		"biography":        1,
		"skinUrl":          1,
		"updatedAt":        1,
	}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var app rpApplicationDoc
		if err := cursor.Decode(&app); err != nil {
			return err
		}
		if strings.TrimSpace(app.DiscordMessageID) == "" {
			continue
		}
		if err := h.updateRPApplicationDiscordMessage(app); err != nil {
			return err
		}
	}

	return cursor.Err()
}
