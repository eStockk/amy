package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"amy/minecraft-server/internal/models"
	"amy/minecraft-server/internal/observability"
)

type NewsHandler struct {
	db               *sql.DB
	telegramChannel  string
	discordBotToken  string
	discordChannelID string
	discordGuildID   string
	httpClient       *http.Client
}

type discordNewsMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Avatar     string `json:"avatar"`
	} `json:"author"`
	Attachments []struct {
		URL         string `json:"url"`
		ProxyURL    string `json:"proxy_url"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	} `json:"attachments"`
	Embeds []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Image       struct {
			URL string `json:"url"`
		} `json:"image"`
	} `json:"embeds"`
}

var (
	telegramTextRe      = regexp.MustCompile(`(?s)<div class="tgme_widget_message_text[^"]*"[^>]*>(.*?)</div>`)
	telegramTimeRe      = regexp.MustCompile(`<time datetime="([^"]+)"`)
	htmlBreakRe         = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe           = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe             = regexp.MustCompile(`[ \t\r\f\v]+`)
	multiNewlineRe      = regexp.MustCompile(`\n{3,}`)
	newsHashTagRe       = regexp.MustCompile(`#([\p{L}\p{N}_-]+)`)
	newsVariantList     = []string{"pink", "blue", "green"}
	userNewsChannelIDs  = []string{"1205246003482075227", "1237481284494950481"}
	systemNewsChannelID = "1460666647273799842"
)

func NewNewsHandler(db *sql.DB, telegramChannel, discordBotToken, discordChannelID, discordGuildID string) *NewsHandler {
	return &NewsHandler{
		db:               db,
		telegramChannel:  strings.TrimSpace(telegramChannel),
		discordBotToken:  strings.TrimSpace(discordBotToken),
		discordChannelID: strings.TrimSpace(discordChannelID),
		discordGuildID:   strings.TrimSpace(discordGuildID),
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
	}
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

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	authorID := strings.TrimSpace(r.URL.Query().Get("authorId"))
	currentDiscordID := currentDiscordIDFromCookie(r)
	var items []models.News
	var err error

	if h.discordBotToken != "" && (category == "" || category == "all" || category == "user" || category == "users" || category == "system") {
		if items, err = h.fetchCategorizedDiscordNews(ctx, category, limit, authorID); err == nil && (len(items) > 0 || category != "") {
			_ = h.enrichNewsInteractions(ctx, items, currentDiscordID)
			writeJSON(w, http.StatusOK, items)
			return
		}
	}

	if h.telegramChannel != "" {
		if items, err = h.fetchTelegramNews(ctx, limit); err == nil && len(items) > 0 {
			_ = h.enrichNewsInteractions(ctx, items, currentDiscordID)
			writeJSON(w, http.StatusOK, items)
			return
		}
	}

	if h.discordBotToken != "" && h.discordChannelID != "" {
		if items, err = h.fetchDiscordNews(ctx, limit); err == nil && len(items) > 0 {
			_ = h.enrichNewsInteractions(ctx, items, currentDiscordID)
			writeJSON(w, http.StatusOK, items)
			return
		}
	}

	items, err = h.listStoredNews(ctx, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch news")
		return
	}

	_ = h.enrichNewsInteractions(ctx, items, currentDiscordID)
	writeJSON(w, http.StatusOK, items)
}

func (h *NewsHandler) fetchCategorizedDiscordNews(ctx context.Context, category string, limit int64, authorID string) ([]models.News, error) {
	if category == "" || category == "all" {
		systemItems, systemErr := h.fetchDiscordNewsFromChannels(ctx, []string{systemNewsChannelID}, "Системные", "system", limit, false, "")
		userItems, userErr := h.fetchDiscordNewsFromChannels(ctx, userNewsChannelIDs, "Пользовательские", "user", limit, true, authorID)
		if systemErr != nil && userErr != nil {
			return nil, systemErr
		}
		items := append(systemItems, userItems...)
		sortNews(items)
		return limitNews(items, limit), nil
	}
	if category == "system" {
		return h.fetchDiscordNewsFromChannels(ctx, []string{systemNewsChannelID}, "Системные", "system", limit, false, "")
	}
	return h.fetchDiscordNewsFromChannels(ctx, userNewsChannelIDs, "Пользовательские", "user", limit, true, authorID)
}

