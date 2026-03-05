# Compatibility Policy

This policy defines release compatibility rules and operator actions.

## Supported upgrade paths

1. `latest-1 -> latest` must always be supported.
2. Multi-minor jumps require staged intermediate upgrades.

## Principles

1. No silent destructive actions.
2. Any reset action is manual via Compat/jobs.
3. Compatibility actions must be auditable.
4. Server-side permission checks are mandatory:
   - `app.compat.view`
   - `app.compat.manage.partial`
   - `app.compat.manage.full`

## Per-release contract

Every release must document:

1. DB schema changes.
2. New mandatory ENV settings.
3. API/UI compatibility impact.
4. Preflight requirements.
5. Rollback conditions.

## Upgrade stop criteria

Upgrade must stop when:

1. `preflight` has critical baseline failures.
2. Backup is missing or invalid.
3. Tab compatibility is `incompatible` without an approved operator plan.
