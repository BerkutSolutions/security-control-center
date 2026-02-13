# Deploy and CI/CD

## Docker stack
- App image: `docker/Dockerfile`
- Compose stack: `docker-compose.yaml`
- Services: `berkut` (app), `postgres` (DB)
- Persistent volumes: `berkut_data`, `berkut_pgdata`

## Environment variables
Core:
- `ENV`, `APP_ENV`, `APP_CONFIG`, `PORT`, `DATA_PATH`

Secrets:
- `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`

DB/runtime:
- `BERKUT_DB_DRIVER=postgres`
- `BERKUT_DB_URL=postgres://...`
- other `BERKUT_*` overrides as needed

## Typical deploy
```bash
cp .env.example .env
docker compose pull
docker compose up -d --build
```

Note:
- Container startup runs `berkut-migrate` first, then starts `berkut-scc`.
- For non-Docker runs apply migrations explicitly before app start:
```bash
make migrate
go run ./main.go
```

## Rollback
1. Pin previous `IMAGE_TAG`.
2. Restart app service only:
```bash
docker compose up -d --no-deps berkut
```

## CI
`.gitlab-ci.yml` includes a verification stage (`verify-go`) before image build/publish.
