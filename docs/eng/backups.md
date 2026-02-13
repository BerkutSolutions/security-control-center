# Berkut SCC - Backups (.bscc)

## Overview
Backups module in version `1.0.3` supports:
- encrypted `.bscc` creation (DB-only);
- `.bscc` import/upload;
- streaming `.bscc` download;
- restore (dry-run and real) with step-by-step progress;
- backup plan, scheduler, and retention policy;
- selective manual backup (label, scope, include_files);
- zero-trust RBAC and full audit coverage for backup endpoints.

## Configuration
Primary environment variables:
- `BERKUT_BACKUP_PATH` - backup storage directory.
- `BACKUP_ENCRYPTION_KEY` or `BERKUT_BACKUP_ENCRYPTION_KEY` - encryption key.
- `BERKUT_BACKUP_PGDUMP_BIN` - `pg_dump` binary path.
- `BERKUT_BACKUP_MAX_PARALLEL` - max parallel backup jobs.
- `BERKUT_BACKUP_UPLOAD_MAX_BYTES` - import upload size limit.

Important:
- encryption key is mandatory in non-dev;
- never commit secrets to git.

## API (core)
- `GET /api/backups` - list backups.
- `GET /api/backups/{id}` - backup details.
- `POST /api/backups` - create backup (supports `label`, `scope`, `include_files`).
- `POST /api/backups/import` - import `.bscc`.
- `GET /api/backups/{id}/download` - download `.bscc`.
- `DELETE /api/backups/{id}` - delete backup.
- `POST /api/backups/{id}/restore` - start restore.
- `POST /api/backups/{id}/restore/dry-run` - dry-run restore.
- `GET /api/backups/restores/{restore_id}` - restore status and progress steps.
- `GET /api/backups/plan` - current auto-backup plan.
- `PUT /api/backups/plan` - update plan.
- `POST /api/backups/plan/enable` - enable plan.
- `POST /api/backups/plan/disable` - disable plan.

## Maintenance mode
During a real restore, the app enables maintenance mode:
- regular APIs are blocked (`503`);
- restore status endpoint remains available;
- maintenance mode is turned off after restore completion.

## Scheduling and retention
- Schedule presets: daily, weekly, monthly (start/end of month).
- Time of day and weekday are part of the plan.
- After restart, scheduler uses the latest successful auto-backup timestamp and avoids unnecessary immediate runs.
- Retention removes old artifacts by `retention_days` while keeping at least `keep_last_successful` backups.

## Security
- Permissions are enforced server-side for every endpoint.
- API errors are sanitized (no internal paths, stack traces, or raw internals).
- Upload/download/delete flows are hardened (size limits, path validation, concurrency locks, safe error codes).
