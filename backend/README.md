# Backend (Go + PostgreSQL)

## Requirements
- Go 1.22+
- PostgreSQL 16+ or Docker Compose from the repository root

## Environment
Copy `.env.example` to `.env` and set values:

- `PORT` - API port (default `8080`)
- `DATABASE_URL` - PostgreSQL connection string
- `FRONTEND_URL` - frontend origin (`https://amy-world.ru` in production)
- `DISCORD_CLIENT_ID` - Discord OAuth2 Client ID
- `DISCORD_CLIENT_SECRET` - Discord OAuth2 Client Secret
- `DISCORD_REDIRECT_URL` - callback URL, must match Discord app settings
- `DISCORD_TICKET_WEBHOOK` - webhook for support tickets
- `DISCORD_RP_WEBHOOK` - webhook for RP applications moderation channel
- `DISCORD_RP_MODERATOR_IDS` - comma-separated Discord IDs allowed to moderate RP applications
- `DISCORD_TICKET_CHANNEL_ID` - Discord channel ID where admins reply to support tickets
- `DISCORD_BOT_TOKEN` and `DISCORD_GUILD_ID` - optional Discord bot access for member role, presence and support reply sync
- `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` - optional browser push keys for support notifications
- `SUPPORT_PUSH_SUBJECT` - contact subject for Web Push, for example `mailto:support@amyworld.ru`
- `SUPPORT_STORAGE_DIR` - directory for support ticket HTML history and uploaded images

## Run
```bash
cd backend
go run ./cmd/server
```

The backend creates or updates its PostgreSQL tables on startup.

## Main API routes
- `GET /api/health` - backend and database health
- `GET /metrics` - Prometheus metrics
- `GET /api/auth/discord/start` - start Discord OAuth
- `GET /api/auth/discord/callback` - OAuth callback
- `GET /api/auth/me` - current authenticated user
- `POST /api/auth/logout` - logout
- `POST /api/rp/applications` - submit RP application
- `DELETE /api/rp/applications/{id}` - delete own RP ticket (site + Discord message)
- `GET /api/rp/applications/{id}/moderate?action=accept|call|cancel|reconsider&token=...` - moderation endpoint for Discord buttons
- `GET /api/support/tickets` - list current user's support tickets
- `POST /api/support/tickets` - create support ticket
- `GET /api/support/tickets/{id}/messages` - load ticket chat
- `POST /api/support/tickets/{id}/messages` - add a user message to ticket chat
- `GET /api/support/tickets/{id}/attachments/{attachmentId}` - load a saved ticket image
- `GET|POST|DELETE /api/support/notifications` - manage browser push notification subscription
