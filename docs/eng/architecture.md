# Architecture

## Overview
- Monolith: Go backend + embedded UI (`embed`).
- Storage: PostgreSQL.
- Migrations: `goose` as an explicit step (`cmd/migrate` / `make migrate`); in Docker image it runs in container entrypoint before app process.
- Configuration: `cleanenv` (`config/app.yaml` + ENV).

## Core modules
- `api/` — HTTP server, middleware, routing, handlers.
- `api/routegroups` — package-level decomposition of route groups by domain.
- `core/appbootstrap` — explicit runtime initialization entry point.
- `core/auth` — authentication, sessions, CSRF.
- `core/rbac` — Casbin-backed RBAC.
- `core/store` — PostgreSQL stores and migration layer.
- `core/docs`, `core/incidents`, `core/monitoring`, `tasks/` — domain modules.
- `gui/static` — frontend, local assets, RU/EN i18n.

## Routing
- Core API/shell routing uses modular `chi`.
- `tasks/http` remains a dedicated module router and is mounted into the API tree.
- Route wiring is split into dedicated modules to reduce coupling and improve maintainability.

## Background services
- Scheduler/monitoring lifecycle is separated from API transport concerns.
- Start/stop is controlled via context contracts and centralized lifecycle manager.

## Principles
- Zero-trust: permissions are validated server-side on every endpoint.
- Deny-by-default RBAC + ACL + classification/clearance.
- Audit logging for critical operations and configuration changes.