func (h *NewsHandler) listStoredNews(ctx context.Context, limit int64) ([]models.News, error) {
	rows, err := h.db.QueryContext(
		ctx,
		`SELECT id, title, intro, array_to_string(tags, ','), source, url, variant, created_at
		 FROM news
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.News, 0)
	for rows.Next() {
		var item models.News
		var rawTags string
		if err := rows.Scan(&item.ID, &item.Title, &item.Intro, &rawTags, &item.Source, &item.URL, &item.Variant, &item.CreatedAt); err != nil {
			return nil, err
		}
		if rawTags != "" {
			item.Tags = strings.Split(rawTags, ",")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for index := range items {
		if items[index].Variant == "" {
			items[index].Variant = newsVariant(index)
		}
	}

	return items, nil
}

func (h *NewsHandler) fetchTelegramNews(ctx context.Context, limit int64) ([]models.News, error) {
	channel := normalizeTelegramChannel(h.telegramChannel)
	if channel == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://t.me/s/"+url.PathEscape(channel), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "amy-world-news/1.0")

	startedAt := time.Now()
	resp, err := h.httpClient.Do(req)
	if err != nil {
		observability.ObserveDiscordOutbound("news_fetch", startedAt, 0, err)
		return nil, err
	}
	defer resp.Body.Close()
	observability.ObserveDiscordOutbound("news_fetch", startedAt, resp.StatusCode, nil)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errUnexpectedStatus(resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 3*1024*1024))
	if err != nil {
		return nil, err
	}

	segments := strings.Split(string(raw), `data-post="`)
	items := make([]models.News, 0, len(segments))
	for index := 1; index < len(segments); index++ {
		segment := segments[index]
		postID := html.UnescapeString(readUntil(segment, `"`))
		if postID == "" {
			continue
		}

		textHTML := firstSubmatch(telegramTextRe, segment)
		text := cleanNewsHTML(textHTML)
		if text == "" {
			continue
		}

		createdAt := parseNewsTime(firstSubmatch(telegramTimeRe, segment))
		if createdAt.IsZero() {
			createdAt = time.Now().UTC().Add(-time.Duration(index) * time.Minute)
		}

		title, intro := titleAndIntro(text)
		items = append(items, models.News{
			ID:        "telegram:" + postID,
			Title:     title,
			Intro:     intro,
			Tags:      extractNewsTags(text, "Telegram"),
			Source:    "Telegram",
			URL:       "https://t.me/" + postID,
			Variant:   newsVariant(index - 1),
			CreatedAt: createdAt,
		})
	}

	sortNews(items)
	return limitNews(items, limit), nil
}

func (h *NewsHandler) fetchDiscordNews(ctx context.Context, limit int64) ([]models.News, error) {
	requestURL := "https://discord.com/api/v10/channels/" + url.PathEscape(h.discordChannelID) + "/messages?limit=" + strconv.FormatInt(limit, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+h.discordBotToken)
	req.Header.Set("User-Agent", "amy-world-news/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errUnexpectedStatus(resp.StatusCode)
	}

	var messages []discordNewsMessage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&messages); err != nil {
		return nil, err
	}

	items := make([]models.News, 0, len(messages))
	for index, message := range messages {
		title, text := discordMessageText(message)
		if strings.TrimSpace(title) == "" && strings.TrimSpace(text) == "" {
			continue
		}

		if title == "" {
			title, text = titleAndIntro(text)
		}
		if text == "" {
			text = title
		}

		items = append(items, models.News{
			ID:        "discord:" + message.ID,
			Title:     truncateRunes(title, 96),
			Intro:     truncateRunes(text, 220),
			Tags:      extractNewsTags(title+" "+text, "Discord"),
			Source:    "Discord",
			Variant:   newsVariant(index),
			CreatedAt: parseNewsTime(message.Timestamp),
		})
	}

	sortNews(items)
	return limitNews(items, limit), nil
}

