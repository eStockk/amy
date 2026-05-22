# VPS Access After Hardening

After the security hardening, only these ports should be public:

- `22/tcp` SSH
- `80/tcp` HTTP for Caddy/Let's Encrypt
- `443/tcp` HTTPS for Caddy

Internal services are bound to `127.0.0.1` on the VPS:

| Service | VPS-local URL |
| --- | --- |
| Frontend SSR | `http://127.0.0.1:3000` |
| Backend API | `http://127.0.0.1:8080` |
| PostgreSQL | `127.0.0.1:5432` |
| Prometheus | `http://127.0.0.1:9090` |
| Grafana | `http://127.0.0.1:3001` |

## Open Grafana Locally

```bash
ssh -L 3001:127.0.0.1:3001 root@5.83.140.117
```

Then open:

```text
http://127.0.0.1:3001
```

## Open Prometheus Locally

```bash
ssh -L 9090:127.0.0.1:9090 root@5.83.140.117
```

Then open:

```text
http://127.0.0.1:9090
```

## Connect To PostgreSQL Locally

```bash
ssh -L 5432:127.0.0.1:5432 root@5.83.140.117
```

Then connect a DB client to:

```text
host=127.0.0.1 port=5432
```

## Check Services On The VPS

```bash
cd /opt/amy/app
docker-compose -p app -f docker-compose.yml ps
curl -fsS http://127.0.0.1:8080/api/health
curl -fsS http://127.0.0.1:3000/metrics | head
```

`docker-compose` on the VPS is a wrapper around Docker Compose v2, so keep using it from `/opt/amy/app`.
