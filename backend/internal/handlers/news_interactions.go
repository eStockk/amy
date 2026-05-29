package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type newsCommentOut struct {
	ID           int64     `json:"id"`
	NewsID       string    `json:"newsId"`
	Author       string    `json:"author"`
	AuthorID     string    `json:"authorId,omitempty"`
	AuthorAvatar string    `json:"authorAvatar,omitempty"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (h *NewsHandler) Like(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	discordID := currentDiscordIDFromCookie(r)
	if discordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var payload struct {
		NewsID string `json:"newsId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8*1024)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	newsID := strings.TrimSpace(payload.NewsID)
	if newsID == "" || len(newsID) > 180 {
		writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var liked bool
	err := h.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM news_likes WHERE news_id = $1 AND discord_id = $2)`, newsID, discordID).Scan(&liked)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load like")
		return
	}
	if liked {
		_, err = h.db.ExecContext(ctx, `DELETE FROM news_likes WHERE news_id = $1 AND discord_id = $2`, newsID, discordID)
		liked = false
	} else {
		_, err = h.db.ExecContext(ctx, `INSERT INTO news_likes (news_id, discord_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, newsID, discordID)
		liked = true
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save like")
		return
	}
	count := 0
	_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM news_likes WHERE news_id = $1`, newsID).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{"liked": liked, "likeCount": count})
}

func (h *NewsHandler) Comments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listComments(w, r)
	case http.MethodPost:
		h.createComment(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *NewsHandler) listComments(w http.ResponseWriter, r *http.Request) {
	newsID := strings.TrimSpace(r.URL.Query().Get("newsId"))
	if newsID == "" || len(newsID) > 180 {
		writeError(w, http.StatusBadRequest, "invalid news id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.db.QueryContext(
		ctx,
		`SELECT c.id, c.news_id, c.author_name, c.discord_id, COALESCE(u.avatar, ''), c.message, c.created_at
		 FROM news_comments c
		 LEFT JOIN discord_users u ON u.discord_id = c.discord_id
		 WHERE c.news_id = $1
		 ORDER BY c.created_at ASC
		 LIMIT 80`,
		newsID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load comments")
		return
	}
	defer rows.Close()
	items := make([]newsCommentOut, 0)
	for rows.Next() {
		var item newsCommentOut
		var avatarHash string
		if err := rows.Scan(&item.ID, &item.NewsID, &item.Author, &item.AuthorID, &avatarHash, &item.Message, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read comments")
			return
		}
		item.AuthorAvatar = avatarURLFor(item.AuthorID, avatarHash)
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": items})
}

func (h *NewsHandler) createComment(w http.ResponseWriter, r *http.Request) {
	discordID := currentDiscordIDFromCookie(r)
	if discordID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var payload struct {
		NewsID  string `json:"newsId"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	newsID := strings.TrimSpace(payload.NewsID)
	message := strings.TrimSpace(payload.Message)
	if newsID == "" || len(newsID) > 180 || message == "" {
		writeError(w, http.StatusBadRequest, "comment is required")
		return
	}
	if len([]rune(message)) > 800 {
		writeError(w, http.StatusBadRequest, "comment is too long")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := (&DiscordAuthHandler{db: h.db}).loadDiscordUser(ctx, discordID)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	author := "Игрок"
	if user != nil {
		author = displayNameFor(*user)
	}
	var out newsCommentOut
	var avatarHash string
	err = h.db.QueryRowContext(
		ctx,
		`INSERT INTO news_comments (news_id, discord_id, author_name, message)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, news_id, author_name, discord_id, message, created_at`,
		newsID,
		discordID,
		author,
		message,
	).Scan(&out.ID, &out.NewsID, &out.Author, &out.AuthorID, &out.Message, &out.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save comment")
		return
	}
	if user != nil {
		avatarHash = user.Avatar
	}
	out.AuthorAvatar = avatarURLFor(out.AuthorID, avatarHash)
	writeJSON(w, http.StatusOK, map[string]any{"comment": out})
}
