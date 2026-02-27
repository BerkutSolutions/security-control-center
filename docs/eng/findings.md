# Findings

The **Findings** module is a registry of technical/process/compliance findings with tags and relations to other entities (assets, controls, incidents, tasks, documents).

## Fields (MVP)

- `title` — required.
- `description_md` — Markdown description.
- `status` — `open | in_progress | resolved | accepted_risk | false_positive`.
- `severity` — `low | medium | high | critical`.
- `finding_type` — `technical | config | process | compliance | other`.
- `owner` — free-text owner (MVP).
- `due_at` — due date (date/ISO time).
- `tags` — string array (normalized: trim, upper).
- system fields: `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

## RBAC and zero-trust

New permissions (deny-by-default):
- `findings.view` — view list/details/search.
- `findings.manage` — create/edit/archive/restore + manage relations.

Every `/api/findings*` endpoint must enforce permissions server-side. UI/menu is not a security control.

## Menu access (menu_permissions)

- tab: `findings`
- UI route: `/registry/findings` (Registries tab). Legacy direct route: `/findings`
- API route group: `/api/findings`

Tab access is additionally limited by `menu_permissions` (same model as other tabs).

## API (core)

- `GET /api/findings` — list/search (filters: `q`, `status`, `severity`, `type`, `tag`; `include_deleted=1` requires `findings.manage`).
- `GET /api/findings/{id}` — details.
- `POST /api/findings` — create.
- `PUT /api/findings/{id}` — update (optimistic lock via `version`).
- `DELETE /api/findings/{id}` — archive (soft-delete).
- `POST /api/findings/{id}/restore` — restore.

## API (export/autocomplete)

- `GET /api/findings/export.csv` — CSV export (same filters as list; `limit` up to 5000).
- `GET /api/findings/autocomplete` — field suggestions (params: `field=all|titles|owners|tags`, `q`, `limit`, `include_deleted=1` requires `findings.manage`).

## Relations (links)

Relations are stored in `entity_links` (`source_type="finding"`):
- `GET /api/findings/{id}/links`
- `POST /api/findings/{id}/links`
- `DELETE /api/findings/{id}/links/{link_id}`

For link types `asset`/`control`, the server additionally enforces permissions (`assets.view`/`controls.view`) and tab availability via `menu_permissions`.

## Audit

Key actions:
- `finding.create`, `finding.update`, `finding.archive`, `finding.restore`
- `finding.link.add`, `finding.link.remove`
- `finding.export.csv`