func (h *NewsHandler) fetchDiscordNewsFromChannels(ctx context.Context, channelIDs []string, source, category string, limit int64, imagesOnly bool, authorID string) ([]models.News, error) {
	perChannelLimit := limit
	if perChannelLimit < 10 {
		perChannelLimit = 10
	}
	if perChannelLimit > 100 {
		perChannelLimit = 100
	}
	items := make([]models.News, 0, limit)
	for _, channelID := range channelIDs {
		requestURL := "https://discord.com/api/v10/channels/" + url.PathEscape(channelID) + "/messages?limit=" + strconv.FormatInt(perChannelLimit, 10)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bot "+h.discordBotToken)
		req.Header.Set("User-Agent", "amy-world-news/1.0")

		resp, err := h.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			_ = resp.Body.Close()
			return nil, errUnexpectedStatus(resp.StatusCode)
		}
		var messages []discordNewsMessage
		err = json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&messages)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if authorID != "" && message.Author.ID != authorID {
				continue
			}
			if category == "user" {
				known, err := h.knownDiscordUser(ctx, message.Author.ID)
				if err != nil {
					return nil, err
				}
				if !known {
					continue
				}
			}
			item, ok := h.discordNewsItem(message, channelID, source, category, imagesOnly)
			if ok {
				item.Variant = newsVariant(len(items))
				items = append(items, item)
			}
		}
	}
	sortNews(items)
	return limitNews(items, limit), nil
}

func (h *NewsHandler) knownDiscordUser(ctx context.Context, discordID string) (bool, error) {
	discordID = strings.TrimSpace(discordID)
	if discordID == "" {
		return false, nil
	}
	var exists bool
	err := h.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM discord_users WHERE discord_id = $1)`, discordID).Scan(&exists)
	return exists, err
}

func (h *NewsHandler) discordNewsItem(message discordNewsMessage, channelID, source, category string, imagesOnly bool) (models.News, bool) {
	if category == "system" {
		source = "Системные"
	} else if category == "user" {
		source = "Пользовательские"
	}
	imageURL := discordMessageImageURL(message)
	if imagesOnly && imageURL == "" {
		return models.News{}, false
	}
	title, text := discordMessageText(message)
	if title == "" {
		title, text = titleAndIntro(text)
	}
	if title == "Пост игрока" || title == "Новость Amy" {
		title = "Пост игрока"
	}
	if title == "" && imageURL != "" {
		title = "Пост игрока"
	}
	if text == "" {
		text = title
	}
	if strings.TrimSpace(title) == "" && strings.TrimSpace(text) == "" {
		return models.News{}, false
	}
	author := strings.TrimSpace(message.Author.GlobalName)
	if author == "" {
		author = strings.TrimSpace(message.Author.Username)
	}
	return models.News{
		ID:           "discord:" + category + ":" + message.ID,
		Title:        truncateRunes(title, 96),
		Intro:        truncateRunes(text, 220),
		Tags:         []string{source},
		Source:       source,
		URL:          h.discordMessageURL(channelID, message.ID),
		ImageURL:     imageURL,
		Category:     category,
		Author:       author,
		AuthorID:     strings.TrimSpace(message.Author.ID),
		AuthorAvatar: avatarURLFor(strings.TrimSpace(message.Author.ID), strings.TrimSpace(message.Author.Avatar)),
		CreatedAt:    parseNewsTime(message.Timestamp),
	}, true
}

func discordMessageImageURL(message discordNewsMessage) string {
	for _, attachment := range message.Attachments {
		if attachment.Size > 10*1024*1024 {
			continue
		}
		if isDiscordImageAttachment(attachment.ContentType, attachment.Filename, attachment.URL) {
			if strings.TrimSpace(attachment.ProxyURL) != "" {
				return strings.TrimSpace(attachment.ProxyURL)
			}
			return strings.TrimSpace(attachment.URL)
		}
	}
	if len(message.Embeds) > 0 {
		return strings.TrimSpace(message.Embeds[0].Image.URL)
	}
	return ""
}

func isDiscordImageAttachment(contentType, filename, rawURL string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	target := strings.ToLower(strings.TrimSpace(filename))
	if target == "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			target = strings.ToLower(parsed.Path)
		}
	}
	return strings.HasSuffix(target, ".png") ||
		strings.HasSuffix(target, ".jpg") ||
		strings.HasSuffix(target, ".jpeg") ||
		strings.HasSuffix(target, ".webp") ||
		strings.HasSuffix(target, ".gif")
}

func (h *NewsHandler) discordMessageURL(channelID, messageID string) string {
	guildID := strings.TrimSpace(h.discordGuildID)
	if guildID == "" {
		guildID = "@me"
	}
	return "https://discord.com/channels/" + url.PathEscape(guildID) + "/" + url.PathEscape(channelID) + "/" + url.PathEscape(messageID)
}

func (h *NewsHandler) enrichNewsInteractions(ctx context.Context, items []models.News, discordID string) error {
	for index := range items {
		newsID := strings.TrimSpace(items[index].ID)
		if newsID == "" {
			continue
		}
		_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM news_likes WHERE news_id = $1`, newsID).Scan(&items[index].LikeCount)
		_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM news_comments WHERE news_id = $1`, newsID).Scan(&items[index].CommentCount)
		if discordID != "" {
			_ = h.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM news_likes WHERE news_id = $1 AND discord_id = $2)`, newsID, discordID).Scan(&items[index].LikedByMe)
		}
	}
	return nil
}

