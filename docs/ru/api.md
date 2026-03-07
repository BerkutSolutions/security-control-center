# API

Base path: `/api`

## РћСЃРЅРѕРІРЅС‹Рµ РіСЂСѓРїРїС‹
- Auth: `/api/auth/*`, `/api/app/*`
- Accounts: `/api/accounts/*`
- Docs/Approvals/Templates: `/api/docs/*`, `/api/approvals/*`, `/api/templates/*`
- Reports: `/api/reports/*`
- Incidents: `/api/incidents/*`
- Tasks: `/api/tasks/*`
- Monitoring: `/api/monitoring/*`
- Backups: `/api/backups/*`
- Logs: `/api/logs`, `/api/logs/export`, `/api/logs/export/package`
- HTTPS settings: `GET/PUT /api/settings/https`

## Р’Р°Р¶РЅРѕ
- Р’СЃРµ state-changing Р·Р°РїСЂРѕСЃС‹ С‚СЂРµР±СѓСЋС‚ CSRF.
- РЎРµСЂРІРµСЂ РІСЃРµРіРґР° РІС‹РїРѕР»РЅСЏРµС‚ permission-check.

## РџСЂРёР»РѕР¶РµРЅРёРµ: СЃРѕРІРјРµСЃС‚РёРјРѕСЃС‚СЊ РІРєР»Р°РґРѕРє Рё jobs (v1.0.13)
- Compat:
  - `GET /api/app/compat`
- Preflight (РїСЂРѕРІРµСЂРєРё Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°):
  - `GET /api/app/preflight`
- Jobs (СЂСѓС‡РЅС‹Рµ РѕРїРµСЂР°С†РёРё Partial adapt / Full reset, Р±РµР· Р°РІС‚Рѕ-РјРёРіСЂР°С†РёР№):
  - `POST /api/app/jobs`
  - `GET /api/app/jobs`
  - `GET /api/app/jobs/{id}`
  - `POST /api/app/jobs/{id}/cancel`

РџСЂР°РІР°:
- `app.compat.view`
- `app.compat.manage.partial`
- `app.compat.manage.full`
- `app.preflight.view`

## Auth (v1.0.13)

РџСѓР±Р»РёС‡РЅС‹Рµ endpoints:
- `POST /api/auth/login`
- `POST /api/auth/login/2fa` (РїРѕРґС‚РІРµСЂР¶РґРµРЅРёРµ РІС‚РѕСЂРѕРіРѕ С„Р°РєС‚РѕСЂР° РїРѕСЃР»Рµ РїР°СЂРѕР»СЏ)
- Passkeys (WebAuthn) login:
  - `POST /api/auth/passkeys/login/begin`
  - `POST /api/auth/passkeys/login/finish`
- Passkeys (WebAuthn) РєР°Рє РІС‚РѕСЂРѕР№ С„Р°РєС‚РѕСЂ:
  - `POST /api/auth/login/2fa/passkey/begin`
  - `POST /api/auth/login/2fa/passkey/finish`

Session endpoints (РЅСѓР¶РЅР° СЃРµСЃСЃРёСЏ + РїСЂР°РІР° РІРєР»Р°РґРєРё):
- 2FA (TOTP):
  - `GET /api/auth/2fa/status`
  - `POST /api/auth/2fa/setup`
  - `POST /api/auth/2fa/enable`
  - `POST /api/auth/2fa/disable`
- Passkeys (СѓРїСЂР°РІР»РµРЅРёРµ РєР»СЋС‡Р°РјРё РґРѕСЃС‚СѓРїР°):
  - `GET /api/auth/passkeys`
  - `POST /api/auth/passkeys/register/begin`
  - `POST /api/auth/passkeys/register/finish`
  - `PUT /api/auth/passkeys/{id}/rename`
  - `DELETE /api/auth/passkeys/{id}`

РџСЂРёРјРµС‡Р°РЅРёСЏ:
- UI РґР»СЏ РїРѕРґС‚РІРµСЂР¶РґРµРЅРёСЏ TOTP/recovery РЅР°С…РѕРґРёС‚СЃСЏ РЅР° `/login/2fa` (С‡С‚РѕР±С‹ РјРµРЅРµРґР¶РµСЂС‹ РїР°СЂРѕР»РµР№ РїРѕРґС…РІР°С‚С‹РІР°Р»Рё `one-time-code`).
- Passkeys С‚СЂРµР±СѓСЋС‚ HTTPS (РёР»Рё `localhost`) Рё РєРѕСЂСЂРµРєС‚РЅРѕР№ РєРѕРЅС„РёРіСѓСЂР°С†РёРё `security.webauthn.*`.

## Р‘СЌРєР°РїС‹ (v1.1.5)
РћСЃРЅРѕРІРЅС‹Рµ endpoint:
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

РџСЂР°РІР°:
- `backups.read`, `backups.create`, `backups.import`, `backups.download`, `backups.delete`, `backups.restore`, `backups.plan.update`.

