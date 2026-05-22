# Minecraft Server Project (Nuxt + Go + PostgreSQL)

## Structure
- `frontend` - Nuxt app
- `backend` - Go API
- `docker-compose.yml` - PostgreSQL, app services, Prometheus, Grafana

## Quick start
```bash
# Full stack
docker compose up -d --build

# Or run services manually
cd backend
go run ./cmd/server

cd frontend
npm install
npm run dev
```

Metrics:
- Backend: `http://localhost:8080/metrics`
- Frontend: `http://localhost:3000/metrics`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001`
