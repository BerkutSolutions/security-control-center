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
