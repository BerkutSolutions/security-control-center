# Журнал изменений

## 1.0.5 — 16.02.2026

### Добавлено

#### Архитектура и продукт
- Центральная лента событий (mini-SOC): `GET /api/logs` с серверными фильтрами `section/action/user/q/since/to/limit`.
- Экспорт ленты аудита в CSV: `GET /api/logs/export`.
- Сохранённые представления фильтров (saved views) во вкладке «Логи».
- Центр уведомлений мониторинга: шаблоны сообщений (`template_text`).
- Quiet hours для каналов уведомлений (`quiet_hours_enabled/start/end/tz`).
- История доставок уведомлений (`sent/failed/suppressed`) с причиной и preview.
- Подтверждение доставки уведомлений (acknowledge).
- Endpoint-ы уведомлений: `GET /api/monitoring/notifications/deliveries`, `POST /api/monitoring/notifications/deliveries/{id}/ack`.
- Case-management инцидентов: SLA-поля first response / resolve.
- Признаки просрочки SLA (`first_response_late`, `resolve_late`).
- Postmortem API: `PUT /api/incidents/{id}/postmortem`.
- Пакет для аудита: `GET /api/reports/audit-package`.
- Форматы audit package: `md`, `json`, `pdf`, `docx`.
- SHA-256 контроль экспорта в заголовке `X-Berkut-Export-SHA256`.

#### Документы и DLP-lite
- Политика dual-control экспорта для `DSP`/`CONFIDENTIAL` и классификационных тегов.
- Таблица `doc_export_approvals` для согласований экспорта.
- Endpoint согласования экспорта: `POST /api/docs/{id}/export-approve`.
- Одноразовое потребление согласования при фактическом экспорте.
- Endpoint фиксации событий безопасности документа: `POST /api/docs/{id}/security-events`.
- Типы document security events: `copy_blocked`, `screenshot_attempt`.
- Пункт «Согласовать экспорт» в UI документов.
- Переход экспорта документов на fetch-поток с отображением policy-ошибок.

#### Мониторинг и бэкапы
- Backup integrity API: `GET /api/backups/integrity`.
- Ручной запуск restore dry-run: `POST /api/backups/integrity/run`.
- Плановая автопроверка restore dry-run по расписанию.
- Новые параметры: `BERKUT_BACKUP_RESTORE_TEST_AUTO_ENABLED`, `BERKUT_BACKUP_RESTORE_TEST_INTERVAL_HOURS`.
- Hardening endpoint: `GET /api/settings/hardening`.
- Hardening score (security posture score) в Settings.
- Проверки hardening: TLS mode, trusted proxies, secrets health, session/password policy.

### Изменено

#### Безопасность
- Автосоздание задач при `DOWN` переведено на per-monitor модель.
- Чекбокс задач перенесён из глобальных настроек в модалку создания/редактирования монитора.
- В модалке монитора расположены рядом:
- `Создание инцидентов при недоступности`.
- `Создание задач при недоступности`.
- Добавлено поле монитора `auto_task_on_down`.
- Дефолт `auto_task_on_down` для новых мониторов — `OFF`.
- Ужесточён TTL сессии: server-side cap `<= 3h`.
- Default `BERKUT_SESSION_TTL` установлен в `3h`.
- Trusted proxy defaults ужесточены: широкие CIDR убраны из дефолтной конфигурации.
- TLS-материалы исключены из публикации (`*.crt`, `*.pem` в `.gitignore`).

#### UX и вкладки
- Удаление документа переведено с `confirm()` на внутреннее модальное окно.
- Модал подтверждения удаления стилизован в существующем стиле приложения.
- Исправлен роутинг документов: `/docs/{id}` корректно восстанавливается после перезагрузки.
- В Backups/Reports убраны inline styles для соответствия CSP `style-src 'self'`.
- В Backups -> Overview создание бэкапа теперь использует выбранный `scope/label/include_files`.

### Исправлено

#### Бэкенд и миграции
- Docker builder обновлён до `golang:1.24-alpine` (совместимость с `go.mod` `go >= 1.24`).
- PostgreSQL миграция `00014_notification_channels_quiet_hours.sql` устраняет `500` на `/api/monitoring/notifications`.
- PostgreSQL миграция `00015_sessions_revocation_columns.sql` добавляет совместимость старых схем `sessions`.
- PostgreSQL миграция `00016_monitors_auto_task_on_down.sql` добавляет per-monitor автоматизацию задач.
- Добавлена расширенная диагностика ошибки создания сессии при `/api/auth/login`.
- Синхронизированы JWT secret переменные OnlyOffice: `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET` и `ONLYOFFICE_JWT_SECRET`.

#### Автоматизация и SOC-сценарии
- Реализовано правило `repeated auth failures -> incident (auth_lockout)`.
- Добавлена дедупликация инцидентов автолокаута.
- Добавлено авто-закрытие auth_lockout-инцидента после успешного логина.
- Реализовано правило `monitor DOWN -> auto task`.
- Реализовано правило `TLS expires < N days -> auto incident`.
- Добавлено авто-закрытие TLS-инцидента после выхода сертификата из порога.
- Добавлены аудит-события:
- `auth.lockout.incident.auto_create`.
- `auth.lockout.incident.auto_close`.
- `monitoring.task.auto_create`.
- `monitoring.tls.incident.auto_create`.
- `monitoring.tls.incident.auto_close`.
- `monitoring.notification.delivery.ack`.

#### Локализация
- Исправлены повреждённые RU-строки (`????`) в Monitoring, Backups, Reports, Hardening, Logs.
- Исправлены fallback-тексты с проблемной кодировкой в UI.
- Закрыты пропуски i18n-ключей для мониторинга автоматизаций и логов.
- Добавлены/выравнены RU/EN ключи для новых полей уведомлений и статусов доставки.

### Тесты и валидация
- Добавлены тесты reverse-proxy security (`Secure` cookie / `HSTS`).
- Добавлены anti-spoofing тесты для `X-Forwarded-For`.
- Добавлены config-gate тесты trusted proxies и forwarded scheme.
- Добавлен i18n regression-тест на непустые переводы RU/EN.
- Добавлен i18n regression-тест на совпадение placeholders RU/EN.
- Добавлены тесты backups guard-обёрток (zero-trust доступ на endpoint-ах).
- Обновлены monitoring automation тесты (DOWN/TLS сценарии).
- Актуализированы task fixtures под обязательный `space_id`.
- Полный прогон `go test ./... -count=1` проходит успешно.

### Версия
- Версия приложения обновлена до `1.0.5`.
