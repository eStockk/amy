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
			linked_minecraft TEXT NOT NULL DEFAULT '',
			rp_first_name TEXT NOT NULL DEFAULT '',
			rp_last_name TEXT NOT NULL DEFAULT '',
			minecraft_verified_at TIMESTAMPTZ,
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
		`CREATE TABLE IF NOT EXISTS minecraft_verification_codes (
			code TEXT PRIMARY KEY,
			discord_id TEXT NOT NULL REFERENCES discord_users(discord_id) ON DELETE CASCADE,
			nickname TEXT NOT NULL,
			application_id TEXT NOT NULL REFERENCES rp_applications(id) ON DELETE CASCADE,
			used BOOLEAN NOT NULL DEFAULT FALSE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS verification_codes_discord_id_created_at_idx ON minecraft_verification_codes(discord_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS verification_codes_code_idx ON minecraft_verification_codes(code)`,
		`CREATE TABLE IF NOT EXISTS support_tickets (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			discord_nick TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			moderation_token TEXT NOT NULL DEFAULT '',
			discord_message_id TEXT NOT NULL DEFAULT '',
			resolved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS discord_nick TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open'`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS moderation_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS discord_message_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS support_tickets_status_created_at_idx ON support_tickets(status, created_at DESC)`,
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
			roles TEXT[] NOT NULL DEFAULT '{}',
			discord_status TEXT NOT NULL DEFAULT 'unknown',
			synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS discord_member_states_synced_at_idx ON discord_member_states(synced_at DESC)`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}
