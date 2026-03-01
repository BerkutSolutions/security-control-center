# Tab Compatibility (Compat) and the Action Wizard

## Why this exists
After upgrades, a tab (module) may require data alignment due to schema or behavior changes. To prevent silent breakage, the app checks compatibility **after login** and, if needed, **offers user-driven actions** per tab:

- **Partial adapt** — safe adaptation (must not delete business data).
- **Full reset** — full tab re-initialization (deletes tab data).

Important: the system performs **no automatic migrations/resets**. The user decides.

## How it works (high level)
1) After login, the UI calls `GET /api/app/compat`.
2) If any module is not `ok`, the UI shows a wizard with “what changed” and available actions.
3) Actions run as background **jobs**, and the UI polls progress until completion.

## Healthcheck page (/healthcheck)
Right after login, a one-time `/healthcheck` page may be available. It shows:

- basic availability/permission probes;
- the Compat report (per tab/module) based on `GET /api/app/compat`.

This page is read-only and is meant for quick “post-login/post-upgrade” diagnostics.

See: `docs/eng/healthcheck.md`.

## Compatibility statuses
Each module has “expected versions” (in code) and “applied versions” (in DB).

- `ok` — applied >= expected, no action required.
- `needs_attention` — action required, but a safe path exists (**Partial adapt** available).
- `needs_reinit` — safe adaptation is not available/sufficient → **Full reset** is recommended.
- `broken` — cannot evaluate reliably (read error/corruption) → manual diagnosis is required, Full reset may be used.

## Partial adapt (safe)
Typical operations:
- cleanup/repair of derived data that can be recomputed;
- backfills for new fields;
- format normalization;
- index/aggregate rebuilds.

Guarantee: **Partial adapt must not delete business data** (docs, incidents, monitors, etc.).

## Full reset (destructive)
Full reset deletes all data for the selected module (and related files, if any) and restores defaults.

Recommendations:
- Before Full reset, **create a backup** (Backups tab or your external DB/storage backup).
- Use Full reset only if you understand the data-loss impact.

Security constraints:
- Full reset of critical modules may require the `superadmin` role.

## Jobs and progress
Every action runs as a job:
- Create: `POST /api/app/jobs`
- Progress/result: `GET /api/app/jobs/:id`
- Recent list: `GET /api/app/jobs`
- (Optional) cancel: `POST /api/app/jobs/:id/cancel`

Jobs expose:
- status (`queued`/`running`/`finished`/`failed`/`canceled`)
- progress (0–100)
- affected object counts (for transparency)

## Permissions (zero-trust)
All endpoints are authorized on the server. The UI is not a security boundary.

Base permissions:
- `app.compat.view` — view compat report and jobs status
- `app.compat.manage.partial` — start Partial adapt
- `app.compat.manage.full` — start Full reset

Additionally:
- reset/adapt requires `settings.advanced`;
- specific modules may require their own permissions (e.g. `monitoring.manage` for monitoring).

## Audit
Administrative actions are written to server audit log, including:
- job start/finish (`app.job.start`, `app.job.finish`);
- module actions (`app.module.reset.partial`, `app.module.reset.full`) with details (module/mode/counts).
