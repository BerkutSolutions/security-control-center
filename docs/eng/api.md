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
- Backups: `/api/backups/*`
- Logs: `/api/logs`
- HTTPS settings: `GET/PUT /api/settings/https`

## Notes
- State-changing requests require CSRF.
- All endpoints are enforced server-side with permission checks.

## Backups (v1.0.3)
Primary endpoints:
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

Permissions:
- `backups.read`, `backups.create`, `backups.import`, `backups.download`, `backups.delete`, `backups.restore`, `backups.plan.update`.

## Monitoring (v1.0.3)
- Monitor types currently supported by backend:
  - `http`, `tcp`, `ping`, `http_keyword`, `http_json`, `grpc_keyword`, `dns`, `docker`, `push`, `steam`, `gamedig`, `mqtt`, `kafka_producer`, `mssql`, `postgres`, `mysql`, `mongodb`, `radius`, `redis`, `tailscale_ping`.
- Passive push monitor ingestion:
  - `POST /api/monitoring/monitors/{id}/push`
  - Payload example: `{ "ok": true, "latency_ms": 42, "status_code": 200, "error": "" }`

Primary endpoints:
- Monitors:
  - `GET /api/monitoring/monitors`
  - `POST /api/monitoring/monitors`
  - `GET /api/monitoring/monitors/{id}`
  - `PUT /api/monitoring/monitors/{id}`
  - `DELETE /api/monitoring/monitors/{id}`
  - `POST /api/monitoring/monitors/{id}/pause`
  - `POST /api/monitoring/monitors/{id}/resume`
  - `POST /api/monitoring/monitors/{id}/clone`
  - `POST /api/monitoring/monitors/{id}/push`
- State/metrics/events:
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
- TLS/certificates:
  - `GET /api/monitoring/monitors/{id}/tls`
  - `GET /api/monitoring/certs`
  - `POST /api/monitoring/certs/test-notification`
- Maintenance:
  - `GET /api/monitoring/maintenance`
  - `POST /api/monitoring/maintenance`
  - `PUT /api/monitoring/maintenance/{id}`
  - `POST /api/monitoring/maintenance/{id}/stop`
  - `DELETE /api/monitoring/maintenance/{id}`
- Settings:
  - `GET /api/monitoring/settings`
  - `PUT /api/monitoring/settings`
- Notification channels:
  - `GET /api/monitoring/notifications`
  - `POST /api/monitoring/notifications`
  - `PUT /api/monitoring/notifications/{id}`
  - `DELETE /api/monitoring/notifications/{id}`
  - `POST /api/monitoring/notifications/{id}/test`
  - `GET /api/monitoring/monitors/{id}/notifications`
  - `PUT /api/monitoring/monitors/{id}/notifications`

SLA specifics:
- Closed periods (`day/week/month`) are calculated by background evaluator jobs, not by the UI save action.
- Period status:
  - `ok` means SLA target met;
  - `violated` means SLA target missed;
  - `unknown` means insufficient coverage.
- SLA incidents are created only on period close and only when the monitor policy enables it.
