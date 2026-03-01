# Безопасность

## Аутентификация
- Хеширование паролей: Argon2id + salt + pepper.
- Сессии: `berkut_session` и CSRF-cookie `berkut_csrf`.
- Ограничение частоты попыток входа.
- 2FA (TOTP) опционально: секрет хранится зашифрованным (AES-GCM) в `users.totp_secret_enc`, recovery codes одноразовые и хранятся как Argon2id-хеши.
- Сброс 2FA: через password + recovery code (self-service) или супер-админом (Reset 2FA в Accounts).
- Ключи доступа (passkeys / WebAuthn) опционально: позволяют входить без пароля и/или подтверждать 2FA аппаратным/системным ключом (например, KeePassXC, Windows Hello).
- Вход с 2FA вынесен на отдельную страницу `/login/2fa`, чтобы менеджеры паролей могли корректно подхватывать поле `one-time-code`.

### Passkeys (WebAuthn) — конфигурация
Настройка в `config/app.yaml`:

```yaml
security:
  webauthn:
    enabled: true
    rp_id: ""      # в home/dev может выводиться из host; в prod лучше указать явно (например: scc.example.com)
    rp_name: "Berkut SCC"
    origins: []    # в home/dev может выводиться из request origin; в prod лучше указать явно (например: ["https://scc.example.com"])
```

Требования:
- WebAuthn работает только в secure context (HTTPS) или на `localhost`.
- Для корпоративного деплоя рекомендуется задать `rp_id` и `origins` явно, чтобы избежать ошибок конфигурации.

## Авторизация
- Серверная модель zero-trust: проверка прав на каждом endpoint.
- RBAC (Casbin, deny-by-default).
- ACL и проверки классификации/допуска в доменных модулях.

## Защита данных
- Runtime-хранилище: PostgreSQL.
- Шифрование чувствительного контента и вложений.
- Аудит критичных действий в `audit_log`.
- Аудит блокировок SSRF (например, запретные цели мониторинга/OnlyOffice): `security.ssrf.blocked`.

## Веб-безопасность
- CSRF-защита для state-changing запросов.
- Ограничение размера multipart-запросов (`413 Payload Too Large`).
- Санитизация HTML/URL при рендеринге markdown/docx.

## Сеть и TLS
- Рекомендуется TLS-терминация на reverse proxy (Nginx/Traefik).
- Поддерживается встроенный TLS-режим.
- Изменения HTTPS-конфигурации аудируются.
