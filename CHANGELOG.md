# Журнал изменений

## 1.1.3 — 05.03.2026

### UI
- Исправлена модалка step-up: больше не закрывается по клику вне окна и `Esc`.
- Убран шумный HTTP-суффикс из пользовательских ошибок step-up (`auth.stepup.required`, `auth.stepup.locked`).
- Исправлены отступы/переносы кнопок в step-up модалке.
- Для профиля добавлены недостающие модалки 2FA/passkeys (setup/disable/register/rename/delete).
- Для новых/скопированных мониторов стартовый статус изменен на `pending` (без ложного `DOWN` до первой проверки).

### Мониторинг
- Исправлен ложный `DOWN` и ложные уведомления сразу после создания/копирования монитора.
- Добавлен нейтральный статус `pending` в список/детали/события и i18n.
- Оптимизировано создание монитора: ожидание первой проверки переведено в фоновый режим.
- Для `HTTP 404` сохранена логика: при неразрешенном коде — `ISSUE`, при разрешенном — `UP`.

### Бэкапы и восстановление
- Реализован scoped backup dump: при выборе вкладок выгружаются только таблицы выбранных модулей.
- Реализован scoped restore без полного drop схемы.
- Для scoped restore исправлен сценарий, когда `pg_restore` мог «молча» не восстановить данные области.
- Добавлена консоль восстановления в UI с полным логом шагов (включая dry-run).
- Добавлены before/after счетчики таблиц и валидация результата scoped restore.
- Добавлена расширенная scoped-валидация для всех модулей по `entity_counts`.
- Добавлено восстановление от «зависших» backup/restore операций (очистка stale `queued/running`).

### Профиль и аутентификация
- Починен bind модулей 2FA/passkeys на странице профиля.
- Исправлены статусы 2FA/passkeys в профиле.
- Для passkeys улучшена совместимость регистрации (resident key: `preferred` вместо `required`).
- Для локального режима (`home/dev`) WebAuthn fallback origins теперь учитывает и `https://host`, и `http://host` для устранения ложного origin mismatch за прокси.

### Безопасность
- Для критичных endpoint-ов включен обязательный свежий step-up (окно 15 минут):
  - purge логов;
  - изменение runtime/https hardening;
  - критичные операции аккаунтов/групп/ролей.
- `audit_log` переведен в append-only режим (блок UPDATE/DELETE на уровне БД).
- В аудит добавлена цепочка целостности `prev_hash -> event_hash` и подпись `event_sig`.
- Добавлен обязательный `BERKUT_AUDIT_SIGNING_KEY` вне `APP_ENV=dev`.

### Роли
- Добавлены built-in роли:
  - `soc_viewer`
  - `soc_operator`
  - `backup_operator`
  - `compliance_manager`
- Расширены role templates для новых ролей.

### Инфраструктура и CI
- Добавлены CI-gate скрипты:
  - `scripts/ci/baseline_gate.sh`
  - `scripts/ci/latest1_upgrade_gate.sh`
- В `verify-go` добавлен запуск baseline/upgrade gate перед `make ci`.
- Добавлены тесты миграций и апгрейда latest-1.

### Документация
- Базовая версия документации приведена к `1.1.3`.
- Обновлены разделы deploy/security/api и индексы документации.
- Добавлены:
  - `docs/ru/security_baseline_prod.md`, `docs/eng/security_baseline_prod.md`
  - `docs/ru/compatibility_policy.md`, `docs/eng/compatibility_policy.md`
  - `docs/ru/upgrade_playbook.md`, `docs/eng/upgrade_playbook.md`

