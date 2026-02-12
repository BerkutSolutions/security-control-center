# Berkut Solutions - Security Control Center

[English version](README.en.md)

Berkut Solutions - Security Control Center - это self-hosted платформа для управления безопасностью и соответствием требованиям, реализованная как Go-монолит со встроенным UI.

Ключевые цели:
- zero-trust проверки доступа на каждом API endpoint
- локально контролируемое развертывание (без CDN-зависимостей)
- аудит критичных действий
- предсказуемая эксплуатация через Docker и GitLab CI/CD

## Оглавление
1. [Что входит в платформу](#что-входит-в-платформу)
2. [Архитектура](#архитектура)
3. [Быстрый старт](#быстрый-старт)
4. [Готовый Docker Hub образ](#готовый-docker-hub-образ)
5. [Команды восстановления](#команды-восстановления)
6. [Docker Compose](#docker-compose)
7. [Быстрый деплой через docker-compose](#быстрый-деплой-через-docker-compose)
8. [Конфигурация](#конфигурация)
9. [Карта документации](#карта-документации)
10. [Заметки по безопасности](#заметки-по-безопасности)

## Что входит в платформу
- Управление задачами и workflow (включая согласования)
- Раздел мониторинга с роутингом вкладок
- Модель авторизации RBAC + ACL
- Audit log для чувствительных операций
- Встроенный web UI, отдаваемый backend-ом
- Хранилище SQLite с постоянным Docker volume

## Архитектура
- Backend: Go (1.22+)
- Storage: SQLite
- UI: встроенные статические frontend-ассеты
- Deployment: Docker / Docker Compose / GitLab CI
- TLS-стратегия: рекомендуется внешний reverse proxy (Nginx/Traefik)

## Быстрый старт
Локальная сборка:
```bash
docker rm -f berkut-scc || true
docker build -t berkut-scc -f docker/Dockerfile .
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

Открыть: `http://localhost:8080`

## Готовый Docker Hub образ
Репозиторий образа:
`https://hub.docker.com/repository/docker/berkutsolutions/security-control-center/general`

Запуск без локальной сборки:
```bash
docker pull berkutsolutions/security-control-center:latest
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkutsolutions/security-control-center:latest
```

## Команды восстановления
Исправить права на volume при `readonly database`:
```bash
docker run --rm --user 0 --entrypoint sh -v berkut-data:/app/data berkut-scc -c "chown -R berkut:berkut /app/data"
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

Пересоздать volume (удалит данные):
```bash
docker rm -f berkut-scc || true
docker volume rm berkut-data
```

Логи контейнера:
```bash
docker logs -f berkut-scc
```

## Docker Compose
Запуск:
```bash
docker compose up -d --build
```

Запуск с профилем reverse proxy:
```bash
docker compose --profile proxy up -d
```

## Быстрый деплой через docker-compose
```yaml
version: "3.8"

services:
  scc:
    image: ghcr.io/berkutsolutions/security-control-center:latest
    container_name: berkut-scc
    ports:
      - "8080:8080"
    environment:
      ENV: "dev"
      APP_ENV: "dev"
      APP_CONFIG: "/app/config/app.yaml"
      PORT: "8080"
      DATA_PATH: "/app/data"
      CSRF_KEY: "changeme"
      PEPPER: "changeme"
      DOCS_ENCRYPTION_KEY: "changeme"
      BERKUT_LISTEN_ADDR: "0.0.0.0:8080"
      BERKUT_DB_PATH: "/app/data/berkut.db"
      BERKUT_DOCS_STORAGE_DIR: "/app/data/docs"
      BERKUT_INCIDENTS_STORAGE_DIR: "/app/data/incidents"
      BERKUT_SECURITY_TRUSTED_PROXIES: "127.0.0.1,10.0.0.0/8"
      BERKUT_TLS_ENABLED: "false"
      HTTPS_MODE: "external_proxy"
      HTTPS_PORT: "8080"
      HTTPS_TRUSTED_PROXIES: "127.0.0.1,10.0.0.0/8"
      HTTPS_EXTERNAL_PROXY_HINT: "nginx"
      BERKUT_SCHEDULER_ENABLED: "true"
      BERKUT_SCHEDULER_INTERVAL_SECONDS: "60"
      BERKUT_SCHEDULER_MAX_JOBS_PER_TICK: "20"
    volumes:
      - berkut-data:/app/data
    restart: unless-stopped

volumes:
  berkut-data:
```

Обязательные переменные:
- `CSRF_KEY`
- `PEPPER`
- `DOCS_ENCRYPTION_KEY`
- `DATA_PATH` (или эквивалентные `BERKUT_*_PATH`)

Что менять в production:
- Всегда заменить `changeme` на сильные секреты.
- Перевести `APP_ENV`/`ENV` на production профиль и использовать `DEPLOYMENT_MODE=enterprise`.
- Включить TLS через reverse proxy или встроенный TLS режим согласно политике эксплуатации.
- Ограничить `BERKUT_SECURITY_TRUSTED_PROXIES` только реальными доверенными адресами.

SQLite хранение:
- Файл БД: `/app/data/berkut.db` (персистентно через volume `berkut-data`).
- Данные документов и инцидентов: `/app/data/docs`, `/app/data/incidents` в том же volume.
- Для резервного копирования достаточно архивировать volume/каталог `/app/data`.

## Конфигурация
- Основной env-файл: `.env`
- Шаблон env: `.env.example`
- Конфиг приложения: `config/app.yaml`
- Dockerfile: `docker/Dockerfile`

## Карта документации
Этот README - обзорный. Подробная документация вынесена в `docs`.

- Общий индекс: `docs/README.md`
- Русская документация: `docs/ru/README.md`
- Английская документация: `docs/eng/README.md`

Runbook:
- RU: `docs/ru/runbook.md`
- EN: `docs/eng/runbook.md`

Углубленная wiki:
- RU (вкладки/функции): `docs/ru/wiki/tabs.md`, `docs/ru/wiki/features.md`
- EN (tabs/features): `docs/eng/wiki/tabs.md`, `docs/eng/wiki/features.md`

## Заметки по безопасности
- Не используйте default secrets вне dev-среды.
- Для production предпочтительнее TLS-терминация на reverse proxy.
- Ограничивайте `BERKUT_SECURITY_TRUSTED_PROXIES` только доверенными адресами прокси.
- Аудит критичных изменений конфигурации должен быть включен.

