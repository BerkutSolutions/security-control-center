# Berkut Solutions - Security Control Center

<p align="center">
  <img src="gui/static/logo.png" alt="Berkut SCC logo" width="220">
</p>

[English version](README.en.md)

Berkut Solutions - Security Control Center — self-hosted платформа управления безопасностью и соответствием требованиям, реализованная как Go-монолит со встроенным UI.

Ключевые принципы:
- zero-trust проверки прав сервером на каждом endpoint
- локальное развёртывание без внешних CDN-зависимостей
- аудит критичных операций
- предсказуемая эксплуатация через Docker/Docker Compose и CI

## Что входит в платформу
- Документы, согласования и шаблоны
- Инциденты и отчётность
- Задачи (spaces/boards/tasks)
- Мониторинг и уведомления
- Управление пользователями, ролями и группами
- Аудит действий

## Текущая архитектура
- Backend: Go `1.23`
- БД: PostgreSQL (production runtime)
- Миграции: `goose`
- Конфигурация: `cleanenv` + `config/app.yaml` + ENV
- RBAC: Casbin
- UI: встроенные статические ассеты (`gui/static`), RU/EN i18n
- Роутинг: модульный стек `chi`

## Быстрый старт
```bash
cp .env.example .env
docker compose up -d --build
```

Открыть: `http://localhost:8080`

## Полезные команды
Применить миграции (явно, отдельно от запуска приложения):
```bash
make migrate
```

Перезапуск стека:
```bash
docker compose down
docker compose up -d --build
```

Полное пересоздание данных (деструктивно):
```bash
docker compose down -v
docker compose up -d --build
```

Логи:
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## Конфигурация
- `.env` — рабочий env-файл
- `.env.example` — шаблон
- `config/app.yaml` — базовый конфиг приложения
- `docker-compose.yaml` — основной compose-стек
- `docker/Dockerfile` — образ приложения

Обязательные секреты для non-dev:
- `CSRF_KEY`
- `PEPPER`
- `DOCS_ENCRYPTION_KEY`

## Разработка и проверки
```bash
make ci
```

Доступные цели:
```bash
make fmt
make fmt-check
make vet
make test
make lint
make migrate
```

## Документация
- Общий индекс: `docs/README.md`
- Русская документация: `docs/ru/README.md`
- English docs: `docs/eng/README.md`

## Безопасность
- Не используйте default-secrets вне dev.
- Проверка прав выполняется сервером для каждого endpoint.
- Ограничивайте `BERKUT_SECURITY_TRUSTED_PROXIES` только доверенными адресами.
- Используйте TLS-терминацию на reverse proxy в production.

## Скриншоты
![Скриншот 1](gui/static/screen1.png)
![Скриншот 2](gui/static/screen2.png)
![Скриншот 3](gui/static/screen3.png)
