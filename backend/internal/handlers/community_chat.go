package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const communityChatChannelID = "1458094528723423338"

type CommunityChatHandler struct {
	db         *sql.DB
	botToken   string
	httpClient *http.Client
}

type communityChatMessage struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	AvatarURL string    `json:"avatarUrl,omitempty"`
	Message   string    `json:"message"`
	ImageURL  string    `json:"imageUrl,omitempty"`
	GIFURL    string    `json:"gifUrl,omitempty"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

type discordChannelMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Avatar     string `json:"avatar"`
		Bot        bool   `json:"bot"`
	} `json:"author"`
	Member *struct {
		Nick string `json:"nick"`
	} `json:"member"`
	Attachments []struct {
		Filename    string `json:"filename"`
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	} `json:"attachments"`
	Embeds []struct {
		Description string `json:"description"`
		URL         string `json:"url"`
		Author      struct {
			Name    string `json:"name"`
			IconURL string `json:"icon_url"`
		} `json:"author"`
		Image struct {
			URL string `json:"url"`
		} `json:"image"`
	} `json:"embeds"`
}

func NewCommunityChatHandler(db *sql.DB, botToken string) *CommunityChatHandler {
	return &CommunityChatHandler{
		db:       db,
		botToken: strings.TrimSpace(botToken),
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (h *CommunityChatHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.botToken == "" {
		writeError(w, http.StatusServiceUnavailable, "discord chat is not configured")
		return
	}

	discordID := currentDiscordIDFromCookie(r)
	if discordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, app, err := h.acceptedChatUser(ctx, discordID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusForbidden, "accepted rp application required")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to check chat access")
		return
	}

	if r.Method == http.MethodPost {
		var payload struct {
			Message string `json:"message"`
			GIFURL  string `json:"gifUrl"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		payload.Message = strings.TrimSpace(payload.Message)
		payload.GIFURL = strings.TrimSpace(payload.GIFURL)
		if payload.Message == "" && payload.GIFURL == "" {
			writeError(w, http.StatusBadRequest, "message or gif required")
			return
		}
		if len([]rune(payload.Message)) > 1000 {
			writeError(w, http.StatusBadRequest, "message is too long")
			return
		}
		if payload.GIFURL != "" && !isAllowedTenorURL(payload.GIFURL) {
			writeError(w, http.StatusBadRequest, "only tenor gif links are allowed")
			return
		}
		if err := h.sendDiscordChatMessage(ctx, *user, *app, payload.Message, payload.GIFURL); err != nil {
			writeError(w, http.StatusBadGateway, "failed to send discord message")
			return
		}
	}

	messages, err := h.fetchDiscordChatMessages(ctx, 50)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load discord messages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (h *CommunityChatHandler) acceptedChatUser(ctx context.Context, discordID string) (*discordUserDoc, *rpApplicationDoc, error) {
	user, err := (&DiscordAuthHandler{db: h.db}).loadDiscordUser(ctx, discordID)
	if err != nil {
		return nil, nil, err
	}
	app, err := (&DiscordAuthHandler{db: h.db}).loadAcceptedApplicationForDiscord(ctx, discordID)
	if err != nil {
		return nil, nil, err
	}
	return user, app, nil
}

func (h *CommunityChatHandler) fetchDiscordChatMessages(ctx context.Context, limit int) ([]communityChatMessage, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	requestURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages?limit=%d", url.PathEscape(communityChatChannelID), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+h.botToken)
	req.Header.Set("User-Agent", "amy-world-community-chat/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errUnexpectedStatus(resp.StatusCode)
	}

	var raw []discordChannelMessage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&raw); err != nil {
		return nil, err
	}
	result := make([]communityChatMessage, 0, len(raw))
	for _, item := range raw {
		if mapped := mapDiscordChatMessage(item); mapped.ID != "" {
			result = append(result, mapped)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (h *CommunityChatHandler) sendDiscordChatMessage(ctx context.Context, user discordUserDoc, app rpApplicationDoc, message, gifURL string) error {
	authorName := strings.TrimSpace(app.Nickname)
	if display := displayNameFor(user); display != "" {
		authorName += " / " + display
	}
	if authorName == "" {
		authorName = displayNameFor(user)
	}
	description := strings.TrimSpace(message)
	if description == "" && gifURL != "" {
		description = "GIF"
	}
	payload := map[string]any{
		"allowed_mentions": map[string]any{"parse": []string{}},
		"embeds": []map[string]any{{
			"description": description,
			"color":       16079160,
			"author": map[string]string{
				"name":     authorName,
				"icon_url": avatarURLFor(user.DiscordID, user.Avatar),
			},
		}},
	}
	if gifURL != "" {
		payload["embeds"].([]map[string]any)[0]["image"] = map[string]string{"url": gifURL}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://discord.com/api/v10/channels/"+url.PathEscape(communityChatChannelID)+"/messages", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+h.botToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "amy-world-community-chat/1.0")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errUnexpectedStatus(resp.StatusCode)
	}
	return nil
}

func mapDiscordChatMessage(item discordChannelMessage) communityChatMessage {
	createdAt := parseNewsTime(item.Timestamp)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	author := strings.TrimSpace(item.Author.GlobalName)
	if item.Member != nil && strings.TrimSpace(item.Member.Nick) != "" {
		author = strings.TrimSpace(item.Member.Nick)
	}
	if author == "" {
		author = strings.TrimSpace(item.Author.Username)
	}
	avatarURL := avatarURLFor(item.Author.ID, item.Author.Avatar)
	text := strings.TrimSpace(item.Content)
	imageURL := ""
	gifURL := ""
	if len(item.Embeds) > 0 {
		if strings.TrimSpace(item.Embeds[0].Author.Name) != "" {
			author = strings.TrimSpace(item.Embeds[0].Author.Name)
		}
		if strings.TrimSpace(item.Embeds[0].Author.IconURL) != "" {
			avatarURL = strings.TrimSpace(item.Embeds[0].Author.IconURL)
		}
		if text == "" {
			text = strings.TrimSpace(item.Embeds[0].Description)
		}
		if strings.TrimSpace(item.Embeds[0].Image.URL) != "" {
			gifURL = strings.TrimSpace(item.Embeds[0].Image.URL)
		}
	}
	for _, attachment := range item.Attachments {
		if attachment.Size > 10*1024*1024 {
			continue
		}
		contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
		if strings.HasPrefix(contentType, "image/gif") {
			gifURL = attachment.URL
			break
		}
		if strings.HasPrefix(contentType, "image/") && imageURL == "" {
			imageURL = attachment.URL
		}
	}
	if text == "" && imageURL == "" && gifURL == "" {
		return communityChatMessage{}
	}
	if len([]rune(text)) > 1200 {
		text = string([]rune(text)[:1200]) + "..."
	}
	return communityChatMessage{
		ID:        item.ID,
		Author:    author,
		AvatarURL: avatarURL,
		Message:   text,
		ImageURL:  imageURL,
		GIFURL:    gifURL,
		Source:    "Discord",
		CreatedAt: createdAt,
	}
}

func isAllowedTenorURL(raw string) bool {
	if len(raw) > 500 {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "tenor.com" && host != "www.tenor.com" && host != "media.tenor.com" {
		return false
	}
	path := strings.ToLower(parsed.EscapedPath())
	return strings.Contains(path, "/view/") || strings.HasSuffix(path, ".gif")
}
