package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type dashboardResponse struct {
	Players dashboardPlayersBlock `json:"players"`
	Support dashboardSupportBlock `json:"support"`
	RP      dashboardRPBlock      `json:"rp"`
}

type dashboardPlayersBlock struct {
	Total int                   `json:"total"`
	Items []dashboardPlayerItem `json:"items"`
}

type dashboardPlayerItem struct {
	DiscordID       string     `json:"discordId"`
	DiscordName     string     `json:"discordName"`
	MinecraftNick   string     `json:"minecraftNick"`
	SkinURL         string     `json:"skinUrl"`
	SiteOnline      bool       `json:"siteOnline"`
	MinecraftOnline bool       `json:"minecraftOnline"`
	DiscordOnline   string     `json:"discordOnline"`
	SupportTickets  int        `json:"supportTickets"`
	OpenTickets     int        `json:"openTickets"`
	RPStatus        string     `json:"rpStatus"`
	DiscordRoles    []string   `json:"discordRoles"`
	LastSeenAt      *time.Time `json:"lastSeenAt,omitempty"`
}

type dashboardSupportBlock struct {
	Total    int                   `json:"total"`
	Open     int                   `json:"open"`
	Resolved int                   `json:"resolved"`
	Items    []dashboardTicketItem `json:"items"`
}

type dashboardTicketItem struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	DiscordNick string     `json:"discordNick"`
	Subject     string     `json:"subject"`
	Category    string     `json:"category"`
	Message     string     `json:"message"`
	Status      string     `json:"status"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type dashboardRPBlock struct {
	Total    int                        `json:"total"`
	Accepted int                        `json:"accepted"`
	Other    int                        `json:"other"`
	Items    []dashboardApplicationItem `json:"items"`
}

type dashboardApplicationItem struct {
	ID            string    `json:"id"`
	DiscordID     string    `json:"discordId"`
	DiscordName   string    `json:"discordName"`
	MinecraftNick string    `json:"minecraftNick"`
	Status        string    `json:"status"`
	SiteOnline    bool      `json:"siteOnline"`
	DiscordOnline string    `json:"discordOnline"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ticketStatusRequest struct {
	Status string `json:"status"`
}

func (h *DiscordAuthHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.requireRPModerator(w, r) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	minecraftOnline := h.minecraftOnlineSet(ctx)
	ticketCounts, openCounts, err := h.supportCountsByDiscordNick(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load support stats")
		return
	}
	latestRP, err := h.latestRPStatuses(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load rp stats")
		return
	}
	roleMap := h.discordRoleMap(ctx)

	players, err := h.dashboardPlayers(ctx, minecraftOnline, ticketCounts, openCounts, latestRP, roleMap)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load players")
		return
	}
	support, err := h.dashboardSupport(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load support tickets")
		return
	}
	rp, err := h.dashboardRP(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load rp applications")
		return
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		Players: dashboardPlayersBlock{Total: len(players), Items: players},
		Support: support,
		RP:      rp,
	})
}

