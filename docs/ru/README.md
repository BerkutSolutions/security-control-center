# Berkut SCC - Р”РѕРєСѓРјРµРЅС‚Р°С†РёСЏ (RU)



РђРєС‚СѓР°Р»СЊРЅР°СЏ РІРµСЂСЃРёСЏ РґРѕРєСѓРјРµРЅС‚Р°С†РёРё: `1.1.5`



## Р Р°Р·РґРµР»С‹

1. РђСЂС…РёС‚РµРєС‚СѓСЂР°: `docs/ru/architecture.md`

2. API: `docs/ru/api.md`

3. Р‘РµР·РѕРїР°СЃРЅРѕСЃС‚СЊ: `docs/ru/security.md`

4. РЎРѕРІРјРµСЃС‚РёРјРѕСЃС‚СЊ РІРєР»Р°РґРѕРє (Compat): `docs/ru/compatibility.md` (С€РїР°СЂРіР°Р»РєР°: `docs/ru/compatibility_cheatsheet.md`)

5. РџСЂРѕРІРµСЂРєР° СЃРѕСЃС‚РѕСЏРЅРёСЏ (/healthcheck): `docs/ru/healthcheck.md`

6. Р”РµРїР»РѕР№ Рё CI/CD: `docs/ru/deploy.md`

7. Runbook (Р·Р°РїСѓСЃРє Рё РІРѕСЃСЃС‚Р°РЅРѕРІР»РµРЅРёРµ): `docs/ru/runbook.md`

7.1 РћР±РЅРѕРІР»РµРЅРёРµ / РѕС‚РєР°С‚: `docs/ru/upgrade.md`

8. Wiki РїРѕ РІРєР»Р°РґРєР°Рј: `docs/ru/wiki/tabs.md`

9. Wiki РїРѕ С„СѓРЅРєС†РёРѕРЅР°Р»Сѓ: `docs/ru/wiki/features.md`

10. РђРєС‚СѓР°Р»СЊРЅС‹Р№ РїР»Р°РЅ СЂР°Р·РІРёС‚РёСЏ: `docs/ru/roadmap.md`

11. Р‘СЌРєР°РїС‹ (.bscc): `docs/ru/backups.md`

12. HTTPS + OnlyOffice: `docs/ru/https_onlyoffice.md`

13. РџСЂРёРјРµСЂ compose РґР»СЏ reverse-proxy + OnlyOffice: `docs/ru/docker-compose.https.yml`

13.1 РџСЂРёРјРµСЂ HA compose (api + worker): `docs/ru/docker-compose.ha.yml`


14. РЁР°Р±Р»РѕРЅ message СѓРІРµРґРѕРјР»РµРЅРёР№ РјРѕРЅРёС‚РѕСЂРёРЅРіР°: `docs/ru/monitoring_notifications_message_template.md`


## РљРѕРЅС‚РµРєСЃС‚

Р”РѕРєСѓРјРµРЅС‚Р°С†РёСЏ СЃРёРЅС…СЂРѕРЅРёР·РёСЂРѕРІР°РЅР° СЃ С‚РµРєСѓС‰РµР№ РјРѕРґРµР»СЊСЋ:

- PostgreSQL runtime

- goose РјРёРіСЂР°С†РёРё

- cleanenv-РєРѕРЅС„РёРіСѓСЂР°С†РёСЏ

- zero-trust РїСЂРѕРІРµСЂРєРё РґРѕСЃС‚СѓРїР° РЅР° СЃРµСЂРІРµСЂРµ

- РјРѕРґСѓР»СЊ Р±СЌРєР°РїРѕРІ `.bscc` (create/import/download/restore/plan/scheduler/retention)

- SLA-РјРѕРґСѓР»СЊ РјРѕРЅРёС‚РѕСЂРёРЅРіР° (РІРєР»Р°РґРєР° SLA, Р·Р°РєСЂС‹С‚С‹Рµ РїРµСЂРёРѕРґС‹, background evaluator, policy РёРЅС†РёРґРµРЅС‚РѕРІ)



## Р§С‚Рѕ СѓС‡С‚РµРЅРѕ РґР»СЏ 1.1.0

- UI/РЅР°РІРёРіР°С†РёСЏ: РІРєР»Р°РґРєР° В«Р РµРµСЃС‚СЂС‹В» (`/registry/...`) РІРєР»СЋС‡Р°РµС‚ РђРєС‚РёРІС‹/РџРћ/Р—Р°РјРµС‡Р°РЅРёСЏ РєР°Рє РІРЅСѓС‚СЂРµРЅРЅРёРµ РІРєР»Р°РґРєРё СЃ РјР°СЂС€СЂСѓС‚Р°РјРё РІРёРґР° `/registry/assets`, `/registry/software`, `/registry/findings`.

- Settings: РІС‹РґРµР»РµРЅР° РѕС‚РґРµР»СЊРЅР°СЏ РІРєР»Р°РґРєР° В«РћС‡РёСЃС‚РєР°В» СЃ РІС‹Р±РѕСЂРѕС‡РЅРѕР№ РѕС‡РёСЃС‚РєРѕР№ РґР°РЅРЅС‹С… РїРѕ РјРѕРґСѓР»СЏРј.

