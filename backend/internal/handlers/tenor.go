package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TenorHandler struct {
	apiKey     string
	httpClient *http.Client
}

type tenorGIFOut struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Preview string `json:"preview"`
	Title   string `json:"title"`
}

func NewTenorHandler(apiKey string) *TenorHandler {
	return &TenorHandler{
		apiKey: strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
	}
}

func (h *TenorHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.apiKey == "" {
		writeError(w, http.StatusServiceUnavailable, "tenor is not configured")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 12
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 24 {
			limit = parsed
		}
	}
	endpoint := "https://tenor.googleapis.com/v2/featured"
	values := url.Values{
		"key":          {h.apiKey},
		"client_key":   {"amy_world"},
		"media_filter": {"gif,tinygif"},
		"limit":        {strconv.Itoa(limit)},
		"locale":       {"ru_RU"},
	}
	if query != "" {
		endpoint = "https://tenor.googleapis.com/v2/search"
		values.Set("q", query)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build tenor request")
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load tenor gifs")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		writeError(w, http.StatusBadGateway, "failed to load tenor gifs")
		return
	}
	var raw struct {
		Results []struct {
			ID           string `json:"id"`
			ContentDesc  string `json:"content_description"`
			MediaFormats map[string]struct {
				URL string `json:"url"`
			} `json:"media_formats"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&raw); err != nil {
		writeError(w, http.StatusBadGateway, "invalid tenor response")
		return
	}
	items := make([]tenorGIFOut, 0, len(raw.Results))
	for _, item := range raw.Results {
		gifURL := strings.TrimSpace(item.MediaFormats["gif"].URL)
		preview := strings.TrimSpace(item.MediaFormats["tinygif"].URL)
		if gifURL == "" {
			continue
		}
		if preview == "" {
			preview = gifURL
		}
		items = append(items, tenorGIFOut{ID: item.ID, URL: gifURL, Preview: preview, Title: item.ContentDesc})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": items})
}
