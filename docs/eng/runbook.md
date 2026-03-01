# Runbook (start and recovery)

## 1. Start with Docker Compose
```bash
docker compose up -d --build
```

Check status:
```bash
docker compose ps
```

## 2. View logs
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 3. Common issues

### 3.1 Port 8080 already in use
Symptom: `Bind for 0.0.0.0:8080 failed`.

Fix:
- free the port on host, or
- change `PORT` in `.env` and restart compose.

### 3.2 Migration/startup errors after failed update
Try explicit migration run first:
```bash
docker compose run --rm berkut /usr/local/bin/berkut-migrate
docker compose up -d berkut
```

Recovery (destructive):
```bash
docker compose down -v
docker compose up -d --build
```

### 3.3 Default secrets error outside dev
Symptom: `default secrets are not allowed outside APP_ENV=dev`.

Fix:
- for dev: `ENV=dev`, `APP_ENV=dev`;
- for prod: set real values for `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`.

### 3.4 Lost access to 2FA (TOTP)
Symptom: after entering the password the system requires a 2FA code, but the authenticator app is unavailable.

Fix:
- use one of your recovery codes (spaces/hyphens are accepted);
- if recovery codes are lost — a superadmin can reset 2FA: `Accounts → Users → Reset 2FA`.

### 3.5 Passkeys (WebAuthn) not working / cancelled dialog
Symptoms:
- when adding/signing-in with a passkey: “operation cancelled/timed out”;
- `NotAllowedError`, `SecurityError` in devtools.

Causes / fixes:
- WebAuthn requires HTTPS (or `localhost`): use `--profile proxy` and `https://localhost`, or a domain with valid TLS.
- Check `security.webauthn` configuration (for prod set `rp_id` and `origins` explicitly).
- If 2FA is enabled, code entry is on a separate page: `http://localhost:8080/login/2fa` (or `https://.../login/2fa` behind proxy).

## 4. Full recovery
WARNING: removes PostgreSQL and file storage data.
```bash
docker compose down -v
docker compose up -d --build
```

## 5. Availability checks
- UI: `http://localhost:8080/login`
- Liveness: `http://localhost:8080/healthz`
- Readiness (DB ping): `http://localhost:8080/readyz`
- Container health: `docker compose ps`

## 6. Observability quick checks (Prometheus)
Metrics are disabled by default. To enable:
- set `BERKUT_METRICS_ENABLED=true`
- (recommended) set `BERKUT_METRICS_TOKEN` and scrape with `Authorization: Bearer ...`
- (home/dev only) allow unauthenticated scrape: `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=true`

Manual scrape example:
```bash
curl -fsS -H "Authorization: Bearer $BERKUT_METRICS_TOKEN" http://localhost:8080/metrics | head
```

Useful signals:
- `berkut_worker_last_tick_timestamp` — background workers are alive
- `berkut_app_jobs_oldest_queued_age_seconds` — stuck app jobs queue
- `berkut_monitoring_inflight_checks` and `berkut_monitoring_error_class_total` — monitoring health
- `berkut_backup_plan_last_auto_run_age_seconds` — backups are not running

### Minimal alerts (example)
1) Workers are “dead” (no ticks):
```promql
time() - berkut_worker_last_tick_timestamp > 300
```

2) app jobs queue is stuck:
```promql
berkut_app_jobs_oldest_queued_age_seconds > 600
```

3) Monitoring errors keep happening (by error class, except `ok`):
```promql
sum(rate(berkut_monitoring_error_class_total{class!=\"ok\"}[5m])) > 0
```

4) Auto-backups are not running (plan enabled):
```promql
berkut_backup_plan_enabled == 1 and berkut_backup_plan_last_auto_run_age_seconds > 86400
```