- РџСЂРѕРІРµСЂРєР° СЃРѕСЃС‚РѕСЏРЅРёСЏ: РґРѕР±Р°РІР»РµРЅР° РѕРґРЅРѕСЂР°Р·РѕРІР°СЏ СЃС‚СЂР°РЅРёС†Р° `/healthcheck` (РґРѕСЃС‚СѓРїРЅР° С‚РѕР»СЊРєРѕ СЃСЂР°Р·Сѓ РїРѕСЃР»Рµ РІС…РѕРґР°/СЃРјРµРЅС‹ РїР°СЂРѕР»СЏ) СЃ СЃРµСЂРёРµР№ probes Рё РѕС‚С‡С‘С‚РѕРј Compat РїРѕ РІРєР»Р°РґРєР°Рј.

- Compat: РґРѕР±Р°РІР»РµРЅС‹ `/api/app/compat` Рё jobs `/api/app/jobs*` РґР»СЏ СЂСѓС‡РЅРѕРіРѕ Partial adapt / Full reset (Р±РµР· Р°РІС‚Рѕ-РјРёРіСЂР°С†РёР№).

- Monitoring: РґРѕР±Р°РІР»РµРЅ С„Р»Р°Рі Р°РІС‚Рѕ-Р·Р°РєСЂС‹С‚РёСЏ РёРЅС†РёРґРµРЅС‚Р° РїСЂРё РІРѕСЃСЃС‚Р°РЅРѕРІР»РµРЅРёРё РјРѕРЅРёС‚РѕСЂР° (`DOWN -> UP`).

- Monitoring engine: РґРѕР±Р°РІР»РµРЅС‹ РґРµС‚РµСЂРјРёРЅРёСЂРѕРІР°РЅРЅС‹Р№ jitter due-РїР»Р°РЅРёСЂРѕРІР°РЅРёСЏ, scheduled retries (РѕРґРЅР° РїРѕРїС‹С‚РєР° = РѕРґРёРЅ СЃР»РѕС‚; Р±РµР· `sleep` РІРЅСѓС‚СЂРё СЃР»РѕС‚Р°) Рё `GET /api/monitoring/engine/stats` РґР»СЏ РґРёР°РіРЅРѕСЃС‚РёРєРё (inflight/due/skipped, p95 РѕР¶РёРґР°РЅРёСЏ СЃС‚Р°СЂС‚Р°/РґР»РёС‚РµР»СЊРЅРѕСЃС‚Рё, СЂР°СЃРїСЂРµРґРµР»РµРЅРёРµ РѕС€РёР±РѕРє).

- Р›РѕРєР°Р»РёР·Р°С†РёСЏ Рё UX: РІС‹СЂРѕРІРЅРµРЅС‹ РїСЂРѕР±Р»РµРјРЅС‹Рµ СЌР»РµРјРµРЅС‚С‹ РёРЅС‚РµСЂС„РµР№СЃР° Р»РѕРіРѕРІ/РјРѕРЅРёС‚РѕСЂРёРЅРіР° Рё Р·Р°РєСЂС‹С‚С‹ РїСЂРѕРїСѓСЃРєРё i18n.

- Backups: РѕР±РЅРѕРІР»С‘РЅ UX Р±Р»РѕРєР° В«РџР°СЂР°РјРµС‚СЂС‹ РЅРѕРІРѕРіРѕ Р±СЌРєР°РїР°В» Рё СЃС‚Р°Р±РёР»РёР·РёСЂРѕРІР°РЅ pipeline РІРѕСЃСЃС‚Р°РЅРѕРІР»РµРЅРёСЏ Р‘Р” (`pg_restore`).

- Compose/runtime: РґРѕР±Р°РІР»РµРЅР° РµРґРёРЅР°СЏ С‚Р°Р№РјР·РѕРЅР° РєРѕРЅС‚РµР№РЅРµСЂРѕРІ С‡РµСЂРµР· `TZ` (СЂРµРєРѕРјРµРЅРґСѓРµРјРѕРµ Р·РЅР°С‡РµРЅРёРµ `Europe/Moscow`).

- Observability: `GET /healthz` (liveness), `GET /readyz` (readiness, DB ping), Prometheus `GET /metrics` (РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ РІС‹РєР»СЋС‡РµРЅРѕ; РІРєР»СЋС‡РµРЅРёРµ С‡РµСЂРµР· `BERKUT_METRICS_ENABLED=true`).

- Upgrade: РґРѕР±Р°РІР»РµРЅ preflight `GET /api/app/preflight` (РїСЂРѕРІРµСЂРєРё Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°) Рё РѕРїС†РёРѕРЅР°Р»СЊРЅС‹Р№ backup-before-migrate.

- Auth: РґРѕР±Р°РІР»РµРЅС‹ 2FA (TOTP + recovery codes), passkeys (WebAuthn) Рё РѕС‚РґРµР»СЊРЅР°СЏ СЃС‚СЂР°РЅРёС†Р° РїРѕРґС‚РІРµСЂР¶РґРµРЅРёСЏ 2FA `/login/2fa` РґР»СЏ СЃРѕРІРјРµСЃС‚РёРјРѕСЃС‚Рё СЃ РјРµРЅРµРґР¶РµСЂР°РјРё РїР°СЂРѕР»РµР№.


15. Security baseline (prod): `docs/ru/security_baseline_prod.md`
16. Compatibility policy: `docs/ru/compatibility_policy.md`
17. Upgrade playbook: `docs/ru/upgrade_playbook.md`

18. Load profile (Medium-20): docs/ru/load_profile_medium20.md

19. OSS: Security — `docs/ru/oss/SECURITY.md`
20. OSS: Contributing — `docs/ru/oss/CONTRIBUTING.md`
21. OSS: Code of Conduct — `docs/ru/oss/CODE_OF_CONDUCT.md`
22. OSS: Support — `docs/ru/oss/SUPPORT.md`

