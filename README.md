# Minecraft Server Project (Nuxt + Go + MongoDB)

## Structure
- `frontend` - Nuxt app
- `backend` - Go API
- `docker-compose.yml` - MongoDB service

## Quick start
```bash
# MongoDB
docker compose up -d

# Backend
cd backend
go run ./cmd/server

# Frontend
cd frontend
npm install
npm run dev
```
