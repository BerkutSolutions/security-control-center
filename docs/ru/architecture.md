# Архитектура

## Обзор
- Монолит: Go backend + встроенный UI (`embed`).
- База: SQLite с миграциями на старте.
- UI: SPA-маршрутизация внутри `app.html`.

## Ключевые модули
- `api/` - HTTP сервер, роутинг, middleware, handlers.
- `core/auth` - auth, sessions, CSRF.
- `core/rbac` - role/permission policy.
- `core/store` - SQLite stores и migrations.
- `core/docs`, `core/incidents`, `core/monitoring`, `tasks/` - бизнес-домены.
- `gui/static` - UI и i18n RU/EN.

## Принципы
- Zero-trust: проверка прав сервером на каждом endpoint.
- Deny-by-default RBAC + ACL + classification/clearance.
- Аудит критичных операций.
