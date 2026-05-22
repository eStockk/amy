# Deploy From GitHub To Server

## 1. Prepare the server

Install Docker, Docker Compose, Git, and OpenSSH client.

```bash
sudo apt update
sudo apt install -y git docker.io docker-compose openssh-client
sudo usermod -aG docker "$USER"
```

Log out and back in after adding the user to the `docker` group.

## 2. Create a deploy SSH key

```bash
ssh-keygen -t ed25519 -C "amy-deploy" -f ~/.ssh/amy_deploy
cat ~/.ssh/amy_deploy.pub
```

Add the public key in GitHub:

`Repository -> Settings -> Deploy keys -> Add deploy key`

Use read-only access unless the server must push changes back.

## 3. Clone the repository

```bash
mkdir -p /opt/amy
cd /opt/amy
GIT_SSH_COMMAND='ssh -i ~/.ssh/amy_deploy -o IdentitiesOnly=yes' \
  git clone git@github.com:OWNER/REPOSITORY.git app
cd app
```

Replace `OWNER/REPOSITORY` with the real GitHub repository path.

## 4. Configure environment

```bash
cp .env.example .env
nano .env
```

Set production values for:

- `FRONTEND_URL`
- `NUXT_PUBLIC_API_BASE`
- Discord OAuth and webhook variables
- `MINECRAFT_SERVER_TOKEN`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `DATABASE_URL`
- `GRAFANA_ADMIN_USER`
- `GRAFANA_ADMIN_PASSWORD`

## 5. Build and start

```bash
docker-compose -f docker-compose.yml up -d --build
docker-compose -f docker-compose.yml ps
```

## 6. Update from GitHub

```bash
cd /opt/amy/app
GIT_SSH_COMMAND='ssh -i ~/.ssh/amy_deploy -o IdentitiesOnly=yes' git pull --ff-only
docker-compose -f docker-compose.yml up -d --build
docker image prune -f
```

## 7. Useful checks

```bash
docker-compose -f docker-compose.yml logs --tail=100 backend
docker-compose -f docker-compose.yml logs --tail=100 frontend
docker-compose -f docker-compose.yml exec postgres psql -U "$POSTGRES_USER" -d amy -c '\dt'
```

Open:

- Frontend: `http://SERVER_IP:3000`
- Backend health: `http://SERVER_IP:8080/api/health`
- Prometheus: `http://SERVER_IP:9090/targets`
- Grafana: `http://SERVER_IP:3001`