## РњРѕРЅРёС‚РѕСЂРёРЅРі (v1.0.13)
- РўРёРїС‹ РјРѕРЅРёС‚РѕСЂРѕРІ, РїРѕРґРґРµСЂР¶РёРІР°РµРјС‹Рµ backend:
  - `http`, `tcp`, `ping`, `http_keyword`, `http_json`, `grpc_keyword`, `dns`, `docker`, `push`, `steam`, `gamedig`, `mqtt`, `kafka_producer`, `mssql`, `postgres`, `mysql`, `mongodb`, `radius`, `redis`, `tailscale_ping`.
- РџР°СЃСЃРёРІРЅС‹Р№ push ingestion:
  - `POST /api/monitoring/monitors/{id}/push`
  - РџСЂРёРјРµСЂ payload: `{ "ok": true, "latency_ms": 42, "status_code": 200, "error": "" }`

РћСЃРЅРѕРІРЅС‹Рµ endpoint:
- Engine stats (РґРёР°РіРЅРѕСЃС‚РёРєР° РґРІРёР¶РєР°/РїР»Р°РЅРёСЂРѕРІС‰РёРєР°):
  - `GET /api/monitoring/engine/stats`
- РњРѕРЅРёС‚РѕСЂС‹:
  - `GET /api/monitoring/monitors`
  - `POST /api/monitoring/monitors`
  - `GET /api/monitoring/monitors/{id}`
  - `PUT /api/monitoring/monitors/{id}`
  - `DELETE /api/monitoring/monitors/{id}`
  - `POST /api/monitoring/monitors/{id}/pause`
  - `POST /api/monitoring/monitors/{id}/resume`
  - `POST /api/monitoring/monitors/{id}/clone`
  - `POST /api/monitoring/monitors/{id}/push`
- РЎРѕСЃС‚РѕСЏРЅРёРµ/РјРµС‚СЂРёРєРё/СЃРѕР±С‹С‚РёСЏ:
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
- TLS/СЃРµСЂС‚РёС„РёРєР°С‚С‹:
  - `GET /api/monitoring/monitors/{id}/tls`
  - `GET /api/monitoring/certs`
  - `POST /api/monitoring/certs/test-notification`
- Maintenance:
  - `GET /api/monitoring/maintenance`
  - `POST /api/monitoring/maintenance`
  - `PUT /api/monitoring/maintenance/{id}`
  - `POST /api/monitoring/maintenance/{id}/stop`
  - `DELETE /api/monitoring/maintenance/{id}`
- РќР°СЃС‚СЂРѕР№РєРё:
  - `GET /api/monitoring/settings`
  - `PUT /api/monitoring/settings`
- РљР°РЅР°Р»С‹ СѓРІРµРґРѕРјР»РµРЅРёР№:
  - `GET /api/monitoring/notifications`
  - `POST /api/monitoring/notifications`
  - `PUT /api/monitoring/notifications/{id}`
  - `DELETE /api/monitoring/notifications/{id}`
  - `POST /api/monitoring/notifications/{id}/test`
  - `GET /api/monitoring/monitors/{id}/notifications`
  - `PUT /api/monitoring/monitors/{id}/notifications`

SLA-РѕСЃРѕР±РµРЅРЅРѕСЃС‚Рё:
- Р—Р°РєСЂС‹С‚С‹Рµ РїРµСЂРёРѕРґС‹ (`day/week/month`) СЂР°СЃСЃС‡РёС‚С‹РІР°СЋС‚СЃСЏ С„РѕРЅРѕРІС‹Рј evaluator (scheduler), Р° РЅРµ РєРЅРѕРїРєРѕР№ UI.
- РЎС‚Р°С‚СѓСЃ РїРµСЂРёРѕРґР°:
  - `ok` вЂ” С†РµР»СЊ SLA РІС‹РїРѕР»РЅРµРЅР°;
  - `violated` вЂ” С†РµР»СЊ SLA РЅР°СЂСѓС€РµРЅР°;
  - `unknown` вЂ” РЅРµРґРѕСЃС‚Р°С‚РѕС‡РЅРѕ РїРѕРєСЂС‹С‚РёСЏ РёР·РјРµСЂРµРЅРёСЏРјРё.
- SLA-РёРЅС†РёРґРµРЅС‚ СЃРѕР·РґР°РµС‚СЃСЏ С‚РѕР»СЊРєРѕ РїСЂРё Р·Р°РєСЂС‹С‚РёРё РІС‹Р±СЂР°РЅРЅРѕРіРѕ РїРµСЂРёРѕРґР° Рё С‚РѕР»СЊРєРѕ РїСЂРё РІРєР»СЋС‡РµРЅРЅРѕР№ policy.

## API update notes (1.1.5)
- Critical endpoints require fresh step-up verification (15-minute window): log purge requests/approve, runtime/https updates, privileged account/group/role mutations.
- `GET /api/monitoring/monitors/{id}/metrics` now includes forensic/debug fields: `final_url`, `remote_ip`, `response_headers`.

