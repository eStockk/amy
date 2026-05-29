package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxProxiedMediaBytes = 12 * 1024 * 1024

type MediaProxyHandler struct {
	cacheDir   string
	httpClient *http.Client
}

func NewMediaProxyHandler(cacheDir string) *MediaProxyHandler {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		cacheDir = "data/media-cache"
	}
	return &MediaProxyHandler{
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (h *MediaProxyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if !isSafeMediaURL(rawURL) {
		writeError(w, http.StatusBadRequest, "invalid media url")
		return
	}

	name := mediaCacheName(rawURL)
	target := filepath.Join(h.cacheDir, name)
	if serveCachedMedia(w, r, target) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media url")
		return
	}
	req.Header.Set("User-Agent", "amy-world-media-cache/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load media")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		writeError(w, http.StatusBadGateway, "failed to load media")
		return
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProxiedMediaBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxProxiedMediaBytes {
		writeError(w, http.StatusBadGateway, "failed to load media")
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(raw)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		writeError(w, http.StatusBadGateway, "media is not an image")
		return
	}

	if err := os.MkdirAll(h.cacheDir, 0o750); err == nil {
		tmp := target + ".tmp"
		if writeErr := os.WriteFile(tmp, raw, 0o640); writeErr == nil {
			_ = os.Rename(tmp, target)
		} else {
			_ = os.Remove(tmp)
		}
	}

	writeMediaBytes(w, r, raw, contentType)
}

func proxiedMediaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/api/") {
		return raw
	}
	if !isSafeMediaURL(raw) {
		return raw
	}
	return "/api/media/proxy?url=" + url.QueryEscape(raw)
}

func serveCachedMedia(w http.ResponseWriter, r *http.Request, path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return false
	}
	writeMediaBytes(w, r, raw, http.DetectContentType(raw))
	return true
}

func writeMediaBytes(w http.ResponseWriter, r *http.Request, raw []byte, contentType string) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(raw)
}

func mediaCacheName(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:]) + ".img"
}

func isSafeMediaURL(raw string) bool {
	if len(raw) > 2000 {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || strings.Contains(host, "..") || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return true
}
