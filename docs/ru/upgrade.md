# Обновление и откат (Upgrade / Rollback)

Документ описывает рекомендуемую дисциплину обновлений для Berkut SCC в self-hosted окружении.

## Перед обновлением

1) Убедитесь, что есть актуальная резервная копия (`.bscc`):
- вкладка **Backups** → создать backup (рекомендуется)

2) Выполните preflight (проверки администратора):
- `GET /api/app/preflight` (нужно право `app.preflight.view`)
  - включает проверки БД/миграций/хранилищ/режима запуска, а также security-проверки (trusted proxies, токен для `/metrics`, политика/доступность internal URL для OnlyOffice).

3) Если у вас HA (`BERKUT_RUN_MODE=api|worker`):
- обновляйте `worker` и `api` последовательно (не одновременно), чтобы избежать лишних параллельных стартов.

## Обновление (Docker Compose)

1) Зафиксируйте версию образа (рекомендуется использовать конкретный `IMAGE_TAG`, а не `latest`).

2) Поднимите сервис:
```bash
docker compose pull
docker compose up -d --build
```

Примечание:
- при старте контейнера выполняются миграции (`berkut-migrate`), затем запускается приложение.

## Опционально: backup-before-migrate

Можно включить создание резервной копии перед применением миграций **только если миграции действительно ожидаются**:
- `BERKUT_UPGRADE_BACKUP_BEFORE_MIGRATE=true`
- `BERKUT_UPGRADE_BACKUP_INCLUDE_FILES=true|false`
- `BERKUT_UPGRADE_BACKUP_LABEL=...` (опционально)

## Проверки после обновления

- `GET /readyz`
- `GET /api/app/preflight`
- UI `/healthcheck` (одноразовая post-login страница)

## Откат (Rollback)

Рекомендуемый способ отката — восстановление из `.bscc` бэкапа:

1) Остановить сервис:
```bash
docker compose down
```

2) Запустить предыдущую версию (pin `IMAGE_TAG`) и поднять сервис.

3) Восстановить backup через UI (**Backups** → restore) или через соответствующий API.

Примечание:
- откат без восстановления данных (только смена `IMAGE_TAG`) не гарантирован, если новые миграции уже применены.
