package db

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		databaseURL = "postgres://amy:amy@localhost:5432/amy?sslmode=disable"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS discord_users (
			discord_id TEXT PRIMARY KEY,
			username TEXT NOT NULL DEFAULT '',
			global_name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			avatar TEXT NOT NULL DEFAULT '',
			rp_first_name TEXT NOT NULL DEFAULT '',
			rp_last_name TEXT NOT NULL DEFAULT '',
			last_seen_at TIMESTAMPTZ,
			presence_active BOOLEAN NOT NULL DEFAULT FALSE,
			acceptance_status TEXT NOT NULL DEFAULT 'pending',
			first_authenticated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS rp_applications (
			id TEXT PRIMARY KEY,
			discord_id TEXT NOT NULL REFERENCES discord_users(discord_id) ON DELETE CASCADE,
			nickname TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			rp_name TEXT NOT NULL DEFAULT '',
			birth_date TEXT NOT NULL,
			race TEXT NOT NULL,
			gender TEXT NOT NULL,
			height_cm INTEGER NOT NULL DEFAULT 170,
			skills TEXT NOT NULL,
			plan TEXT NOT NULL,
			biography TEXT NOT NULL,
			prison_reason TEXT NOT NULL DEFAULT '',
			skin_url TEXT NOT NULL,
			status TEXT NOT NULL,
			moderation_token TEXT NOT NULL,
			discord_message_id TEXT NOT NULL DEFAULT '',
			moderated_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS rp_applications_discord_id_created_at_idx ON rp_applications(discord_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS rp_applications_status_updated_at_idx ON rp_applications(status, updated_at DESC)`,
		`DROP TABLE IF EXISTS minecraft_verification_codes`,
		`ALTER TABLE discord_users DROP COLUMN IF EXISTS linked_minecraft`,
		`ALTER TABLE discord_users DROP COLUMN IF EXISTS minecraft_verified_at`,
		`CREATE TABLE IF NOT EXISTS support_tickets (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			discord_nick TEXT NOT NULL DEFAULT '',
			owner_discord_id TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			moderation_token TEXT NOT NULL DEFAULT '',
			discord_message_id TEXT NOT NULL DEFAULT '',
			discord_channel_id TEXT NOT NULL DEFAULT '',
			resolved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS discord_nick TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS owner_discord_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open'`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS moderation_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS discord_message_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS discord_channel_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS support_tickets_status_created_at_idx ON support_tickets(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS support_tickets_owner_created_at_idx ON support_tickets(owner_discord_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS support_tickets_discord_message_id_idx ON support_tickets(discord_message_id)`,
		`CREATE TABLE IF NOT EXISTS support_ticket_messages (
			id BIGSERIAL PRIMARY KEY,
			ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
			author_type TEXT NOT NULL,
			author_name TEXT NOT NULL DEFAULT '',
			author_discord_id TEXT NOT NULL DEFAULT '',
			author_discord_status TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL,
			discord_message_id TEXT NOT NULL DEFAULT '',
			read_by_user BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS support_ticket_messages_ticket_created_at_idx ON support_ticket_messages(ticket_id, created_at ASC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS support_ticket_messages_discord_message_id_uq ON support_ticket_messages(discord_message_id) WHERE discord_message_id <> ''`,
		`CREATE TABLE IF NOT EXISTS support_ticket_attachments (
			id BIGSERIAL PRIMARY KEY,
			ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
			message_id BIGINT NOT NULL REFERENCES support_ticket_messages(id) ON DELETE CASCADE,
			file_name TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,
			storage_path TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS support_ticket_attachments_ticket_idx ON support_ticket_attachments(ticket_id, created_at ASC)`,
		`CREATE TABLE IF NOT EXISTS support_push_subscriptions (
			id BIGSERIAL PRIMARY KEY,
			discord_id TEXT NOT NULL REFERENCES discord_users(discord_id) ON DELETE CASCADE,
			endpoint TEXT NOT NULL,
			p256dh TEXT NOT NULL,
			auth TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(discord_id, endpoint)
		)`,
		`CREATE INDEX IF NOT EXISTS support_push_subscriptions_discord_id_idx ON support_push_subscriptions(discord_id)`,
		`CREATE TABLE IF NOT EXISTS players (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS news (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			intro TEXT NOT NULL,
			tags TEXT[] NOT NULL DEFAULT '{}',
			source TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			variant TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS discord_member_states (
			discord_id TEXT PRIMARY KEY,
			username TEXT NOT NULL DEFAULT '',
			global_name TEXT NOT NULL DEFAULT '',
			nick TEXT NOT NULL DEFAULT '',
			roles TEXT[] NOT NULL DEFAULT '{}',
			discord_status TEXT NOT NULL DEFAULT 'unknown',
			synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE discord_member_states ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE discord_member_states ADD COLUMN IF NOT EXISTS global_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE discord_member_states ADD COLUMN IF NOT EXISTS nick TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS discord_member_states_synced_at_idx ON discord_member_states(synced_at DESC)`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}
