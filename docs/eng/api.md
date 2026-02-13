# API

Base path: `/api`

## Main groups
- Auth: `/api/auth/*`, `/api/app/*`
- Accounts: `/api/accounts/*`
- Docs/Approvals/Templates: `/api/docs/*`, `/api/approvals/*`, `/api/templates/*`
- Reports: `/api/reports/*`
- Incidents: `/api/incidents/*`
- Tasks: `/api/tasks/*`
- Monitoring: `/api/monitoring/*`
- Logs: `/api/logs`
- HTTPS settings: `GET/PUT /api/settings/https`

## Notes
- State-changing requests require CSRF.
- All endpoints are enforced server-side with permission checks.

## Monitoring specifics
- Monitor types currently supported by backend:
  - `http`, `tcp`, `ping`, `http_keyword`, `http_json`, `grpc_keyword`, `dns`, `docker`, `push`, `steam`, `gamedig`, `mqtt`, `kafka_producer`, `mssql`, `postgres`, `mysql`, `mongodb`, `radius`, `redis`, `tailscale_ping`.
- Manual check:
  - `POST /api/monitoring/monitors/{id}/check-now`
  - Returns `monitoring.error.busy` if a check is already in progress for the same monitor.
- Passive push monitor ingestion:
  - `POST /api/monitoring/monitors/{id}/push`
  - Payload example: `{ "ok": true, "latency_ms": 42, "status_code": 200, "error": "" }`
