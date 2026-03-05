# Журнал изменений

## 1.1.2 — 05.03.2026

### UI
- Добавлен новый формат экспорта журналов для расследований: `/api/logs/export/package` (zip-пакет с JSONL, manifest и checksums).

### Ядро
- Расширен preflight: добавлены baseline-проверки для production-контура (TLS, trusted proxies, metrics+token, audit signing key, WebAuthn prod-конфиг).
- Добавлен endpoint `GET /api/logs/export/package` с формированием forensic-пакета аудита.
- Расширен `AuditStore`: поддержка выгрузки записей с полями целостности (`prev_hash`, `event_hash`, `event_sig`).

### Вкладка / Логи
- Для раздела логов добавлен отдельный защищенный маршрут `GET /api/logs/export/package` (право `logs.view`).

### Безопасность
- `audit_log` переведен в append-only режим: UPDATE/DELETE блокируются на уровне БД (PostgreSQL и sqlite test runtime).
- В аудит добавлена цепочка целостности `prev_hash -> event_hash` и HMAC-подпись `event_sig`.
- В конфигурацию добавлен `BERKUT_AUDIT_SIGNING_KEY` (обязателен вне `APP_ENV=dev`, валидация минимальной длины включена).

### Документация
- Добавлены новые документы:
  - `docs/ru/security_baseline_prod.md`, `docs/eng/security_baseline_prod.md`
  - `docs/ru/compatibility_policy.md`, `docs/eng/compatibility_policy.md`
  - `docs/ru/upgrade_playbook.md`, `docs/eng/upgrade_playbook.md`
- Обновлены разделы deploy/security/api и индексы документации под новую версию и реалии.
- Добавлен локальный файл правил релизного процесса `LOCAL_RELEASE_RULES.md` (исключен из Git и Docker).

### Вкладка / Мониторинг
- Для метрик мониторинга добавлены forensic-поля: `final_url`, `remote_ip`, `response_headers`.
- В графике/tooltip мониторинга показываются дополнительные отладочные данные для `DOWN/ISSUE`.
- Статус `HTTP 404` теперь трактуется как `ISSUE` (оранжевый) при неразрешенном коде; при добавлении `404` в разрешенные статусы монитор остается `UP`.

### Безопасность
- Для критичных endpoint'ов добавлен обязательный свежий step-up (окно 15 минут):
  - операции purge логов;
  - изменение runtime/https hardening-настроек;
  - критичные операции управления аккаунтами/группами/ролями.

### Инфраструктура/Deploy
- Добавлены CI-gate скрипты:
  - `scripts/ci/baseline_gate.sh`
  - `scripts/ci/latest1_upgrade_gate.sh`
- В `verify-go` добавлен запуск baseline/upgrade gate перед `make ci`.
- Добавлены тесты:
  - `TestGooseMigrationsSequenceNoGaps`
  - `TestSQLiteLatestMinusOneUpgradeSmoke`

### Роли
- Добавлены новые built-in роли:
  - `soc_viewer`
  - `soc_operator`
  - `backup_operator`
  - `compliance_manager`
- Расширены role templates в аккаунтах для этих ролей.
