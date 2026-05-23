package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type DiscordMemberSync struct {
	db         *sql.DB
	botToken   string
	guildID    string
	httpClient *http.Client
}

type discordGuildRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type discordGuildMember struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Roles []string `json:"roles"`
}

type discordGatewayMessage struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s"`
	T  string          `json:"t"`
}

type discordGatewayHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type discordGatewayPresence struct {
	GuildID string `json:"guild_id"`
	Status  string `json:"status"`
	User    struct {
		ID string `json:"id"`
	} `json:"user"`
}

type discordGatewayGuildCreate struct {
	ID        string                   `json:"id"`
	Presences []discordGatewayPresence `json:"presences"`
}

const (
	discordGatewayOpDispatch     = 0
	discordGatewayOpHeartbeat    = 1
	discordGatewayOpIdentify     = 2
	discordGatewayOpHello        = 10
	discordGatewayOpHeartbeatACK = 11

	discordGatewayIntentGuilds         = 1 << 0
	discordGatewayIntentGuildMembers   = 1 << 1
	discordGatewayIntentGuildPresences = 1 << 8
)

func NewDiscordMemberSync(db *sql.DB, botToken, guildID string) *DiscordMemberSync {
	return &DiscordMemberSync{
		db:       db,
		botToken: strings.TrimSpace(botToken),
		guildID:  strings.TrimSpace(guildID),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *DiscordMemberSync) Start(ctx context.Context) {
	if s.botToken == "" || s.guildID == "" {
		return
	}
	if !strings.Contains(s.botToken, ".") {
		log.Printf("discord member sync disabled: DISCORD_BOT_TOKEN looks like an ID, not a bot token")
		return
	}

	go func() {
		if err := s.Sync(ctx); err != nil && ctx.Err() == nil {
			log.Printf("discord member sync failed: %v", err)
		}

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Sync(ctx); err != nil && ctx.Err() == nil {
					log.Printf("discord member sync failed: %v", err)
				}
			}
		}
	}()

	go s.runPresenceGateway(ctx)
}