func (h *DiscordAuthHandler) UpdateSupportTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.requireRPModerator(w, r) {
		return
	}

	id, ok := parseSupportTicketID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var payload ticketStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	status := normalizeTicketStatus(payload.Status)
	if status == "" {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	var resolvedAt any
	if status == "resolved" {
		resolvedAt = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.db.ExecContext(ctx, `UPDATE support_tickets SET status = $1, resolved_at = $2 WHERE id = $3`, status, resolvedAt, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update ticket")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DiscordAuthHandler) requireRPModerator(w http.ResponseWriter, r *http.Request) bool {
	user, err := h.requireAuthenticatedUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return false
	}
	if !h.isRPModerator(user.DiscordID) {
		writeError(w, http.StatusForbidden, "moderator access required")
		return false
	}
	return true
}

func (h *DiscordAuthHandler) minecraftOnlineSet(ctx context.Context) map[string]struct{} {
	result := map[string]struct{}{}
	if strings.TrimSpace(h.minecraftServerAddr) == "" {
		return result
	}
	status, err := queryMinecraftStatus(ctx, h.minecraftServerAddr)
	if err != nil {
		return result
	}
	for _, player := range status.Players.Sample {
		if player.Name != "" {
			result[strings.ToLower(player.Name)] = struct{}{}
		}
	}
	return result
}

func (h *DiscordAuthHandler) supportCountsByDiscordNick(ctx context.Context) (map[string]int, map[string]int, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT LOWER(discord_nick), COUNT(*), COUNT(*) FILTER (WHERE status <> 'resolved') FROM support_tickets WHERE discord_nick <> '' GROUP BY LOWER(discord_nick)`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	total := map[string]int{}
	open := map[string]int{}
	for rows.Next() {
		var nick string
		var count, openCount int
		if err := rows.Scan(&nick, &count, &openCount); err != nil {
			return nil, nil, err
		}
		total[nick] = count
		open[nick] = openCount
	}
	return total, open, rows.Err()
}

func (h *DiscordAuthHandler) latestRPStatuses(ctx context.Context) (map[string]string, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT DISTINCT ON (discord_id) discord_id, status FROM rp_applications ORDER BY discord_id, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]string{}
	for rows.Next() {
		var discordID, status string
		if err := rows.Scan(&discordID, &status); err != nil {
			return nil, err
		}
		result[discordID] = status
	}
	return result, rows.Err()
}

func (h *DiscordAuthHandler) dashboardPlayers(ctx context.Context, minecraftOnline map[string]struct{}, ticketCounts, openCounts map[string]int, latestRP map[string]string, roleMap map[string][]string) ([]dashboardPlayerItem, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT discord_id, username, global_name, avatar, linked_minecraft, last_seen_at, presence_active FROM discord_users ORDER BY updated_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := make([]dashboardPlayerItem, 0)
	for rows.Next() {
		var discordID, username, globalName, avatar, minecraftNick string
		var lastSeen sql.NullTime
		var presenceActive bool
		if err := rows.Scan(&discordID, &username, &globalName, &avatar, &minecraftNick, &lastSeen, &presenceActive); err != nil {
			return nil, err
		}

		user := discordUserDoc{
			DiscordID:       discordID,
			Username:        username,
			GlobalName:      globalName,
			Avatar:          avatar,
			LinkedMinecraft: minecraftNick,
			PresenceActive:  presenceActive,
		}
		if lastSeen.Valid {
			user.LastSeenAt = &lastSeen.Time
		}
		displayName := displayNameFor(user)
		supportTickets := 0
		openTickets := 0
		for _, ticketKey := range supportTicketLookupKeys(username, globalName) {
			supportTickets += ticketCounts[ticketKey]
			openTickets += openCounts[ticketKey]
		}
		_, mcOnline := minecraftOnline[strings.ToLower(minecraftNick)]
		item := dashboardPlayerItem{
			DiscordID:       discordID,
			DiscordName:     displayName,
			MinecraftNick:   minecraftNick,
			SkinURL:         minecraftSkinRenderURL(minecraftNick),
			SiteOnline:      isUserOnline(user, now),
			MinecraftOnline: mcOnline,
			DiscordOnline:   "unknown",
			SupportTickets:  supportTickets,
			OpenTickets:     openTickets,
			RPStatus:        safeDashboardStatus(latestRP[discordID]),
			DiscordRoles:    roleMap[discordID],
		}
		if lastSeen.Valid {
			seen := lastSeen.Time.UTC()
			item.LastSeenAt = &seen
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *DiscordAuthHandler) dashboardSupport(ctx context.Context) (dashboardSupportBlock, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT id, name, discord_nick, subject, category, message, status, resolved_at, created_at FROM support_tickets ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return dashboardSupportBlock{}, err
	}
	defer rows.Close()

	block := dashboardSupportBlock{Items: make([]dashboardTicketItem, 0)}
	for rows.Next() {
		var item dashboardTicketItem
		var resolvedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.DiscordNick, &item.Subject, &item.Category, &item.Message, &item.Status, &resolvedAt, &item.CreatedAt); err != nil {
			return dashboardSupportBlock{}, err
		}
		if resolvedAt.Valid {
			resolved := resolvedAt.Time.UTC()
			item.ResolvedAt = &resolved
		}
		block.Total++
		if item.Status == "resolved" {
			block.Resolved++
		} else {
			block.Open++
		}
		block.Items = append(block.Items, item)
	}
	return block, rows.Err()
}

func (h *DiscordAuthHandler) dashboardRP(ctx context.Context) (dashboardRPBlock, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT a.id, a.discord_id, COALESCE(u.username, ''), COALESCE(u.global_name, ''), a.nickname, a.status, a.created_at, COALESCE(u.presence_active, FALSE), u.last_seen_at FROM rp_applications a LEFT JOIN discord_users u ON u.discord_id = a.discord_id ORDER BY a.created_at DESC LIMIT 200`)
	if err != nil {
		return dashboardRPBlock{}, err
	}
	defer rows.Close()

	block := dashboardRPBlock{Items: make([]dashboardApplicationItem, 0)}
	now := time.Now().UTC()
	for rows.Next() {
		var item dashboardApplicationItem
		var username, globalName string
		var presenceActive bool
		var lastSeen sql.NullTime
		if err := rows.Scan(&item.ID, &item.DiscordID, &username, &globalName, &item.MinecraftNick, &item.Status, &item.CreatedAt, &presenceActive, &lastSeen); err != nil {
			return dashboardRPBlock{}, err
		}
		user := discordUserDoc{DiscordID: item.DiscordID, Username: username, GlobalName: globalName, PresenceActive: presenceActive}
		if lastSeen.Valid {
			user.LastSeenAt = &lastSeen.Time
		}
		item.DiscordName = displayNameFor(user)
		item.SiteOnline = isUserOnline(user, now)
		item.DiscordOnline = "unknown"
		block.Total++
		if normalizedStatus(item.Status) == "accepted" {
			block.Accepted++
		} else {
			block.Other++
		}
		block.Items = append(block.Items, item)
	}
	return block, rows.Err()
}

