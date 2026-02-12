# Runbook (старт и восстановление)

## 1. Старт локально (docker run)
```bash
docker rm -f berkut-scc || true
docker build -t berkut-scc -f docker/Dockerfile .
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 2. Старт через docker compose
```bash
docker compose up -d --build
```

## 3. Частая ошибка: readonly SQLite
Симптом: `attempt to write a readonly database (8)`.

Фикс прав volume:
```bash
docker rm -f berkut-scc || true
docker run --rm --user 0 --entrypoint sh -v berkut-data:/app/data berkut-scc -c "chown -R berkut:berkut /app/data"
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 4. Ошибка default secrets вне dev
Симптом: `default secrets are not allowed outside APP_ENV=dev`.

Решение:
- Для dev: поставить `ENV=dev` и `APP_ENV=dev`.
- Для prod: заменить секреты на собственные значения.

## 5. Полное восстановление с пересозданием данных
ВНИМАНИЕ: удалит базу.
```bash
docker rm -f berkut-scc || true
docker volume rm berkut-data
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 6. Проверка логов
```bash
docker logs -f berkut-scc
```
