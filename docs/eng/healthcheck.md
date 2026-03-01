# Healthcheck (/healthcheck)

`/healthcheck` is a one-time post-login status page. It is available **only immediately after a successful login** (and after a password change on first login). Once you continue into the app, the page is hidden until the next login.

## What it does

It is meant for quick diagnostics to confirm that after an upgrade and login:

- session and user are valid;
- key endpoints required for main tabs are reachable (with your current permissions);
- tab compatibility (Compat) report is fetched via `GET /api/app/compat`.

The page **does not perform automatic migrations** and **does not change data**.

## How to use it

1. Log in to SCC.
2. If the app opens `/healthcheck`, wait until checks finish.
3. Expand the “Tabs” (Compat) section to see per-module statuses.
4. Click “Continue” to enter the main UI.

## Permissions and access

- Visibility of checks and Compat report depends on your role permissions.
- Compat report requires `app.compat.view`.

If you see “403” or some items are marked as “skipped”, it usually means your role does not have access to that tab/endpoint.

## If Compat is not OK

If Compat contains `needs_attention` or `incompatible` statuses, the app shows a post-login compatibility wizard that offers **user-driven** actions only:

- Partial adapt — alignment without deleting business data;
- Full reset — full tab reinitialization (module data wipe).

Actions are executed via background jobs and require additional permissions. See `docs/eng/compatibility.md`.

