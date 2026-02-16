# Runbook (start and recovery)

## 1. Start with Docker Compose
```bash
docker compose up -d --build
```

Check status:
```bash
docker compose ps
```

## 2. View logs
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 3. Common issues

### 3.1 Port 8080 already in use
Symptom: `Bind for 0.0.0.0:8080 failed`.

Fix:
- free the port on host, or
- change `PORT` in `.env` and restart compose.

### 3.2 Migration/startup errors after failed update
Try explicit migration run first:
```bash
docker compose run --rm berkut /usr/local/bin/berkut-migrate
docker compose up -d berkut
```

Recovery (destructive):
```bash
docker compose down -v
docker compose up -d --build
```

### 3.3 Default secrets error outside dev
Symptom: `default secrets are not allowed outside APP_ENV=dev`.

Fix:
- for dev: `ENV=dev`, `APP_ENV=dev`;
- for prod: set real values for `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`.

## 4. Full recovery
WARNING: removes PostgreSQL and file storage data.
```bash
docker compose down -v
docker compose up -d --build
```

## 5. Availability checks
- UI: `http://localhost:8080/login`
- Container health: `docker compose ps`
