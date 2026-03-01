# Security

## Authentication
- Password hashing: Argon2id + salt + pepper.
- Sessions: `berkut_session` and CSRF cookie `berkut_csrf`.
- Login rate limiting.
- Optional 2FA (TOTP): TOTP secret is stored encrypted (AES-GCM) in `users.totp_secret_enc`, recovery codes are one-time and stored as Argon2id hashes.
- 2FA reset: password + recovery code (self-service) or by a superadmin (Reset 2FA in Accounts).
- Optional passkeys (WebAuthn): allow passwordless sign-in and/or using a passkey as a second factor (e.g., KeePassXC, Windows Hello).
- 2FA confirmation is on a separate page (`/login/2fa`) so password managers can detect the `one-time-code` field reliably.

### Passkeys (WebAuthn) configuration
Configuration in `config/app.yaml`:

```yaml
security:
  webauthn:
    enabled: true
    rp_id: ""      # can be derived from host in home/dev; for production set explicitly (e.g. scc.example.com)
    rp_name: "Berkut SCC"
    origins: []    # can be derived from request origin in home/dev; for production set explicitly (e.g. ["https://scc.example.com"])
```

Requirements:
- WebAuthn requires a secure context (HTTPS) or `localhost`.
- For corporate deployments, set `rp_id` and `origins` explicitly to avoid configuration errors.

## Authorization
- Server-side zero-trust model: permission checks on every endpoint.
- RBAC (Casbin, deny-by-default).
- ACL and classification/clearance checks in domain modules.

## Data protection
- Runtime storage: PostgreSQL.
- Encryption for sensitive content and attachments.
- Critical audit records in `audit_log`.
- SSRF block audits (e.g., monitoring/OnlyOffice restricted targets): `security.ssrf.blocked`.

## Web security
- CSRF protection for state-changing requests.
- Multipart size limits (`413 Payload Too Large`).
- HTML/URL sanitization in markdown/docx rendering.

## Network and TLS
- Reverse-proxy TLS termination is recommended (Nginx/Traefik).
- Built-in TLS mode is supported.
- HTTPS configuration changes are audited.