func (h *DiscordAuthHandler) discordRoleMap(ctx context.Context) map[string][]string {
	result := map[string][]string{}
	if strings.TrimSpace(h.discordBotToken) == "" || strings.TrimSpace(h.discordGuildID) == "" {
		return result
	}
	roleNames := h.fetchDiscordGuildRoles(ctx)

	rows, err := h.db.QueryContext(ctx, `SELECT discord_id FROM discord_users ORDER BY updated_at DESC LIMIT 200`)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var discordID string
		if err := rows.Scan(&discordID); err != nil {
			return result
		}
		roleIDs := h.fetchDiscordMemberRoleIDs(ctx, discordID)
		for _, roleID := range roleIDs {
			if name := roleNames[roleID]; name != "" {
				result[discordID] = append(result[discordID], name)
			}
		}
	}
	return result
}

func (h *DiscordAuthHandler) fetchDiscordGuildRoles(ctx context.Context) map[string]string {
	result := map[string]string{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/guilds/"+url.PathEscape(h.discordGuildID)+"/roles", nil)
	if err != nil {
		return result
	}
	req.Header.Set("Authorization", "Bot "+h.discordBotToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return result
	}
	var roles []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return result
	}
	for _, role := range roles {
		result[role.ID] = role.Name
	}
	return result
}

func (h *DiscordAuthHandler) fetchDiscordMemberRoleIDs(ctx context.Context, discordID string) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/guilds/"+url.PathEscape(h.discordGuildID)+"/members/"+url.PathEscape(discordID), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bot "+h.discordBotToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}
	var member struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil
	}
	return member.Roles
}

func parseSupportTicketID(path string) (int64, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "dashboard" || parts[2] != "support" || parts[3] != "tickets" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[4], 10, 64)
	return id, err == nil && id > 0
}

func normalizeTicketStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "open", "unresolved", "pending":
		return "open"
	case "resolved", "closed", "done":
		return "resolved"
	default:
		return ""
	}
}

func safeDashboardStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "missing"
	}
	return status
}

func supportTicketLookupKeys(username, globalName string) []string {
	result := make([]string, 0, 2)
	for _, value := range []string{username, globalName} {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		seen := false
		for _, current := range result {
			if current == value {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, value)
		}
	}
	return result
}

func minecraftSkinRenderURL(nickname string) string {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || !minecraftNicknameRe.MatchString(nickname) {
		return ""
	}
	return fmt.Sprintf("https://starlightskins.lunareclipse.studio/render/default/%s/full", url.PathEscape(nickname))
}
