# Журнал изменений

## 1.0.4-1 — 15.02.2026

### Документация / деплой
- Обновлён пример compose: `docs/ru/docker-compose.https.yml` под запуск через отдельные контейнеры `nginx` + `onlyoffice`.
- Из compose-примера убрана привязка к пользовательской сети и статическим IP.
- В `nginx`-примере добавлен inline reverse-proxy для маршрутов `/`, `/office/`, `/cache/`, `/printfile/`, `/downloadas/`.
- Секреты в compose-примере переведены на безопасные placeholders (`change-me-*`) без чувствительных значений.
- Для `OnlyOffice` публичный URL в примере закреплён как `/office/` для same-origin работы через proxy.
- В `docs/ru/https_onlyoffice.md` добавлен раздел быстрого запуска через отдельный proxy-контейнер.
- Обновлены формулировки ссылок на compose-пример в:
  - `docs/ru/README.md`
  - `docs/README.md`
  - `docs/eng/README.md`

### Документация / презентация проекта
- Переработан `README.md`: расширено описание продукта, целевой аудитории, бизнес-ценности и ключевых возможностей.
- Переработан `README.en.md` в аналогичной структуре для англоязычной презентации проекта.
- Инструкции быстрого запуска вынесены в отдельные файлы:
  - `QUICKSTART.md`
  - `QUICKSTART.en.md`
- В корневых README добавлены прямые гиперссылки на документы по запуску и деплою:
  - `docs/ru/deploy.md`, `docs/eng/deploy.md`
  - `docs/ru/runbook.md`, `docs/eng/runbook.md`
  - `docs/ru/https_onlyoffice.md`, `docs/eng/https_onlyoffice.md`
  - `docs/ru/docker-compose.https.yml`

## 1.0.3 — 13.02.2026
- Базовый релиз платформы с модулями backup/SLA/monitoring и обновлённой серверной архитектурой.


