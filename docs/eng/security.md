# Security

## Authentication
- Password hashing: Argon2id + salt + pepper.
- Sessions: `berkut_session` and CSRF cookie `berkut_csrf`.
- Login rate limiting.

## Authorization
- Server-side zero-trust model: permission checks on every endpoint.
- RBAC (Casbin, deny-by-default).
- ACL and classification/clearance checks in domain modules.

## Data protection
- Runtime storage: PostgreSQL.
- Encryption for sensitive content and attachments.
- Critical audit records in `audit_log`.

## Web security
- CSRF protection for state-changing requests.
- Multipart size limits (`413 Payload Too Large`).
- HTML/URL sanitization in markdown/docx rendering.

## Network and TLS
- Reverse-proxy TLS termination is recommended (Nginx/Traefik).
- Built-in TLS mode is supported.
- HTTPS configuration changes are audited.
