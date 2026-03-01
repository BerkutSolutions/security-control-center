# Berkut SCC Documentation (EN)



Documentation version baseline: `1.0.12`



## Sections

1. Architecture: `docs/eng/architecture.md`

2. API: `docs/eng/api.md`

3. Security: `docs/eng/security.md`

4. Tab compatibility (Compat): `docs/eng/compatibility.md` (cheatsheet: `docs/eng/compatibility_cheatsheet.md`)

5. Healthcheck (/healthcheck): `docs/eng/healthcheck.md`

6. Deploy and CI/CD: `docs/eng/deploy.md`

7. Runbook (start and recovery): `docs/eng/runbook.md`

8. Tabs wiki: `docs/eng/wiki/tabs.md`

9. Features wiki: `docs/eng/wiki/features.md`

10. Assets module (MVP): `docs/eng/assets.md`

11. Findings module (MVP): `docs/eng/findings.md`

12. Software module (MVP): `docs/eng/software.md`

13. Current evolution plan: `docs/eng/roadmap.md`

14. Backups (.bscc): `docs/eng/backups.md`

15. HTTPS + OnlyOffice: `docs/eng/https_onlyoffice.md`

16. Reverse-proxy + OnlyOffice compose example: `docs/ru/docker-compose.https.yml`

17. Monitoring notification message template: `docs/eng/monitoring_notifications_message_template.md`



## Context

Documentation is aligned with current runtime reality:

- PostgreSQL runtime

- goose migrations

- cleanenv configuration

- server-side zero-trust authorization

- `.bscc` backups module (create/import/download/restore/plan/scheduler/retention)

- monitoring SLA module (SLA tab, closed periods, background evaluator, incident policy)



## Included for 1.0.12

- UI navigation: Registries tab (`/registry/...`) now contains Assets/Software/Findings as internal tabs with path routes (e.g. `/registry/assets`, `/registry/software`, `/registry/findings`).

- Settings: dedicated Cleanup tab with selective per-module data cleanup.

- Healthcheck: one-time post-login page `/healthcheck` (available only immediately after login/password change) with basic probes and per-tab Compat report.

- Compat: `/api/app/compat` plus background jobs `/api/app/jobs*` for user-driven Partial adapt / Full reset (no auto migrations/resets).

- Monitoring: server-side flag to auto-close incidents when monitor recovers (`DOWN -> UP`).

- Monitoring engine: deterministic due jitter, scheduled retries (single-attempt, no `sleep` in slots), and `GET /api/monitoring/engine/stats` for diagnostics (inflight/due/skipped, p95 start wait/attempt duration, error kinds).

- Localization and UX: fixes for logs/monitoring UI alignment and missing i18n labels.

- Backups: improved “New backup options” UX and hardened DB restore pipeline (`pg_restore`).

- Compose/runtime: unified container timezone via `TZ` (recommended `Europe/Moscow`).

- Monitoring notifications: hardened deliveries history handling for legacy/new records to avoid intermittent `500` in `/api/monitoring/notifications/deliveries`.

