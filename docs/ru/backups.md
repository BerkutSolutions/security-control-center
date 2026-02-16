# Berkut SCC - Бэкапы (.bscc)

## Обзор
Модуль бэкапов в версии `1.0.3` поддерживает:
- создание зашифрованных `.bscc` (DB-only);
- import/upload `.bscc`;
- потоковое скачивание `.bscc`;
- восстановление (dry-run и реальное) с прогрессом по шагам;
- backup plan, scheduler и retention policy;
- выборочный ручной бэкап (имя, scope, include_files);
- zero-trust RBAC и аудит на всех backup-endpoint.

## Конфигурация
Основные переменные окружения:
- `BERKUT_BACKUP_PATH` - каталог хранения бэкапов.
- `BACKUP_ENCRYPTION_KEY` или `BERKUT_BACKUP_ENCRYPTION_KEY` - ключ шифрования.
- `BERKUT_BACKUP_PGDUMP_BIN` - путь к `pg_dump`.
- `BERKUT_BACKUP_MAX_PARALLEL` - лимит параллельных backup jobs.
- `BERKUT_BACKUP_UPLOAD_MAX_BYTES` - лимит upload при import.

Важно:
- в non-dev ключ шифрования обязателен;
- секреты нельзя хранить в git.

## API (основное)
- `GET /api/backups` - список бэкапов.
- `GET /api/backups/{id}` - детали бэкапа.
- `POST /api/backups` - создать бэкап (поддерживает `label`, `scope`, `include_files`).
- `POST /api/backups/import` - импортировать `.bscc`.
- `GET /api/backups/{id}/download` - скачать `.bscc`.
- `DELETE /api/backups/{id}` - удалить бэкап.
- `POST /api/backups/{id}/restore` - запустить восстановление.
- `POST /api/backups/{id}/restore/dry-run` - dry-run восстановление.
- `GET /api/backups/restores/{restore_id}` - статус и шаги прогресса.
- `GET /api/backups/plan` - текущий план автобэкапов.
- `PUT /api/backups/plan` - обновить план.
- `POST /api/backups/plan/enable` - включить план.
- `POST /api/backups/plan/disable` - отключить план.

## Maintenance mode
Во время реального restore приложение включает maintenance mode:
- обычные API временно блокируются (`503`);
- endpoint статуса restore остаётся доступным;
- после завершения restore maintenance mode отключается.

## Расписание и хранение
- Поддерживаются пресеты расписания: ежедневно, еженедельно, ежемесячно (начало/конец месяца).
- Учитываются время запуска и день недели (для недельного плана).
- После рестарта scheduler ориентируется на последнюю успешную автокопию и не выполняет лишний немедленный запуск.
- Retention удаляет старые артефакты по `retention_days`, но сохраняет минимум `keep_last_successful`.

## Безопасность
- Проверка прав выполняется на сервере для каждого endpoint.
- Ошибки API не раскрывают внутренние пути, SQL/stack trace и технические детали.
- Для upload/download/delete применён hardening: лимиты, path validation, конфликтные блокировки и безопасные коды ошибок.
