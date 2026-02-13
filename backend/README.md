# Backend (Go + MongoDB)

## Requirements
- Go 1.22+
- MongoDB (local or via Docker)

## Environment
Copy `.env.example` to `.env` and set values:

- `PORT` - API port (default `8080`)
- `MONGO_URI` - Mongo connection string
- `MONGO_DB` - database name
- `FRONTEND_URL` - frontend origin (`https://amy-world.ru` in production)
- `DISCORD_CLIENT_ID` - Discord OAuth2 Client ID
- `DISCORD_CLIENT_SECRET` - Discord OAuth2 Client Secret
- `DISCORD_REDIRECT_URL` - callback URL, must match Discord app settings
- `DISCORD_TICKET_WEBHOOK` - webhook for support tickets
- `DISCORD_RP_WEBHOOK` - webhook for RP applications moderation channel
- `MINECRAFT_SERVER_TOKEN` - shared secret for Minecraft server <-> website API

## Run
```bash
cd backend
go run ./cmd/server
```

## Main API routes
- `GET /api/auth/discord/start` - start Discord OAuth
- `GET /api/auth/discord/callback` - OAuth callback
- `GET /api/auth/me` - current authenticated user
- `POST /api/auth/logout` - logout
- `POST /api/auth/verify-minecraft` - verify website account with one-time code
- `POST /api/rp/applications` - submit RP application
- `GET /api/rp/applications/{id}/moderate?action=approve|reject&token=...` - moderation endpoint for Discord buttons
- `POST /api/minecraft/verification-code` - generate/reuse verification code (server token required)
- `POST /api/minecraft/rp-name` - sync RP first/last name from server plugin (server token required)
