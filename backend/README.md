# Backend (Go + MongoDB)

## Requirements
- Go 1.22+
- MongoDB (local or via Docker)

## Environment
Copy `.env.example` to `.env` and adjust as needed:

- `PORT` (default 8080)
- `MONGO_URI` (default mongodb://localhost:27017)
- `MONGO_DB` (default minecraft)

## Run
```bash
cd backend
go run ./cmd/server
```