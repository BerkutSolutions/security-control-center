# Wiki: feature areas

## Authentication
Login/logout/me, cookie sessions, CSRF, password policy.

## Authorization
RBAC + ACL + classification/clearance, server-side zero-trust checks.

## Audit
Critical action logging in `audit_log`.

## i18n
RU/EN localization via `gui/static/i18n/*.json`.

## HTTPS and network
Reverse proxy (recommended) or built-in TLS.
Trusted proxies and HTTPS config audit trail.

## Storage and encryption
PostgreSQL runtime + encrypted sensitive content/attachments + persistent volumes.

## Import/export/conversion
Document/report import-export and local converter pipeline.

## Monitoring and SLA
- Monitors, metrics, events, maintenance windows, and notification channels.
- SLA tab with coverage-aware `24h/7d/30d` windows.
- Closed periods `day/week/month` and delayed SLA incident creation on period close.

## Backups (.bscc)
- Create/import/download/delete backup flows.
- Dry-run/restore with progress steps.
- Auto-backup scheduler and retention policy.

## Deploy and CI/CD
Docker/Compose workflow, verify pipeline, rollback.
