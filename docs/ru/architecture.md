# Архитектура

## Обзор
- Монолит: Go backend + встроенный UI (`embed`).
- Хранение: PostgreSQL.
- Миграции: `goose` как явный шаг (`cmd/migrate` / `make migrate`); в Docker-образе выполняются в entrypoint контейнера до старта приложения.
- Конфигурация: `cleanenv` (`config/app.yaml` + ENV).

## Ключевые модули
- `api/` — HTTP сервер, middleware, роутинг, handlers.
- `api/routegroups` — пакетная декомпозиция групп роутов по доменам.
- `core/appbootstrap` — единая точка инициализации runtime-компонентов.
- `core/auth` — аутентификация, сессии, CSRF.
- `core/rbac` — RBAC на Casbin.
- `core/store` — слой PostgreSQL stores и миграции.
- `core/docs`, `core/incidents`, `core/monitoring`, `tasks/` — доменные модули.
- `gui/static` — фронтенд, локальные ассеты и i18n RU/EN.

## Роутинг
- Core API/shell роутинг использует модульный `chi`.
- `tasks/http` остается выделенным модулем роутера и монтируется в общее API-дерево.
- Route wiring вынесен в отдельные модули, чтобы минимизировать связность и упростить сопровождение.

## Фоновые сервисы
- Lifecycle scheduler/monitoring отделен от транспортного слоя API.
- Запуск/остановка выполняются через контекстный контракт и централизованный lifecycle-менеджер.

## Принципы
- Zero-trust: сервер проверяет права на каждом endpoint.
- Deny-by-default RBAC + ACL + classification/clearance.
- Аудит критичных операций и изменений конфигурации.
