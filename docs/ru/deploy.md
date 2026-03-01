# Деплой и CI/CD

## Docker-стек
- Образ приложения: `docker/Dockerfile`
- Compose-стек: `docker-compose.yaml`
- Сервисы: `berkut` (app), `postgres` (DB)
- Данные: volumes `berkut_data`, `berkut_pgdata`

## Переменные окружения
Основные:
- `ENV`, `APP_ENV`, `APP_CONFIG`, `PORT`, `DATA_PATH`

Секреты:
- `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`

DB/runtime:
- `BERKUT_DB_DRIVER=postgres`
- `BERKUT_DB_URL=postgres://...`
- прочие `BERKUT_*` overrides при необходимости

HA / Scaling:
- Режим запуска: `BERKUT_RUN_MODE=all|api|worker` (по умолчанию `all`).
- Для HA рекомендуется: несколько реплик `api` (только HTTP/UI/API) + одна реплика `worker` (фоновые воркеры).

Observability:
- Liveness: `GET /healthz`
- Readiness (DB ping): `GET /readyz`
- Prometheus-метрики (по умолчанию выключено): `BERKUT_METRICS_ENABLED=true` и `GET /metrics` (если задан `BERKUT_METRICS_TOKEN`, используйте `Authorization: Bearer $BERKUT_METRICS_TOKEN`)
- (только для home/dev) разрешить `/metrics` без токена: `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=true`

Passkeys (WebAuthn):
- Включение/выключение: `BERKUT_WEBAUTHN_ENABLED=true|false`
- RP ID / origins (рекомендуется для prod):
  - `BERKUT_WEBAUTHN_RP_ID=scc.example.com`
  - `BERKUT_WEBAUTHN_ORIGINS=https://scc.example.com`
  - `BERKUT_WEBAUTHN_RP_NAME=SCC`
- Требования: HTTPS (или `localhost`).

Upgrade:
- Preflight: `GET /api/app/preflight` (право `app.preflight.view`)
- Backup-before-migrate (опционально): `BERKUT_UPGRADE_BACKUP_BEFORE_MIGRATE=true`

## Типовой деплой
```bash
cp .env.example .env
docker compose pull
docker compose up -d --build
```

Примечание:
- При старте контейнера сначала выполняется `berkut-migrate`, затем запускается `berkut-scc`.
- Для запуска вне Docker применяйте миграции явно до старта приложения:
```bash
make migrate
go run ./main.go
```

## Rollback
1. Зафиксировать предыдущий `IMAGE_TAG`.
2. Перезапустить только приложение:
```bash
docker compose up -d --no-deps berkut
```

## CI
`.gitlab-ci.yml` содержит проверочный этап (`verify-go`) перед сборкой/публикацией образа.
