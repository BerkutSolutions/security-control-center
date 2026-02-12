# Wiki: функциональные блоки

## Authentication
Login/logout/me, cookie sessions, CSRF, password policy.

## Authorization
RBAC + ACL + classification/clearance, zero-trust на сервере.

## Audit
Логирование критичных действий в `audit_log`.

## i18n
RU/EN локализация через `gui/static/i18n/*.json`.

## HTTPS and network
Reverse proxy (recommended) или built-in TLS.
Trusted proxies и аудит изменений HTTPS.

## Storage and encryption
SQLite + encrypted content/attachments + persistent volumes.

## Import/export/conversion
Импорт/экспорт документов и отчетов, локальная конвертация.

## Deploy and CI/CD
Docker/compose, GitLab pipeline, rollback.
