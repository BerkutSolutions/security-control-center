# Wiki: функциональные блоки

## Authentication
Login/logout/me, cookie sessions, CSRF, password policy.

## Authorization
RBAC + ACL + classification/clearance, zero-trust проверки на сервере.

## Audit
Логирование критичных действий в `audit_log`.

## i18n
RU/EN локализация через `gui/static/i18n/*.json`.

## HTTPS and network
Reverse proxy (рекомендуется) или built-in TLS.
Trusted proxies и аудит изменений HTTPS-конфига.

## Storage and encryption
PostgreSQL runtime + шифрование чувствительного контента/вложений + persistent volumes.

## Import/export/conversion
Импорт/экспорт документов и отчетов, локальная конвертация.

## Мониторинг и SLA
- Мониторы, метрики, события, maintenance windows, каналы уведомлений.
- SLA-вкладка с coverage-aware расчетом окон `24h/7d/30d`.
- Закрытые периоды `day/week/month` и отложенное создание SLA-инцидентов на закрытии периода.

## Бэкапы (.bscc)
- Создание/импорт/скачивание/удаление бэкапов.
- Dry-run/restore с шагами прогресса.
- Планировщик автобэкапов и retention policy.

## Deploy and CI/CD
Docker/Compose, verify pipeline, rollback.
