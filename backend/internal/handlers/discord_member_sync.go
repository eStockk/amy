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
	"time"
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
				 VALUES ($1, $2, 'unknown', $3)
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