func (s *DiscordMemberSync) Sync(ctx context.Context) error {
	roles, err := s.fetchRoles(ctx)
	if err != nil {
		return err
	}

	after := "0"
	now := time.Now().UTC()
	for {
		members, err := s.fetchMembers(ctx, after)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		for _, member := range members {
			discordID := strings.TrimSpace(member.User.ID)
			if discordID == "" {
				continue
			}

			roleNames := make([]string, 0, len(member.Roles))
			for _, roleID := range member.Roles {
				if name := roles[roleID]; name != "" && name != "@everyone" {
					roleNames = append(roleNames, name)
				}
			}

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO discord_member_states (discord_id, roles, discord_status, synced_at)
				 VALUES ($1, $2, 'offline', $3)
				 ON CONFLICT (discord_id) DO UPDATE SET
				   roles = EXCLUDED.roles,
				   synced_at = EXCLUDED.synced_at`,
				discordID,
				roleNames,
				now,
			); err != nil {
				_ = tx.Rollback()
				return err
			}

			after = discordID
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		if len(members) < 1000 {
			return nil
		}
	}
}

func (s *DiscordMemberSync) fetchRoles(ctx context.Context) (map[string]string, error) {
	var roles []discordGuildRole
	if err := s.fetchDiscord(ctx, "/guilds/"+url.PathEscape(s.guildID)+"/roles", &roles); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(roles))
	for _, role := range roles {
		result[role.ID] = role.Name
	}
	return result, nil
}

func (s *DiscordMemberSync) fetchMembers(ctx context.Context, after string) ([]discordGuildMember, error) {
	var members []discordGuildMember
	path := "/guilds/" + url.PathEscape(s.guildID) + "/members?limit=1000&after=" + url.QueryEscape(after)
	if err := s.fetchDiscord(ctx, path, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *DiscordMemberSync) fetchDiscord(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+s.botToken)
	req.Header.Set("User-Agent", "amy-world-discord-sync/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &discordSyncStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}

	return json.NewDecoder(io.LimitReader(resp.Body, 8*1024*1024)).Decode(target)
}

func (s *DiscordMemberSync) runPresenceGateway(ctx context.Context) {
	for {
		if err := s.listenPresenceGateway(ctx); err != nil && ctx.Err() == nil {
			log.Printf("discord presence gateway failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

func (s *DiscordMemberSync) listenPresenceGateway(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, "wss://gateway.discord.gg/?v=10&encoding=json", http.Header{
		"User-Agent": []string{"amy-world-discord-presence/1.0"},
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	var lastSequence atomic.Int64
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	var writeMu sync.Mutex

	for {
		var message discordGatewayMessage
		if err := conn.ReadJSON(&message); err != nil {
			return err
		}
		if message.S != nil {
			lastSequence.Store(*message.S)
		}

		switch message.Op {
		case discordGatewayOpHello:
			var hello discordGatewayHello
			if err := json.Unmarshal(message.D, &hello); err != nil {
				return err
			}
			if hello.HeartbeatInterval <= 0 {
				return nil
			}
			if err := s.identifyGateway(conn, &writeMu); err != nil {
				return err
			}
			go heartbeatGateway(conn, &writeMu, time.Duration(hello.HeartbeatInterval)*time.Millisecond, &lastSequence, heartbeatDone)
		case discordGatewayOpDispatch:
			if message.T == "PRESENCE_UPDATE" {
				if err := s.handlePresenceUpdate(ctx, message.D); err != nil && ctx.Err() == nil {
					log.Printf("discord presence update failed: %v", err)
				}
			}
			if message.T == "GUILD_CREATE" {
				if err := s.handleGuildCreate(ctx, message.D); err != nil && ctx.Err() == nil {
					log.Printf("discord guild presence snapshot failed: %v", err)
				}
			}
		case discordGatewayOpHeartbeat:
			if err := writeGatewayHeartbeat(conn, &writeMu, lastSequence.Load()); err != nil {
				return err
			}
		case discordGatewayOpHeartbeatACK:
		}
	}
}

func (s *DiscordMemberSync) identifyGateway(conn *websocket.Conn, writeMu *sync.Mutex) error {
	payload := map[string]any{
		"op": discordGatewayOpIdentify,
		"d": map[string]any{
			"token":   s.botToken,
			"intents": discordGatewayIntentGuilds | discordGatewayIntentGuildMembers | discordGatewayIntentGuildPresences,
			"properties": map[string]string{
				"os":      "linux",
				"browser": "amy-world",
				"device":  "amy-world",
			},
		},
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.WriteJSON(payload)
}

func heartbeatGateway(conn *websocket.Conn, writeMu *sync.Mutex, interval time.Duration, lastSequence *atomic.Int64, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_ = writeGatewayHeartbeat(conn, writeMu, lastSequence.Load())
		}
	}
}

func writeGatewayHeartbeat(conn *websocket.Conn, writeMu *sync.Mutex, sequence int64) error {
	var d any
	if sequence > 0 {
		d = sequence
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.WriteJSON(map[string]any{
		"op": discordGatewayOpHeartbeat,
		"d":  d,
	})
}

func (s *DiscordMemberSync) handlePresenceUpdate(ctx context.Context, raw json.RawMessage) error {
	var presence discordGatewayPresence
	if err := json.Unmarshal(raw, &presence); err != nil {
		return err
	}
	if presence.GuildID != s.guildID || strings.TrimSpace(presence.User.ID) == "" {
		return nil
	}

	status := normalizeDiscordStatus(presence.Status)
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO discord_member_states (discord_id, roles, discord_status, synced_at)
		 VALUES ($1, '{}', $2, $3)
		 ON CONFLICT (discord_id) DO UPDATE SET
		   discord_status = EXCLUDED.discord_status,
		   synced_at = EXCLUDED.synced_at`,
		presence.User.ID,
		status,
		time.Now().UTC(),
	)
	return err
}

func (s *DiscordMemberSync) handleGuildCreate(ctx context.Context, raw json.RawMessage) error {
	var guild discordGatewayGuildCreate
	if err := json.Unmarshal(raw, &guild); err != nil {
		return err
	}
	if guild.ID != s.guildID {
		return nil
	}
	for _, presence := range guild.Presences {
		presence.GuildID = guild.ID
		payload, err := json.Marshal(presence)
		if err != nil {
			return err
		}
		if err := s.handlePresenceUpdate(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

func normalizeDiscordStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "idle", "dnd", "offline":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "unknown"
	}
}

type discordSyncStatusError struct {
	status int
	body   string
}

func (e *discordSyncStatusError) Error() string {
	if e.body == "" {
		return "discord api status " + http.StatusText(e.status)
	}
	return "discord api status " + http.StatusText(e.status) + ": " + e.body
}
