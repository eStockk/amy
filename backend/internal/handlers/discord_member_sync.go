package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type DiscordMemberSync struct {
	db                 *sql.DB
	botToken           string
	guildID            string
	ticketChannelID    string
	notifySupportReply func(context.Context, int64, string, string)
	httpClient         *http.Client
}

type discordGuildRole struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    int    `json:"color"`
	Position int    `json:"position"`
}

type discordGuildMember struct {
	User struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
	} `json:"user"`
	Nick  string   `json:"nick"`
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

type discordGatewayMessageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Bot        bool   `json:"bot"`
	} `json:"author"`
	Member *struct {
		Nick string `json:"nick"`
	} `json:"member"`
	MessageReference *struct {
		MessageID string `json:"message_id"`
		ChannelID string `json:"channel_id"`
	} `json:"message_reference"`
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
	discordGatewayIntentGuildMessages  = 1 << 9
	discordGatewayIntentMessageContent = 1 << 15
)

func NewDiscordMemberSync(db *sql.DB, botToken, guildID, ticketChannelID string, notifySupportReply func(context.Context, int64, string, string)) *DiscordMemberSync {
	return &DiscordMemberSync{
		db:                 db,
		botToken:           strings.TrimSpace(botToken),
		guildID:            strings.TrimSpace(guildID),
		ticketChannelID:    strings.TrimSpace(ticketChannelID),
		notifySupportReply: notifySupportReply,
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
			roleIDs := make([]string, 0, len(member.Roles))
			for _, roleID := range member.Roles {
				if name := roles[roleID]; name != "" && name != "@everyone" {
					roleNames = append(roleNames, name)
					roleIDs = append(roleIDs, roleID)
				}
			}

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO discord_member_states (discord_id, username, global_name, nick, roles, role_ids, discord_status, synced_at)
				 VALUES ($1, $2, $3, $4, $5, $6, 'offline', $7)
				 ON CONFLICT (discord_id) DO UPDATE SET
				   username = EXCLUDED.username,
				   global_name = EXCLUDED.global_name,
				   nick = EXCLUDED.nick,
				   roles = EXCLUDED.roles,
				   role_ids = EXCLUDED.role_ids,
				   synced_at = EXCLUDED.synced_at`,
				discordID,
				strings.TrimSpace(member.User.Username),
				strings.TrimSpace(member.User.GlobalName),
				strings.TrimSpace(member.Nick),
				roleNames,
				roleIDs,
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
			if message.T == "MESSAGE_CREATE" {
				if err := s.handleSupportTicketReply(ctx, message.D); err != nil && ctx.Err() == nil {
					log.Printf("discord support reply sync failed: %v", err)
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
			"intents": discordGatewayIntentGuilds | discordGatewayIntentGuildMembers | discordGatewayIntentGuildPresences | discordGatewayIntentGuildMessages | discordGatewayIntentMessageContent,
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

func (s *DiscordMemberSync) handleSupportTicketReply(ctx context.Context, raw json.RawMessage) error {
	var message discordGatewayMessageCreate
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	if message.Author.Bot || strings.TrimSpace(message.Author.ID) == "" || strings.TrimSpace(message.Content) == "" {
		return nil
	}
	if s.ticketChannelID != "" && message.ChannelID != s.ticketChannelID {
		return nil
	}
	var ticketID int64
	var ticketOwner string
	content := strings.TrimSpace(message.Content)
	adminMessage := content
	var err error
	if message.MessageReference != nil && strings.TrimSpace(message.MessageReference.MessageID) != "" {
		err = s.db.QueryRowContext(
			ctx,
			`SELECT id, owner_discord_id
			 FROM support_tickets
			 WHERE discord_message_id = $1
			 LIMIT 1`,
			strings.TrimSpace(message.MessageReference.MessageID),
		).Scan(&ticketID, &ticketOwner)
	} else if matched := supportTicketPrefix.FindStringSubmatch(content); len(matched) == 3 {
		ticketID, _ = strconv.ParseInt(matched[1], 10, 64)
		adminMessage = strings.TrimSpace(matched[2])
		err = s.db.QueryRowContext(ctx, `SELECT owner_discord_id FROM support_tickets WHERE id = $1`, ticketID).Scan(&ticketOwner)
	} else {
		return nil
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if adminMessage == "" {
		return nil
	}

	authorName := strings.TrimSpace(message.MemberNick())
	status := "unknown"
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(discord_status, ''), 'unknown') FROM discord_member_states WHERE discord_id = $1`, message.Author.ID).Scan(&status)

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO support_ticket_messages
		 (ticket_id, author_type, author_name, author_discord_id, author_discord_status, message, discord_message_id, read_by_user, created_at)
		 VALUES ($1, 'admin', $2, $3, $4, $5, $6, FALSE, $7)
		 ON CONFLICT (discord_message_id) WHERE discord_message_id <> '' DO NOTHING`,
		ticketID,
		authorName,
		message.Author.ID,
		status,
		adminMessage,
		strings.TrimSpace(message.ID),
		time.Now().UTC(),
	)
	_ = ticketOwner
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected > 0 && s.notifySupportReply != nil {
		s.notifySupportReply(ctx, ticketID, authorName, adminMessage)
	}
	return nil
}

func (m discordGatewayMessageCreate) MemberNick() string {
	if m.Member != nil && strings.TrimSpace(m.Member.Nick) != "" {
		return strings.TrimSpace(m.Member.Nick)
	}
	if strings.TrimSpace(m.Author.GlobalName) != "" {
		return strings.TrimSpace(m.Author.GlobalName)
	}
	return strings.TrimSpace(m.Author.Username)
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