func normalizeTelegramChannel(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "@")
	raw = strings.TrimPrefix(raw, "https://t.me/")
	raw = strings.TrimPrefix(raw, "http://t.me/")
	raw = strings.TrimPrefix(raw, "t.me/")
	raw = strings.Trim(raw, "/")
	return raw
}

func cleanNewsHTML(raw string) string {
	raw = htmlBreakRe.ReplaceAllString(raw, "\n")
	raw = htmlTagRe.ReplaceAllString(raw, "")
	raw = html.UnescapeString(raw)
	raw = strings.ReplaceAll(raw, "\u00a0", " ")

	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(spaceRe.ReplaceAllString(line, " "))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.TrimSpace(multiNewlineRe.ReplaceAllString(strings.Join(cleaned, "\n"), "\n\n"))
}

func titleAndIntro(text string) (string, string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	title := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			title = line
			break
		}
	}

	if title == "" {
		title = "Новость Amy"
	}

	intro := strings.TrimSpace(strings.Join(lines[1:], " "))
	if intro == "" {
		intro = text
	}

	return truncateRunes(title, 96), truncateRunes(intro, 220)
}

func discordMessageText(message discordNewsMessage) (string, string) {
	content := strings.TrimSpace(message.Content)
	if len(message.Embeds) == 0 {
		return "", content
	}

	embed := message.Embeds[0]
	title := strings.TrimSpace(embed.Title)
	description := strings.TrimSpace(embed.Description)

	if content != "" && description != "" {
		description = content + "\n" + description
	} else if description == "" {
		description = content
	}

	return title, description
}

func extractNewsTags(text, fallback string) []string {
	matches := newsHashTagRe.FindAllStringSubmatch(text, -1)
	tags := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tag := strings.TrimSpace(match[1])
		key := strings.ToLower(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, tag)
		if len(tags) == 4 {
			break
		}
	}

	if len(tags) == 0 {
		tags = append(tags, fallback)
	}

	return tags
}

func parseNewsTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000000-07:00"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

func firstSubmatch(re *regexp.Regexp, text string) string {
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func readUntil(text, marker string) string {
	index := strings.Index(text, marker)
	if index < 0 {
		return text
	}
	return text[:index]
}

func sortNews(items []models.News) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

func limitNews(items []models.News, limit int64) []models.News {
	if limit <= 0 || int64(len(items)) <= limit {
		return items
	}
	return items[:int(limit)]
}

func newsVariant(index int) string {
	if index < 0 {
		index = 0
	}
	return newsVariantList[index%len(newsVariantList)]
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	return strings.TrimSpace(string(runes[:limit-1])) + "..."
}

func errUnexpectedStatus(status int) error {
	return &unexpectedStatusError{status: status}
}

type unexpectedStatusError struct {
	status int
}

func (e *unexpectedStatusError) Error() string {
	return "unexpected status: " + strconv.Itoa(e.status)
}
