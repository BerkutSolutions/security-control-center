# Assets (MVP spec)

The Assets module is a registry of systems/services and hosts used across SCC domains (incidents, monitoring, reports, docs/links, findings/controls).

## Terminology

- **Asset** — a single entity representing either a *host* or a *system/service/application*.
- MVP goal: a unified catalog + stable identifiers for links, search, and reporting (no external sync in the first stage).

## Data model (fields)

Required:
- `name` — asset name (strict DB uniqueness is added only if explicitly required; MVP relies on search/UX).
- `type` — `host | service | application | network | other`.
- `criticality` — `low | medium | high | critical`.

Recommended for MVP:
- `description` — purpose/notes.
- `commissioned_at` — date (when put into operation).
- `ip_addresses` — list of IPs (IPv4/IPv6), stored as an array; UI allows multi-value input.
- `owner` — owner (MVP: free text; may become user/group reference later).
- `administrator` — administrator (MVP: free text; may become user/group reference later).
- `env` — `prod | stage | dev | test | other`.
- `status` — `active | decommissioned`.
- `tags` — string array.

System fields (consistent with other domains):
- `id` (int64), `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at` (soft-delete/archive).

## RBAC and zero-trust

New permissions (deny-by-default):
- `assets.view` — view list/details/search.
- `assets.manage` — create/update/archive/restore.

Every `/api/assets*` endpoint must enforce permissions server-side. UI/menu is not a security control.

## Menu access (menu_permissions)

- New tab key: `assets`
- UI route: `/registry/assets` (Registries tab). Legacy direct route: `/assets`
- API route group: `/api/assets`

Tab access is additionally limited by `menu_permissions` (same model as other tabs).

## Audit (audit_log)

Key events to log:
- `assets.create`, `assets.update`, `assets.archive`, `assets.restore`.

Additional actions:
- `assets.export.csv`.

## Export and autocomplete

- `GET /api/assets/export.csv` — CSV export (same filters as list; `limit` up to 5000).
- `GET /api/assets/autocomplete` — field suggestions (params: `field=all|owners|administrators|tags`, `q`, `limit`, `include_deleted=1` requires `assets.manage`).

## Integrations (requirements)

Goal: use `asset_id` in relations/filters instead of free text.

Planned integrations (next stages):
- Incidents: incident ↔ assets (many-to-many) + coexist/migration from `meta.assets` (text).
- Monitoring: monitor ↔ assets (many-to-many) + filters.
- Findings/Controls/Tasks/Docs/Reports: allow linking assets as a first-class related entity + search/navigation.

## Validation (MVP)

- `name`: required, 1..200, trim.
- `ip_addresses`: each item must be a valid IPv4/IPv6; blanks/duplicates removed.
- `tags`: 0..N, each tag 1..50, trim; duplicates removed.
- `commissioned_at`: valid date, nullable.
