# Security Baseline (prod)

Документ фиксирует обязательные требования безопасности для production-контура.

## Mandatory

1. `APP_ENV=prod`, default secrets запрещены.
2. Включен TLS:
   - built-in (`BERKUT_TLS_ENABLED=true`) или
   - reverse-proxy TLS с явно заданными `BERKUT_SECURITY_TRUSTED_PROXIES`.
3. `BERKUT_SECURITY_TRUSTED_PROXIES` не пуст и не содержит слишком широких CIDR.
4. Метрики включены только с токеном:
   - `BERKUT_METRICS_ENABLED=true`
   - `BERKUT_METRICS_TOKEN` задан
   - `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=false`
5. Подпись audit-цепочки включена:
   - `BERKUT_AUDIT_SIGNING_KEY` задан.
6. Для WebAuthn в prod заданы:
   - `BERKUT_WEBAUTHN_RP_ID`
   - `BERKUT_WEBAUTHN_ORIGINS`

## Recommended

1. Обязательная 2FA для администраторов.
2. Регулярная проверка `/api/app/preflight` перед обновлением.
3. Регулярная проверка `/api/settings/hardening`.
4. Запрет доступа к management-интерфейсам из недоверенных сетей.

## Проверка

- Используйте `GET /api/app/preflight` для машинной проверки baseline.
- Для расследований используйте `GET /api/logs/export/package`.
