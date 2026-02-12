# Architecture

## Overview
- Monolith: Go backend + embedded UI (`embed`).
- Storage: SQLite with startup migrations.
- UI: SPA-style routing inside `app.html`.

## Core modules
- `api/` - HTTP server, routing, middleware, handlers.
- `core/auth` - auth, sessions, CSRF.
- `core/rbac` - permission policy.
- `core/store` - SQLite stores and migrations.
- `core/docs`, `core/incidents`, `core/monitoring`, `tasks/` - business domains.
- `gui/static` - UI and RU/EN i18n.

## Principles
- Zero-trust: server validates permissions on every endpoint.
- Deny-by-default RBAC + ACL + classification/clearance.
- Audit logging for critical operations.
