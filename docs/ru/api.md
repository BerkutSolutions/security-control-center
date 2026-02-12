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
