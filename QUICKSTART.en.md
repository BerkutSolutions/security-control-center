# Quick Start (EN)

Minimal local startup for Berkut SCC.

## 1. Prepare
```bash
cp .env.example .env
```

For non-dev environments, make sure secrets in `.env` are not default values:
- `CSRF_KEY`
- `PEPPER`
- `DOCS_ENCRYPTION_KEY`
- `BACKUP_ENCRYPTION_KEY`

## 2. Start
```bash
docker compose up -d --build
```

## 3. Verify
- UI: `http://localhost:8080/login` (without reverse proxy) or `http://localhost/login` (if you started with `docker compose --profile proxy ...`).
- If 2FA is enabled, confirmation happens on a separate page: `/login/2fa`.
- Right after login, you may see a one-time `/healthcheck` page (probes/Compat) — click “Continue” to enter the app.
- Container status:
```bash
docker compose ps
```

## 4. Logs
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 5. Next Docs
- Full deploy and CI/CD: [`docs/eng/deploy.md`](docs/eng/deploy.md)
- Runbook (diagnostics/recovery): [`docs/eng/runbook.md`](docs/eng/runbook.md)
- HTTPS + OnlyOffice: [`docs/eng/https_onlyoffice.md`](docs/eng/https_onlyoffice.md)
- Reverse-proxy compose example: [`docs/ru/docker-compose.https.yml`](docs/ru/docker-compose.https.yml)
