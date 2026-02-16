# Quick Start (RU)

Минимальный запуск Berkut SCC в локальной среде.

## 1. Подготовка
```bash
cp .env.example .env
```

Проверьте, что в `.env` заданы не дефолтные секреты для non-dev:
- `CSRF_KEY`
- `PEPPER`
- `DOCS_ENCRYPTION_KEY`
- `BACKUP_ENCRYPTION_KEY`

## 2. Запуск
```bash
docker compose up -d --build
```

## 3. Проверка
- UI: `http://localhost:8080/login`
- Статус контейнеров:
```bash
docker compose ps
```

## 4. Логи
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 5. Дальше
- Полный деплой и CI/CD: [`docs/ru/deploy.md`](docs/ru/deploy.md)
- Runbook (диагностика/восстановление): [`docs/ru/runbook.md`](docs/ru/runbook.md)
- HTTPS + OnlyOffice: [`docs/ru/https_onlyoffice.md`](docs/ru/https_onlyoffice.md)
- Reverse-proxy compose пример: [`docs/ru/docker-compose.https.yml`](docs/ru/docker-compose.https.yml)
