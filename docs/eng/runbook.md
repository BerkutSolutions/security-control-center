# Runbook (start and recovery)

## 1. Local start (docker run)
```bash
docker rm -f berkut-scc || true
docker build -t berkut-scc -f docker/Dockerfile .
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 2. Start with docker compose
```bash
docker compose up -d --build
```

## 3. Common error: readonly SQLite
Symptom: `attempt to write a readonly database (8)`.

Fix volume ownership:
```bash
docker rm -f berkut-scc || true
docker run --rm --user 0 --entrypoint sh -v berkut-data:/app/data berkut-scc -c "chown -R berkut:berkut /app/data"
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 4. Default secrets error outside dev
Symptom: `default secrets are not allowed outside APP_ENV=dev`.

Fix:
- Dev mode: set `ENV=dev` and `APP_ENV=dev`.
- Prod mode: replace all default secrets.

## 5. Full recovery (recreate data)
WARNING: this removes DB data.
```bash
docker rm -f berkut-scc || true
docker volume rm berkut-data
docker run -d --name berkut-scc -p 8080:8080 -v berkut-data:/app/data --env-file .env berkut-scc
```

## 6. Logs
```bash
docker logs -f berkut-scc
```
