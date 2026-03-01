# API

Base path: `/api`

## Основные группы
- Auth: `/api/auth/*`, `/api/app/*`
- Accounts: `/api/accounts/*`
- Docs/Approvals/Templates: `/api/docs/*`, `/api/approvals/*`, `/api/templates/*`
- Reports: `/api/reports/*`
- Incidents: `/api/incidents/*`
- Tasks: `/api/tasks/*`
- Monitoring: `/api/monitoring/*`
- Backups: `/api/backups/*`
- Logs: `/api/logs`
- HTTPS settings: `GET/PUT /api/settings/https`

## Важно
- Все state-changing запросы требуют CSRF.
- Сервер всегда выполняет permission-check.

## Приложение: совместимость вкладок и jobs (v1.0.13)
- Compat:
  - `GET /api/app/compat`
- Preflight (проверки администратора):
  - `GET /api/app/preflight`
- Jobs (ручные операции Partial adapt / Full reset, без авто-миграций):
  - `POST /api/app/jobs`
  - `GET /api/app/jobs`
  - `GET /api/app/jobs/{id}`
  - `POST /api/app/jobs/{id}/cancel`

Права:
- `app.compat.view`
- `app.compat.manage.partial`
- `app.compat.manage.full`
- `app.preflight.view`

## Auth (v1.0.13)

Публичные endpoints:
- `POST /api/auth/login`
- `POST /api/auth/login/2fa` (подтверждение второго фактора после пароля)
- Passkeys (WebAuthn) login:
  - `POST /api/auth/passkeys/login/begin`
  - `POST /api/auth/passkeys/login/finish`
- Passkeys (WebAuthn) как второй фактор:
  - `POST /api/auth/login/2fa/passkey/begin`
  - `POST /api/auth/login/2fa/passkey/finish`

Session endpoints (нужна сессия + права вкладки):
- 2FA (TOTP):
  - `GET /api/auth/2fa/status`
  - `POST /api/auth/2fa/setup`
  - `POST /api/auth/2fa/enable`
  - `POST /api/auth/2fa/disable`
- Passkeys (управление ключами доступа):
  - `GET /api/auth/passkeys`
  - `POST /api/auth/passkeys/register/begin`
  - `POST /api/auth/passkeys/register/finish`
  - `PUT /api/auth/passkeys/{id}/rename`
  - `DELETE /api/auth/passkeys/{id}`

Примечания:
- UI для подтверждения TOTP/recovery находится на `/login/2fa` (чтобы менеджеры паролей подхватывали `one-time-code`).
- Passkeys требуют HTTPS (или `localhost`) и корректной конфигурации `security.webauthn.*`.

## Бэкапы (v1.0.3)
Основные endpoint:
- `GET /api/backups`
- `GET /api/backups/{id}`
- `POST /api/backups`
- `POST /api/backups/import`
- `GET /api/backups/{id}/download`
- `DELETE /api/backups/{id}`
- `POST /api/backups/{id}/restore`
- `POST /api/backups/{id}/restore/dry-run`
- `GET /api/backups/restores/{restore_id}`
- `GET /api/backups/plan`
- `PUT /api/backups/plan`
- `POST /api/backups/plan/enable`
- `POST /api/backups/plan/disable`

Права:
- `backups.read`, `backups.create`, `backups.import`, `backups.download`, `backups.delete`, `backups.restore`, `backups.plan.update`.

## Мониторинг (v1.0.13)
- Типы мониторов, поддерживаемые backend:
  - `http`, `tcp`, `ping`, `http_keyword`, `http_json`, `grpc_keyword`, `dns`, `docker`, `push`, `steam`, `gamedig`, `mqtt`, `kafka_producer`, `mssql`, `postgres`, `mysql`, `mongodb`, `radius`, `redis`, `tailscale_ping`.
- Пассивный push ingestion:
  - `POST /api/monitoring/monitors/{id}/push`
  - Пример payload: `{ "ok": true, "latency_ms": 42, "status_code": 200, "error": "" }`

Основные endpoint:
- Engine stats (диагностика движка/планировщика):
  - `GET /api/monitoring/engine/stats`
- Мониторы:
  - `GET /api/monitoring/monitors`
  - `POST /api/monitoring/monitors`
  - `GET /api/monitoring/monitors/{id}`
  - `PUT /api/monitoring/monitors/{id}`
  - `DELETE /api/monitoring/monitors/{id}`
  - `POST /api/monitoring/monitors/{id}/pause`
  - `POST /api/monitoring/monitors/{id}/resume`
  - `POST /api/monitoring/monitors/{id}/clone`
  - `POST /api/monitoring/monitors/{id}/push`
- Состояние/метрики/события:
  - `GET /api/monitoring/monitors/{id}/state`
  - `GET /api/monitoring/monitors/{id}/metrics`
  - `DELETE /api/monitoring/monitors/{id}/metrics`
  - `GET /api/monitoring/monitors/{id}/events`
  - `DELETE /api/monitoring/monitors/{id}/events`
  - `GET /api/monitoring/events`
- SLA:
  - `GET /api/monitoring/sla/overview`
  - `GET /api/monitoring/sla/history`
  - `PUT /api/monitoring/monitors/{id}/sla-policy`
- TLS/сертификаты:
  - `GET /api/monitoring/monitors/{id}/tls`
  - `GET /api/monitoring/certs`
  - `POST /api/monitoring/certs/test-notification`
- Maintenance:
  - `GET /api/monitoring/maintenance`
  - `POST /api/monitoring/maintenance`
  - `PUT /api/monitoring/maintenance/{id}`
  - `POST /api/monitoring/maintenance/{id}/stop`
  - `DELETE /api/monitoring/maintenance/{id}`
- Настройки:
  - `GET /api/monitoring/settings`
  - `PUT /api/monitoring/settings`
- Каналы уведомлений:
  - `GET /api/monitoring/notifications`
  - `POST /api/monitoring/notifications`
  - `PUT /api/monitoring/notifications/{id}`
  - `DELETE /api/monitoring/notifications/{id}`
  - `POST /api/monitoring/notifications/{id}/test`
  - `GET /api/monitoring/monitors/{id}/notifications`
  - `PUT /api/monitoring/monitors/{id}/notifications`

SLA-особенности:
- Закрытые периоды (`day/week/month`) рассчитываются фоновым evaluator (scheduler), а не кнопкой UI.
- Статус периода:
  - `ok` — цель SLA выполнена;
  - `violated` — цель SLA нарушена;
  - `unknown` — недостаточно покрытия измерениями.
- SLA-инцидент создается только при закрытии выбранного периода и только при включенной policy.
