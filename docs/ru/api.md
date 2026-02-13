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
- Logs: `/api/logs`
- HTTPS settings: `GET/PUT /api/settings/https`

## Важно
- Все state-changing запросы требуют CSRF.
- Сервер всегда выполняет permission-check.

## Мониторинг: детали
- Типы мониторов, поддерживаемые backend:
  - `http`, `tcp`, `ping`, `http_keyword`, `http_json`, `grpc_keyword`, `dns`, `docker`, `push`, `steam`, `gamedig`, `mqtt`, `kafka_producer`, `mssql`, `postgres`, `mysql`, `mongodb`, `radius`, `redis`, `tailscale_ping`.
- Ручная проверка:
  - `POST /api/monitoring/monitors/{id}/check-now`
  - При конкурентной проверке возвращается `monitoring.error.busy`.
- Пассивный push ingestion:
  - `POST /api/monitoring/monitors/{id}/push`
  - Пример payload: `{ "ok": true, "latency_ms": 42, "status_code": 200, "error": "" }`
