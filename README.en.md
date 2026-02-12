# Berkut Solutions - Security Control Center

[Русская версия](README.md)

Berkut Solutions - Security Control Center is a self-hosted platform for security and compliance management, implemented as a Go monolith with embedded UI.

Core goals:
- zero-trust access checks on every API endpoint
- local-first deployment model (no CDN dependencies)
- auditability of critical actions
- predictable operations with Docker and GitLab CI/CD

## Table of Contents
1. [What Is Included](#what-is-included)
2. [Architecture](#architecture)
3. [Quick Start](#quick-start)
4. [Prebuilt Docker Hub Image](#prebuilt-docker-hub-image)
5. [Recovery Commands](#recovery-commands)
6. [Docker Compose](#docker-compose)
7. [Configuration](#configuration)
8. [Documentation Map](#documentation-map)
9. [Security Notes](#security-notes)

## What Is Included
- Task and workflow management (including approvals)
- Monitoring section with tab routing
- RBAC + ACL authorization model
- Audit log for security-sensitive operations
- Embedded web UI served by backend
- SQLite storage with persistent Docker volume

## Architecture
- Backend: Go (1.22+)
- Storage: SQLite
- UI: embedded static frontend assets
- Deployment: Docker / Docker Compose / GitLab CI
- TLS strategy: external reverse proxy is recommended (Nginx/Traefik)

## Quick Start
Local build:
```bash
docker build -t berkut-scc -f docker/Dockerfile .
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

Open: `http://localhost:8080`

## Prebuilt Docker Hub Image
Image repository:
`https://hub.docker.com/repository/docker/berkutsolutions/security-control-center/general`

Run without local build:
```bash
docker pull berkutsolutions/security-control-center:latest
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkutsolutions/security-control-center:latest
```

## Recovery Commands
Fix volume permissions for `readonly database`:
```bash
docker rm -f berkut-scc || true
docker run --rm --user 0 --entrypoint sh -v berkut-data:/app/data berkut-scc -c "chown -R berkut:berkut /app/data"
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

Recreate volume (destructive):
```bash
docker rm -f berkut-scc || true
docker volume rm berkut-data
```

Container logs:
```bash
docker logs -f berkut-scc
```

## Docker Compose
Start:
```bash
docker compose up -d --build
```

Start with reverse proxy profile:
```bash
docker compose --profile proxy up -d
```

## Configuration
- Main env file: `.env`
- Env template: `.env.example`
- App config: `config/app.yaml`
- Dockerfile: `docker/Dockerfile`

## Documentation Map
This README is the high-level overview. Deep-dive documentation is in `docs`.

- Main docs index: `docs/README.md`
- Russian docs: `docs/ru/README.md`
- English docs: `docs/eng/README.md`

Runbooks:
- RU: `docs/ru/runbook.md`
- EN: `docs/eng/runbook.md`

Detailed wiki:
- RU tabs/features: `docs/ru/wiki/tabs.md`, `docs/ru/wiki/features.md`
- EN tabs/features: `docs/eng/wiki/tabs.md`, `docs/eng/wiki/features.md`

## Security Notes
- Do not use default secrets outside development.
- Prefer TLS termination on reverse proxy in production.
- Restrict `BERKUT_SECURITY_TRUSTED_PROXIES` to trusted proxy ranges only.
- Keep audit logging enabled for critical configuration changes.

