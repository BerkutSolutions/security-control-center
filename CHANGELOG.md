# Changelog

## 1.0.14 — 02.03.2026

### Инфраструктура / Deploy
- Docker Compose (`docker-compose.yaml`): профиль `proxy` включает сервис `certgen` для автогенерации self-signed TLS сертификатов в `TLS_CERTS_PATH` (если `fullchain.pem`/`privkey.pem` отсутствуют).
- Nginx (`docker/nginx/default.conf`): обновлена конфигурация HTTP/2 (`http2 on;`) чтобы убрать предупреждение `listen ... http2` (nginx 1.27+).

### Документация
- Обновлены инструкции по HTTPS/OnlyOffice и self-signed сертификатам (описан `certgen`, `TLS_CERTS_PATH`, актуальные URL `https://localhost`).
- Обновлён Quick Start (proxy-режим теперь открывается по HTTPS).
- Добавлен документ с полным описанием математической модели incident scoring (HMM3), численного метода и вычислительного эксперимента: `docs/ru/monitoring_incident_scoring_model.md` (и EN-версия).

### Конфигурация
- `.env.example`: добавлены опциональные переменные `CERT_*` для настройки self-signed сертификатов.

### База данных
- Добавлены поля incident scoring в `monitor_state` и настройки в `monitoring_settings`.
- Incident scoring: добавлены поля HMM3 (posterior/state/observation) в `monitor_state` и настройка `incident_scoring_model` в `monitoring_settings`.

### Примеры
- `docs/ru/docker-compose.https.yml`: добавлены `certgen`, HTTPS (443) и редирект 80→443.

### Ядро / Monitoring
- Добавлена модель скоринга инцидентов (без интеграции).
- Инцидент-скоринг рассчитывается в движке мониторинга и хранится в `monitor_state`.
- Добавлена 3-состоянийная модель HMM (Normal/Degraded/Outage) как вариант incident scoring (с хранением posterior/state в `monitor_state`).

### Вкладка Monitoring
- Добавлен адаптивный режим автосоздания инцидентов по score (с подтверждением и hysteresis).

### Безопасность / Audit
- Расширены audit details для auto incident.

### API
- Расширен API настроек мониторинга: incident scoring.
- Incident scoring: добавлен выбор модели (`incident_scoring_model`).

### Безопасность
- Audit: логирование изменения настроек incident scoring.

### UI
- Добавлены настройки и визуализация incident scoring в Monitoring.
- Monitoring Settings: добавлен выбор модели скоринга (heuristic vs HMM3) и отображение HMM-состояния/распределения в деталях монитора.

### Локализация
- Новые RU/EN строки для incident scoring.
- Новые RU/EN строки для выбора модели и HMM-состояний.

### CLI / Tools
- Добавлен экспериментальный прогон политик incident scoring (replay/simulate).
- Эксперимент: добавлена политика `score_hmm3` (HMM3) для сравнения с baseline и scoring v1.
- Эксперимент: добавлен `experiment-fit` (grid search) для подбора параметров политики по функции потерь.

### Ядро / Monitoring
- Добавлен модуль вычислительного эксперимента для сравнения политик.
