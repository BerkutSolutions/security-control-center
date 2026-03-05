# Upgrade Playbook

Production upgrade runbook.

## 1. Preflight

1. Run `GET /api/app/preflight`.
2. Ensure critical baseline checks have no `failed` status.
3. Attach preflight output to change request.

## 2. Backup

1. Create backup before migrations (`backup-before-migrate`).
2. Validate backup integrity (dry-run restore or integrity check).
3. Record backup ID in change request.

## 3. Migrate

1. Apply target-version migrations.
2. Monitor migration logs and duration.

## 4. Smoke checks

Verify:

1. Login/session/CSRF flows.
2. Role-protected API access.
3. `GET /healthz`, `GET /readyz`.
4. Core tabs (Docs/Monitoring/Incidents/Backups/Logs).

## 5. Rollback decision

Rollback is mandatory when:

1. Smoke checks do not pass within the change window.
2. Critical functions are unavailable.
3. Data or audit integrity is compromised.

## 6. Post-upgrade

1. Export `GET /api/logs/export/package` as release evidence.
2. Update `CHANGELOG.md` by sections (`UI`, `Core`, `Security`, `Infrastructure`, `Tabs/Modules`).
