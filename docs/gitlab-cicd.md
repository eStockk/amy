# GitLab CI/CD

Pipeline file: `.gitlab-ci.yml`.

## What It Does

- Runs backend tests with Go 1.23.
- Builds the Nuxt frontend with Node 22.
- Validates `docker-compose.yml`.
- Deploys `main` from a self-hosted GitLab Runner Docker container on the VPS.
- Keeps the server `.env` file intact.
- Rebuilds and restarts Docker Compose services on deploy.

## GitLab Runner Container On VPS

The runner is registered as:

```text
amy-vps-docker
```

With the new GitLab runner authentication tokens, tags and "run untagged jobs" are controlled in the GitLab UI when the runner is created. The pipeline intentionally does not require tags, so it can run on this project runner without extra YAML changes.

The runner uses Docker executor and mounts:

| Host path | Job container path | Purpose |
| --- | --- | --- |
| `/var/run/docker.sock` | `/var/run/docker.sock` | Let deploy job control host Docker |
| `/opt/amy/app` | `/deploy/amy` | Deploy target workspace |
| runner cache volume | `/cache` | GitLab Runner cache |

The pipeline uses `COMPOSE_PROJECT_NAME=app` to keep existing Docker volumes and container names compatible with the previous manual deployment.

## Register Runner Manually

Use a GitLab runner authentication token from:

`GitLab -> Project -> Settings -> CI/CD -> Runners -> New project runner`

```bash
docker volume create gitlab-runner-config

docker run --rm \
  -v gitlab-runner-config:/etc/gitlab-runner \
  gitlab/gitlab-runner:alpine register \
  --non-interactive \
  --url "https://gitlab.com" \
  --token "YOUR_RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image "alpine:3.20" \
  --name "amy-vps-docker" \
  --docker-pull-policy "if-not-present" \
  --docker-volumes "/cache" \
  --docker-volumes "/var/run/docker.sock:/var/run/docker.sock" \
  --docker-volumes "/opt/amy/app:/deploy/amy"

docker run -d \
  --name gitlab-runner \
  --restart always \
  -v gitlab-runner-config:/etc/gitlab-runner \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gitlab/gitlab-runner:alpine
```

Do not commit runner tokens.

## Server Requirements

The VPS must have:

```bash
apt update
apt install -y docker.io docker-compose
```

The production env file must exist and stay on the server:

```bash
cd /opt/amy/app
nano .env
```

The deploy job excludes `.env`, so secrets are not overwritten by CI.
