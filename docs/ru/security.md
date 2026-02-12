# Безопасность

## Аутентификация
- Пароли: Argon2id + salt + pepper.
- Сессии: cookie `berkut_session` + `berkut_csrf`.
- Login rate limit.

## Авторизация
- RBAC (deny-by-default).
- ACL на объекты доменов.
- Classification/clearance проверки.

## Данные
- SQLite для метаданных.
- Шифрование чувствительных данных/вложений на диске.
- Аудит критичных действий в `audit_log`.

## HTTPS
- Рекомендуется reverse proxy (Nginx/Traefik).
- Поддерживается built-in TLS.
- Изменение HTTPS-конфига аудируется (`settings.https.update`).
