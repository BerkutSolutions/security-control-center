# Деплой и CI/CD

## Docker
- Образ: `docker/Dockerfile`
- Compose: `docker-compose.yaml`
- Данные: volume на `${DATA_PATH}`

## Переменные окружения
- Основные: `ENV`, `APP_ENV`, `APP_CONFIG`, `PORT`, `DATA_PATH`
- Секреты: `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`
- Overrides: `BERKUT_*`
- HTTPS defaults: `HTTPS_*`

## GitLab CI/CD
`.gitlab-ci.yml`:
1. Build image
2. Push image в registry
3. Deploy по SSH: `docker compose pull` + `docker compose up -d --wait`

## Rollback
- Поставить прошлый `IMAGE_TAG`
- Запустить `docker compose up -d --no-deps berkut`
