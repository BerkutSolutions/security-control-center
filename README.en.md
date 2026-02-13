# Berkut Solutions - Security Control Center

<p align="center">
  <img src="gui/static/logo.png" alt="Berkut SCC logo" width="220">
</p>

[Русская версия](README.md)

Berkut Solutions - Security Control Center is a self-hosted security and compliance platform implemented as a Go monolith with embedded UI.

Core principles:
- server-side zero-trust permission checks on every endpoint
- local deployment model without external CDN dependencies
- auditability of critical operations
- predictable operations with Docker/Docker Compose and CI

## What Is Included
- Documents, approvals, and templates
- Incidents and reporting
- Tasks (spaces/boards/tasks)
- Monitoring and notifications
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

## Development Checks
```bash
make ci
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

## Security Notes
- Do not use default secrets outside development.
- Authorization is enforced server-side for every endpoint.
- Restrict `BERKUT_SECURITY_TRUSTED_PROXIES` to trusted proxy ranges only.
- Use reverse-proxy TLS termination in production.
