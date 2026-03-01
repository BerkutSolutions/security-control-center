# Upgrade / Rollback

This document describes a recommended upgrade discipline for Berkut SCC in a self-hosted environment.

## Before upgrade

1) Ensure you have a recent `.bscc` backup:
- **Backups** tab → create a backup (recommended)

2) Run preflight (admin checks):
- `GET /api/app/preflight` (requires `app.preflight.view`)
  - includes checks for DB/migrations/storage/run mode, plus security-relevant checks (trusted proxies sanity, metrics token, OnlyOffice internal URL policy/reachability).

3) If you run HA (`BERKUT_RUN_MODE=api|worker`):
- upgrade `worker` and `api` sequentially (not simultaneously) to avoid unnecessary parallel startups.

## Upgrade (Docker Compose)

1) Pin an image version (prefer a specific `IMAGE_TAG` instead of `latest`).

2) Start the stack:
```bash
docker compose pull
docker compose up -d --build
```

Note:
- container startup runs migrations (`berkut-migrate`) and then starts the app.

## Optional: backup-before-migrate

You can enable creating a backup before migrations **only when migrations are pending**:
- `BERKUT_UPGRADE_BACKUP_BEFORE_MIGRATE=true`
- `BERKUT_UPGRADE_BACKUP_INCLUDE_FILES=true|false`
- `BERKUT_UPGRADE_BACKUP_LABEL=...` (optional)

## After upgrade checks

- `GET /readyz`
- `GET /api/app/preflight`
- UI `/healthcheck` (one-time post-login page)

## Rollback

Recommended rollback path is restoring from a `.bscc` backup:

1) Stop the stack:
```bash
docker compose down
```

2) Start the previous version (pin `IMAGE_TAG`).

3) Restore a backup via UI (**Backups** → restore) or the corresponding API.

Note:
- a rollback by image tag only (without data restore) is not guaranteed if new migrations were applied.
