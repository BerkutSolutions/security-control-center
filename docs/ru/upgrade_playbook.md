# Upgrade Playbook

Пошаговый регламент обновления production-контура.

## 1. Preflight

1. Выполнить `GET /api/app/preflight`.
2. Убедиться, что критичные baseline-check не имеют `failed`.
3. Зафиксировать результат preflight в change request.

## 2. Backup

1. Создать backup перед миграциями (`backup-before-migrate`).
2. Проверить целостность backup (dry-run restore или integrity-check).
3. Зафиксировать ID backup в change request.

## 3. Migrate

1. Применить миграции целевой версии.
2. Контролировать логи и длительность этапа.

## 4. Smoke checks

Проверить:

1. Логин/сессии/CSRF.
2. Ключевые API по ролям.
3. `GET /healthz`, `GET /readyz`.
4. Ключевые вкладки (Docs/Monitoring/Incidents/Backups/Logs).

## 5. Rollback decision

Откат обязателен, если:

1. Невозможно пройти smoke checks в пределах change window.
2. Критичные функции недоступны.
3. Есть нарушение целостности данных/аудита.

## 6. Post-upgrade

1. Выгрузить `GET /api/logs/export/package` и сохранить как release evidence.
2. Обновить `CHANGELOG.md` по главам (`UI`, `Core`, `Security`, `Infrastructure`, `Tabs/Modules`).
