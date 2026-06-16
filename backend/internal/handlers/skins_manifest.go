package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type SkinsManifestHandler struct {
	db *sql.DB
}

type skinManifestResponse struct {
	GeneratedAt time.Time           `json:"generatedAt"`
	TTLSeconds  int                 `json:"ttlSeconds"`
	Skins       []skinManifestEntry `json:"skins"`
}

type skinManifestEntry struct {
	Name      string    `json:"name"`
	SkinURL   string    `json:"skinUrl"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewSkinsManifestHandler(db *sql.DB) *SkinsManifestHandler {
	return &SkinsManifestHandler{db: db}
}

func (h *SkinsManifestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.db.QueryContext(ctx, `
WITH latest AS (
	SELECT DISTINCT ON (LOWER(nickname))
		nickname, skin_url, updated_at
	FROM rp_applications
	WHERE LOWER(status) IN ('accepted', 'approved')
	  AND NULLIF(TRIM(skin_url), '') IS NOT NULL
	ORDER BY LOWER(nickname), updated_at DESC, created_at DESC
)
SELECT nickname, skin_url, updated_at
FROM latest
ORDER BY nickname`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query skins")
		return
	}
	defer rows.Close()

	entries := make([]skinManifestEntry, 0)
	for rows.Next() {
		var entry skinManifestEntry
		if err := rows.Scan(&entry.Name, &entry.SkinURL, &entry.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read skins")
			return
		}
		entry.Name = strings.TrimSpace(entry.Name)
		entry.SkinURL = absoluteSkinURL(r, strings.TrimSpace(entry.SkinURL))
		if entry.Name == "" || entry.SkinURL == "" {
			continue
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read skins")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, skinManifestResponse{
		GeneratedAt: time.Now().UTC(),
		TTLSeconds:  30,
		Skins:       entries,
	})
}

func absoluteSkinURL(r *http.Request, raw string) string {
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return raw
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}

	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "https"
		if r.TLS == nil && (strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1")) {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + raw
}
