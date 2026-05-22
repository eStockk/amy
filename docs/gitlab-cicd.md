# GitLab CI/CD

Pipeline file: `.gitlab-ci.yml`.

## What It Does

- Runs backend tests with Go 1.23.
- Builds the Nuxt frontend with Node 22.
- Validates `docker-compose.yml`.
- Deploys `main` to the VPS over SSH.
- Keeps the server `.env` file intact.
- Rebuilds and restarts Docker Compose services on deploy.

## Required GitLab CI/CD Variables

Add these in:

`GitLab -> Project -> Settings -> CI/CD -> Variables`

| Variable | Example | Notes |
| --- | --- | --- |
| `DEPLOY_HOST` | `5.83.140.117` | VPS IP address |
| `DEPLOY_USER` | `root` | SSH user |
| `DEPLOY_PATH` | `/opt/amy/app` | Optional, defaults to this path |
| `SSH_PRIVATE_KEY` | private deploy key | Must match a public key in `/root/.ssh/authorized_keys` |

Mark `SSH_PRIVATE_KEY` as masked and protected if the project allows it.

## Create A Deploy SSH Key

Create a key locally:

```bash
ssh-keygen -t ed25519 -C "amy-gitlab-ci" -f amy_gitlab_ci
```

Add the public key to the server:

```bash
ssh root@5.83.140.117 "mkdir -p ~/.ssh && chmod 700 ~/.ssh"
cat amy_gitlab_ci.pub | ssh root@5.83.140.117 "cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
```

Put the private key content into GitLab variable `SSH_PRIVATE_KEY`:

```bash
cat amy_gitlab_ci
```

Do not commit this private key.

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
