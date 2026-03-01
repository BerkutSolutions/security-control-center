# Журнал изменений

## 1.0.13 — 01.03.2026

### Ядро
- Observability: добавлены технические endpoints `GET /healthz` (liveness) и `GET /readyz` (readiness, ping БД).
- Observability: добавлен `GET /metrics` для Prometheus (по умолчанию выключен; включение через `BERKUT_METRICS_ENABLED=true`).
- Observability: добавлены доменные метрики для `app_jobs`, `backups` и движка мониторинга, а также базовые tick-метрики фоновых воркеров.
- Observability: добавлен флаг `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=true` (только для режима `deployment_mode=home`) для удобства dev-скрейпа без токена.
- Auth: добавлен 2FA (TOTP) с challenge-логином и server-side хранением recovery codes; секрет TOTP хранится зашифрованным в `users.totp_secret_enc`.
- Auth: добавлены passkeys (WebAuthn): вход по ключу доступа и возможность подтверждать 2FA через passkey.
- Upgrade: добавлен preflight endpoint `GET /api/app/preflight` (проверки администратора).
- Upgrade: добавлен опциональный pre-upgrade backup перед миграциями (`BERKUT_UPGRADE_BACKUP_BEFORE_MIGRATE=true`).
- Миграции: применяются через session-lock в Postgres для защиты от параллельных запусков в HA.
- Monitoring: усилена защита SSRF при проверках (link-local/metadata адреса блокируются даже при включённых приватных сетях).
- OnlyOffice: callback download URL принимается только для настроенных host (Public/Internal), чтобы исключить SSRF через callback.

### Безопасность
- Observability: `GET /metrics` может быть защищён Bearer-токеном `BERKUT_METRICS_TOKEN` (заголовок `Authorization: Bearer ...`).
- Upgrade: добавлено право `app.preflight.view` для preflight checks.
- HTTP hardening: усилены security headers (CSP + дополнительные заголовки политики браузера).
- Trusted proxies: слишком широкие CIDR-диапазоны игнорируются при доверии `X-Forwarded-*`/`X-Real-IP` (защита от спуфинга).
- Auth: добавлен отдельный rate limit на второй фактор (`POST /api/auth/login/2fa`) и аудит событий `auth.2fa.*`.
- Auth: добавлен аудит событий passkeys `auth.passkey.*` (регистрация/переименование/удаление/вход/использование как 2FA).
- Security: добавлен аудит блокировок SSRF с `reason_code` (`security.ssrf.blocked`).
- Config: добавлены проверки strength для `BERKUT_METRICS_TOKEN` и `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET` (минимальная длина).

### UI
- Preflight: добавлены сообщения для проверок `Trusted proxies`, `/metrics` (токен) и ограничений internal URL для OnlyOffice.
- Monitoring: добавлен локализованный статус ошибки для заблокированных целей (network policy).
- Auth: добавлены экраны/модалки 2FA (TOTP): шаг 2FA при входе, включение/выключение в настройках, сброс 2FA супер-админом в Accounts.
- Auth: шаг подтверждения 2FA вынесен на отдельную страницу `/login/2fa`, чтобы менеджеры паролей (KeePassXC) корректно подхватывали поле `one-time-code`.
- Auth: добавлены UI-кнопки “войти с ключом доступа” (login) и управление passkeys в Settings.
- Healthcheck: добавлена секция “Preflight (админ)” с загрузкой отчёта `GET /api/app/preflight` (если есть право).

### Инфраструктура / Deploy
- Docker Compose: healthcheck контейнера приложения переведён с `/login` на `/readyz`.
- Конфигурация: в `config/app.yaml` добавлен блок `observability` (metrics выключены по умолчанию).
- HA: добавлен режим запуска `BERKUT_RUN_MODE=all|api|worker` и пример `api + worker` compose.

### Документация
- Обновлены runbook/deploy-инструкции: добавлены `healthz/readyz` и описание Prometheus `/metrics`.
- Auth: описан 2FA (TOTP + recovery codes) и сценарии восстановления доступа.
- Auth: описаны passkeys (WebAuthn) и базовая конфигурация `security.webauthn`.
