# Changelog

## 1.0.15 — 02.03.2026

### Ядро / Monitoring
- Incident scoring: ответы HTTP 4xx/5xx не повышают score, если монитор считается UP (например, когда 404 является ожидаемым “здоровым” статусом).
- Monitoring reliability: retryable сетевые ошибки (timeout/TLS/connect/unexpected EOF и т.п.) не переводят монитор в DOWN и не триггерят DOWN-автоматизации; они отображаются как оранжевый статус `ISSUE`. DOWN остаётся для подтверждённых “логических”/HTTP-ошибок (например, неподходящий HTTP статус/keyword/json и т.д.).
