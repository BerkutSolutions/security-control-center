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
