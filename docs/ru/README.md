# Berkut SCC - Документация (RU)



Актуальная версия документации: `1.0.13`



## Разделы

1. Архитектура: `docs/ru/architecture.md`

2. API: `docs/ru/api.md`

3. Безопасность: `docs/ru/security.md`

4. Совместимость вкладок (Compat): `docs/ru/compatibility.md` (шпаргалка: `docs/ru/compatibility_cheatsheet.md`)

5. Проверка состояния (/healthcheck): `docs/ru/healthcheck.md`

6. Деплой и CI/CD: `docs/ru/deploy.md`

7. Runbook (запуск и восстановление): `docs/ru/runbook.md`

7.1 Обновление / откат: `docs/ru/upgrade.md`

8. Wiki по вкладкам: `docs/ru/wiki/tabs.md`

9. Wiki по функционалу: `docs/ru/wiki/features.md`

10. Актуальный план развития: `docs/ru/roadmap.md`

11. Бэкапы (.bscc): `docs/ru/backups.md`

12. HTTPS + OnlyOffice: `docs/ru/https_onlyoffice.md`

13. Пример compose для reverse-proxy + OnlyOffice: `docs/ru/docker-compose.https.yml`

13.1 Пример HA compose (api + worker): `docs/ru/docker-compose.ha.yml`


14. Шаблон message уведомлений мониторинга: `docs/ru/monitoring_notifications_message_template.md`


## Контекст

Документация синхронизирована с текущей моделью:

- PostgreSQL runtime

- goose миграции

- cleanenv-конфигурация

- zero-trust проверки доступа на сервере

- модуль бэкапов `.bscc` (create/import/download/restore/plan/scheduler/retention)

- SLA-модуль мониторинга (вкладка SLA, закрытые периоды, background evaluator, policy инцидентов)



## Что учтено для 1.0.13

- UI/навигация: вкладка «Реестры» (`/registry/...`) включает Активы/ПО/Замечания как внутренние вкладки с маршрутами вида `/registry/assets`, `/registry/software`, `/registry/findings`.

- Settings: выделена отдельная вкладка «Очистка» с выборочной очисткой данных по модулям.

- Проверка состояния: добавлена одноразовая страница `/healthcheck` (доступна только сразу после входа/смены пароля) с серией probes и отчётом Compat по вкладкам.

- Compat: добавлены `/api/app/compat` и jobs `/api/app/jobs*` для ручного Partial adapt / Full reset (без авто-миграций).

- Monitoring: добавлен флаг авто-закрытия инцидента при восстановлении монитора (`DOWN -> UP`).

- Monitoring engine: добавлены детерминированный jitter due-планирования, scheduled retries (одна попытка = один слот; без `sleep` внутри слота) и `GET /api/monitoring/engine/stats` для диагностики (inflight/due/skipped, p95 ожидания старта/длительности, распределение ошибок).

- Локализация и UX: выровнены проблемные элементы интерфейса логов/мониторинга и закрыты пропуски i18n.

- Backups: обновлён UX блока «Параметры нового бэкапа» и стабилизирован pipeline восстановления БД (`pg_restore`).

- Compose/runtime: добавлена единая таймзона контейнеров через `TZ` (рекомендуемое значение `Europe/Moscow`).

- Observability: `GET /healthz` (liveness), `GET /readyz` (readiness, DB ping), Prometheus `GET /metrics` (по умолчанию выключено; включение через `BERKUT_METRICS_ENABLED=true`).

- Upgrade: добавлен preflight `GET /api/app/preflight` (проверки администратора) и опциональный backup-before-migrate.

- Auth: добавлены 2FA (TOTP + recovery codes), passkeys (WebAuthn) и отдельная страница подтверждения 2FA `/login/2fa` для совместимости с менеджерами паролей.
