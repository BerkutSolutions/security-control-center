# Security Baseline (prod)

This document defines mandatory production security requirements.

## Mandatory

1. `APP_ENV=prod`; default secrets are forbidden.
2. TLS is enabled:
   - built-in (`BERKUT_TLS_ENABLED=true`) or
   - reverse-proxy TLS with explicit `BERKUT_SECURITY_TRUSTED_PROXIES`.
3. `BERKUT_SECURITY_TRUSTED_PROXIES` is non-empty and must not include broad CIDRs.
4. Metrics require token in enterprise mode:
   - `BERKUT_METRICS_ENABLED=true`
   - `BERKUT_METRICS_TOKEN` is set
   - `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=false`
5. Audit chain signing is enabled:
   - `BERKUT_AUDIT_SIGNING_KEY` is set.
6. WebAuthn production settings are explicit:
   - `BERKUT_WEBAUTHN_RP_ID`
   - `BERKUT_WEBAUTHN_ORIGINS`

## Recommended

1. Enforce 2FA for administrator accounts.
2. Run `GET /api/app/preflight` before each upgrade.
3. Regularly review `GET /api/settings/hardening`.
4. Restrict management endpoints to trusted networks.

## Verification

- Use `GET /api/app/preflight` as a machine-checkable baseline gate.
- Use `GET /api/logs/export/package` for forensic-ready audit exports.
