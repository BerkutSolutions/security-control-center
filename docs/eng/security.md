# Security

## Authentication
- Password hashing: Argon2id + salt + pepper.
- Session cookies: `berkut_session` + `berkut_csrf`.
- Login rate limiting.

## Authorization
- RBAC (deny-by-default).
- Object ACL checks in domain handlers.
- Classification/clearance checks.

## Data protection
- SQLite for metadata/state.
- Encrypted sensitive content and attachments at rest.
- Critical operations logged in `audit_log`.

## HTTPS
- Recommended: reverse proxy TLS termination.
- Optional: built-in TLS mode.
- HTTPS config changes are audited (`settings.https.update`).
