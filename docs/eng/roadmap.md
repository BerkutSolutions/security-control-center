# Evolution Plan

## Current status
- Migrations: `goose`, PostgreSQL runtime.
- Logging: `slog`.
- Configuration: `cleanenv`.
- RBAC: Casbin.
- Server-side zero-trust authorization guards are in place for API routes.
- Monitoring: extended real check types (`http`, `http_keyword`, `http_json`, `tcp`, `dns`, `redis`, `postgres`).
- Routing: modular `chi` stack.

## Priorities
1. Continue expanding monitoring check types (no placeholders).
2. Add more integration tests for concurrent write scenarios.
3. Keep docs and runbook continuously aligned with runtime behavior.
4. Maintain strict module boundaries and manageable file size.
5. Strengthen production operational checklists.

## Quality gates
- No unguarded endpoints (server-side authz).
- All user-facing errors localized RU/EN.
- No external CDN/online dependencies for UI assets.
- All changes pass `make ci`.
