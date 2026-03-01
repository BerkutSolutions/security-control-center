# Runbook (запуск и восстановление)

## 1. Старт через Docker Compose
```bash
docker compose up -d --build
```

Проверка статуса:
```bash
docker compose ps
```

## 2. Просмотр логов
```bash
docker compose logs -f berkut
docker compose logs -f postgres
```

## 3. Типовые проблемы

### 3.1 Порт 8080 занят
Симптом: `Bind for 0.0.0.0:8080 failed`.

Решение:
- освободить порт на хосте, либо
- изменить `PORT` в `.env` и перезапустить compose.

### 3.2 Ошибка миграций/старт после неудачного апдейта
Сначала попробуйте явный запуск миграций:
```bash
docker compose run --rm berkut /usr/local/bin/berkut-migrate
docker compose up -d berkut
```

Решение (с потерей данных):
```bash
docker compose down -v
docker compose up -d --build
```

### 3.3 Ошибка default secrets вне dev
Симптом: `default secrets are not allowed outside APP_ENV=dev`.

Решение:
- для dev: `ENV=dev`, `APP_ENV=dev`;
- для prod: задать реальные значения `CSRF_KEY`, `PEPPER`, `DOCS_ENCRYPTION_KEY`.

### 3.4 Потерян доступ к 2FA (TOTP)
Симптом: после ввода пароля система требует код 2FA, но приложение-аутентификатор недоступно.

Решение:
- использовать один из recovery codes (можно вводить с пробелами/дефисами);
- если recovery codes утеряны — супер-админ сбрасывает 2FA: `Accounts → Users → Reset 2FA`.

### 3.5 Passkeys (WebAuthn) не работают / окно отменено
Симптомы:
- при добавлении/входе по ключу доступа: “операция отменена/истекло время ожидания”;
- `NotAllowedError`, `SecurityError` (в devtools).

Причины/решения:
- WebAuthn требует HTTPS (или `localhost`): используйте `--profile proxy` и `https://localhost`, либо домен с валидным TLS.
- Проверьте конфигурацию `security.webauthn` (в prod рекомендуется задать `rp_id` и `origins` явно).
- При входе с 2FA ввод кода происходит на отдельной странице: `http://localhost:8080/login/2fa` (или `https://.../login/2fa` за proxy).

## 4. Полное восстановление
ВНИМАНИЕ: удалит данные PostgreSQL и файловое хранилище.
```bash
docker compose down -v
docker compose up -d --build
```

## 5. Проверка доступности
- UI: `http://localhost:8080/login`
- Liveness: `http://localhost:8080/healthz`
- Readiness (DB ping): `http://localhost:8080/readyz`
- Health статусы контейнеров: `docker compose ps`

## 6. Быстрые проверки Observability (Prometheus)
Метрики по умолчанию выключены. Чтобы включить:
- задать `BERKUT_METRICS_ENABLED=true`
- (рекомендуется) задать `BERKUT_METRICS_TOKEN` и скрейпить с `Authorization: Bearer ...`
- (только для home/dev) можно разрешить scrape без токена: `BERKUT_METRICS_ALLOW_UNAUTH_IN_HOME=true`

Пример ручной проверки:
```bash
curl -fsS -H "Authorization: Bearer $BERKUT_METRICS_TOKEN" http://localhost:8080/metrics | head
```

Полезные сигналы:
- `berkut_worker_last_tick_timestamp` — фоновые воркеры живы
- `berkut_app_jobs_oldest_queued_age_seconds` — очередь app jobs “застряла”
- `berkut_monitoring_inflight_checks` и `berkut_monitoring_error_class_total` — здоровье мониторинга
- `berkut_backup_plan_last_auto_run_age_seconds` — бэкапы давно не запускались

### Мини-алерты (пример)
1) Воркеры “умерли” (нет тиков):
```promql
time() - berkut_worker_last_tick_timestamp > 300
```

2) Очередь app jobs застыла:
```promql
berkut_app_jobs_oldest_queued_age_seconds > 600
```

3) Мониторинг стабильно ошибается (по классам ошибок, кроме `ok`):
```promql
sum(rate(berkut_monitoring_error_class_total{class!=\"ok\"}[5m])) > 0
```

4) Автобэкапы давно не запускались (если план включён):
```promql
berkut_backup_plan_enabled == 1 and berkut_backup_plan_last_auto_run_age_seconds > 86400
```
