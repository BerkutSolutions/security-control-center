# Deploy and CI/CD

## Docker stack
- App image: `docker/Dockerfile`
- Compose stack: `docker-compose.yaml`
- Services: `berkut` (app), `postgres` (DB)
- Persistent volumes: `berkut_data`, `berkut_pgdata`

## Environment variables
Core:
- `ENV`, `APP_ENV`, `APP_CONFIG`, `PORT`, `DATA_PATH`

Secrets:
- `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`

DB/runtime:
- `BERKUT_DB_DRIVER=postgres`
- `BERKUT_DB_URL=postgres://...`
- other `BERKUT_*` overrides as needed

HA / Scaling:
- Run mode: `BERKUT_RUN_MODE=all|api|worker` (`all` by default).
- Recommended HA layout: multiple `api` replicas (HTTP/UI/API only) + a single `worker` replica (background workers).

Observability:
- Liveness: `GET /healthz`
- Readiness (DB ping): `GET /readyz`
- Prometheus metrics (disabled by default): `BERKUT_METRICS_ENABLED=true` and `GET /metrics` (use `Authorization: Bearer $BERKUT_METRICS_TOKEN` if token is set)
- (home/dev only) allow `/metrics` without token: `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=true`

Passkeys (WebAuthn):
- Enable/disable: `BERKUT_WEBAUTHN_ENABLED=true|false`
- RP ID / origins (recommended for prod):
  - `BERKUT_WEBAUTHN_RP_ID=scc.example.com`
  - `BERKUT_WEBAUTHN_ORIGINS=https://scc.example.com`
  - `BERKUT_WEBAUTHN_RP_NAME=SCC`
- Requirements: HTTPS (or `localhost`).

Upgrade:
- Preflight: `GET /api/app/preflight` (permission `app.preflight.view`)
- Backup-before-migrate (optional): `BERKUT_UPGRADE_BACKUP_BEFORE_MIGRATE=true`

## Typical deploy
```bash
cp .env.example .env
docker compose pull
docker compose up -d --build
```

Note:
- Container startup runs `berkut-migrate` first, then starts `berkut-scc`.
- For non-Docker runs apply migrations explicitly before app start:
```bash
make migrate
go run ./main.go
```

## Rollback
1. Pin previous `IMAGE_TAG`.
2. Restart app service only:
```bash
docker compose up -d --no-deps berkut
```

## CI
`.gitlab-ci.yml` includes a verification stage (`verify-go`) before image build/publish.
