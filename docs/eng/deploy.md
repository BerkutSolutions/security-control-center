# Deploy and CI/CD

## Docker
- Image: `docker/Dockerfile`
- Compose stack: `docker-compose.yaml`
- Persistent data: volume mounted to `${DATA_PATH}`

## Environment variables
- Core: `ENV`, `APP_ENV`, `APP_CONFIG`, `PORT`, `DATA_PATH`
- Secrets: `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`
- Runtime overrides: `BERKUT_*`
- HTTPS defaults: `HTTPS_*`

## GitLab CI/CD
`.gitlab-ci.yml`:
1. Build image
2. Push image to registry
3. SSH deploy: `docker compose pull` + `docker compose up -d --wait`

## Rollback
- Set previous `IMAGE_TAG`
- Run `docker compose up -d --no-deps berkut`
