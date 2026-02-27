# Software registry (MVP spec)

The Software module is a registry of software products and versions, plus an Asset -> Software installations relation.

Key goals:
- stable identifiers for "software" relations across SCC domains;
- a unified catalog for inventory and reporting;
- server-side validation and deny-by-default RBAC.

## Data model (MVP)

### Product

Required:
- `name` - product name.

Recommended:
- `vendor` - vendor/publisher (free text).
- `product_type` - `os | middleware | database | application | library | agent | other`.
- `description` - notes.
- `tags` - string array.

System fields:
- `id` (int64), `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at` (soft-delete/archive).

### Version

Required:
- `version` - version string (trimmed, 1..100).

Recommended:
- `release_date` - date (optional).
- `eol_date` - date (optional).
- `notes` - free text.

System fields:
- `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

### Installation (asset -> software)

Required:
- `asset_id`
- `product_id`

Recommended:
- `version_id` - reference to registry version (optional if `version_text` is used).
- `version_text` - version string for cases where the exact version is not in registry (optional).
- `installed_at` - date (optional).
- `source` - inventory source (free text, optional).
- `notes` - free text.

System fields:
- `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

## RBAC and zero-trust

New permissions (deny-by-default):
- `software.view` - view products/versions/installations.
- `software.manage` - create/update/archive/restore + manage versions and installations.

Every `/api/software*` endpoint must enforce permissions server-side. UI/menu is not a security control.

## Menu access (menu_permissions)

- tab: `software`
- UI route: `/registry/software` (Registries tab). Legacy direct route: `/software`
- API route group: `/api/software`

Tab access is additionally limited by `menu_permissions`.

Installations endpoints are under Assets API:
- `/api/assets/{id}/software*`

For installations, the server enforces both:
- Assets permissions (`assets.view` / `assets.manage`) and `menu_permissions` for `assets`
- Software permissions (`software.view` / `software.manage`) and `menu_permissions` for `software`

This prevents bypassing the Software tab restrictions through Assets endpoints.

## API (core)

Products:
- `GET /api/software` - list/search (filters: `q`, `vendor`, `type`, `tag`, `include_deleted=1` requires `software.manage`).
- `GET /api/software/{id}` - details.
- `POST /api/software` - create.
- `PUT /api/software/{id}` - update (optimistic lock via `version`).
- `DELETE /api/software/{id}` - archive (soft-delete).
- `POST /api/software/{id}/restore` - restore.

Versions:
- `GET /api/software/{id}/versions` - list versions.
- `POST /api/software/{id}/versions` - create version.
- `PUT /api/software/{id}/versions/{version_id}` - update.
- `DELETE /api/software/{id}/versions/{version_id}` - archive.
- `POST /api/software/{id}/versions/{version_id}/restore` - restore.

Installations:
- `GET /api/assets/{id}/software` - list installations for the asset.
- `POST /api/assets/{id}/software` - add installation.
- `PUT /api/assets/{id}/software/{install_id}` - update installation.
- `DELETE /api/assets/{id}/software/{install_id}` - archive installation.
- `POST /api/assets/{id}/software/{install_id}/restore` - restore installation.

## Export and autocomplete

- `GET /api/software/export.csv` - products CSV export (same filters as list; `limit` up to 5000).
- `GET /api/software/autocomplete` - suggestions (params: `field=all|names|vendors|tags`, `q`, `limit`, `include_deleted=1` requires `software.manage`).

## Links/integrations

The entity links system supports `software` as a link target type.

When creating links to software, the server additionally validates:
- permissions (`software.view`);
- tab availability via `menu_permissions`.

## Audit

Key actions:
- `software.create`, `software.update`, `software.archive`, `software.restore`
- `software.version.create`, `software.version.update`, `software.version.archive`, `software.version.restore`
- `software.export.csv`
- `assets.software.add`, `assets.software.update`, `assets.software.archive`, `assets.software.restore`
