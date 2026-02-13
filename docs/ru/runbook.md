# Runbook (запуск и восстановление)

## 1. Старт через Docker Compose
```bash
docker compose up -d --build
```

Проверка статуса:
```bash
docker compose ps
```

## 2. Просмотр логов
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 3. Типовые проблемы

### 3.1 Порт 8080 занят
Симптом: `Bind for 0.0.0.0:8080 failed`.

Решение:
- освободить порт на хосте, либо
- изменить `PORT` в `.env` и перезапустить compose.

### 3.2 Ошибка миграций/старт после неудачного апдейта
Сначала попробуйте явный запуск миграций:
```bash
docker compose run --rm berkut /usr/local/bin/berkut-migrate
docker compose up -d berkut
```

Решение (с потерей данных):
```bash
docker compose down -v
docker compose up -d --build
```

### 3.3 Ошибка default secrets вне dev
Симптом: `default secrets are not allowed outside APP_ENV=dev`.

Решение:
- для dev: `ENV=dev`, `APP_ENV=dev`;
- для prod: задать реальные значения `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`.

## 4. Полное восстановление
ВНИМАНИЕ: удалит данные PostgreSQL и файловое хранилище.
```bash
docker compose down -v
docker compose up -d --build
```

## 5. Проверка доступности
- UI: `http://localhost:8080/login`
- Health статусы контейнеров: `docker compose ps`
