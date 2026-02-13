# Berkut Solutions - Security Control Center

<p align="center">
  <img src="gui/static/logo.png" alt="Berkut SCC logo" width="220">
</p>

[Русская версия](README.md)

Berkut Solutions - Security Control Center is a self-hosted security and compliance platform implemented as a Go monolith with embedded UI.

Current version: `1.0.3`

Core principles:
- server-side zero-trust permission checks on every endpoint
- local deployment model without external CDN dependencies
- auditability of critical operations
- predictable operations with Docker/Docker Compose and CI

## What Is Included
- Documents, approvals, and templates
- Incidents and reporting
- Tasks (spaces/boards/tasks)
- Monitoring, SLA, and notifications
- Users, roles, and groups management
- Audit log

## Current Architecture
- Backend: Go `1.23`
- Database: PostgreSQL (production runtime)
- Migrations: `goose`
- Configuration: `cleanenv` + `config/app.yaml` + ENV
- RBAC: Casbin
- UI: embedded static assets (`gui/static`), RU/EN i18n
- Routing: modular `chi` stack

## Quick Start
```bash
cp .env.example .env
docker compose up -d --build
```

Open: `http://localhost:8080`

## Useful Commands
Apply migrations explicitly (separate from app startup):
```bash
make migrate
```

Restart stack:
```bash
docker compose down
docker compose up -d --build
```

Recreate all data volumes (destructive):
```bash
docker compose down -v
docker compose up -d --build
```

Logs:
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## Configuration
- `.env` — runtime env file
- `.env.example` — template
- `config/app.yaml` — base app config
- `docker-compose.yaml` — main compose stack
- `docker/Dockerfile` — application image

Required secrets for non-dev environments:
- `CSRF_KEY`
- `PEPPER`
- `DOCS_ENCRYPTION_KEY`
- `BACKUP_ENCRYPTION_KEY` (or `BERKUT_BACKUP_ENCRYPTION_KEY`)

## Backups (.bscc)
- Backup storage path: `BERKUT_BACKUP_PATH` (default `data/backups`, typically `/app/data/backups` in container).
- Format: one encrypted `.bscc` file (manifest + checksums + db dump).
- Encryption key is required in non-dev: `BACKUP_ENCRYPTION_KEY`/`BERKUT_BACKUP_ENCRYPTION_KEY`.
- Supported flows: create/import/download/delete, dry-run/restore with progress, maintenance mode, auto-backups and retention.
- Manual run supports `label`, `scope`, `include_files`.
- Schedule presets: daily, weekly, monthly (start/end of month).

Primary ENV:
- `BERKUT_BACKUP_PATH` — backups storage path.
- `BERKUT_BACKUP_PGDUMP_BIN` — `pg_dump` binary path (default `pg_dump`).
- `BERKUT_BACKUP_MAX_PARALLEL` — max parallel backup jobs.
- `BERKUT_BACKUP_UPLOAD_MAX_BYTES` — upload size limit for `.bscc` import.

Create backup manually (API):
```bash
curl -X POST http://localhost:8080/api/backups \
  -H "Cookie: berkut_session=..." \
  -H "X-CSRF-Token: ..."
```

Restore:
- Dry-run: `POST /api/backups/{id}/restore/dry-run`
- Real restore: `POST /api/backups/{id}/restore`
- Progress: `GET /api/backups/restores/{restore_id}`

Maintenance mode:
- Real restore enables maintenance mode.
- Regular APIs return `503` while restore status endpoint stays available.
- Maintenance mode is disabled after restore completion.

Docker Compose note:
- Persist both `berkut_data` (contains `DATA_PATH`, including `BACKUP_PATH`) and `berkut_pgdata`.

## Monitoring and SLA
- Monitoring tabs: `/monitoring`, `/monitoring/events`, `/monitoring/sla`, `/monitoring/maintenance`, `/monitoring/certs`, `/monitoring/notifications`, `/monitoring/settings`.
- SLA uses a coverage-aware model with `24h/7d/30d` windows.
- Closed periods `day/week/month` are evaluated asynchronously by background evaluator jobs.
- SLA incidents are created only on selected period close and only when monitor policy enables it.

### Maintenance
- Maintenance page: `/monitoring/maintenance`, placed next to the SLA tab.
- Scheduler strategies: `single`, `cron`, `interval`, `weekday`, `monthday`.
- Each plan stores: title, markdown description, affected monitors, timezone, and active date/time boundaries.
- Maintenance windows are treated as accepted risk: those intervals are excluded from SLA downtime penalties.
- Full lifecycle actions are available: create, edit, stop early, and delete.

## Development Checks
```bash
make ci
```

Build with version override (optional):
```bash
go build -ldflags "-X 'berkut-scc/core/appmeta.AppVersion=1.0.3'" ./...
```

Available targets:
```bash
make fmt
make fmt-check
make vet
make test
make lint
make migrate
```

## Documentation
- Docs index: `docs/README.md`
- Russian docs: `docs/ru/README.md`
- English docs: `docs/eng/README.md`
- RU Backups: `docs/ru/backups.md`
- EN Backups: `docs/eng/backups.md`

## Security Notes
- Do not use default secrets outside development.
- Authorization is enforced server-side for every endpoint.
- Restrict `BERKUT_SECURITY_TRUSTED_PROXIES` to trusted proxy ranges only.
- Use reverse-proxy TLS termination in production.
- Do not store secrets (`BACKUP_ENCRYPTION_KEY`, `DOCS_ENCRYPTION_KEY`, `PEPPER`, `CSRF_KEY`) in git.
