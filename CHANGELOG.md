# Журнал изменений

## 1.0.2 — 12.02.2026

### Визуал и UX
- Интерфейс документов переведен на встроенные вкладки: просмотр и редактирование выполняются внутри tab-shell без legacy-модалок.
- Открытие документа без табов переведено на маршрут `/docs/{id}`.
- В карточке документа поля метаданных отображаются как read-only текст без визуала disabled-input.
- Компоновка страницы `Мониторинг` уплотнена: уменьшены верхние фильтры и высота селекторов.
- В `Задачах` расширена рабочая зона контекстного меню (ПКМ) по всей внутренней области пространства/вкладки.
- Обновлена визуальная стилизация одиночных выпадающих списков (стрелка, фон, popup-элементы) в рамках существующего UI-стиля.
- Детальная карточка монитора улучшена по UX: кнопка `Check now` корректно блокируется для мониторов на паузе.

### Мониторинг
- Добавлены новые рабочие типы мониторов:
  - `HTTP(s)` (`http`)
  - `TCP Port` (`tcp`)
  - `Ping` (`ping`)
  - `HTTP(s) - Word` (`http_keyword`)
  - `HTTP(s) - JSON` (`http_json`)
  - `gRPC(s) - Word` (`grpc_keyword`)
  - `DNS` (`dns`)
  - `Docker container` (`docker`)
  - `Push` (`push`, пассивный монитор)
  - `Steam Game Server` (`steam`)
  - `GameDig` (`gamedig`)
  - `MQTT` (`mqtt`)
  - `Kafka Producer` (`kafka_producer`)
  - `Microsoft SQL Server` (`mssql`)
  - `Redis` (`redis`)
  - `PostgreSQL` (`postgres`)
  - `MySQL/MariaDB` (`mysql`)
  - `MongoDB` (`mongodb`)
  - `Radius` (`radius`)
  - `Tailscale Ping` (`tailscale_ping`)
- Добавлен пассивный ingestion endpoint для push-мониторов:
  - `POST /api/monitoring/monitors/{id}/push`
- Для `Check now` добавлена корректная обработка состояния `monitoring.error.busy` (конкурирующая проверка уже выполняется).
- Для ping-like мониторов добавлена нормализация host из URL (`https://host` -> `host`), чтобы исключить ошибки вида `lookup https://...: no such host`.
- В модалке мониторинга улучшено переключение типов:
  - при смене типа значения URL/host/port автоматически адаптируются;
  - уменьшено «потерянное» состояние поля URL при переходе между типами.
- Для HTTP-мониторов доработана обработка редиректов:
  - по умолчанию редиректы следуются;
  - если в `allowed_status` ожидаются `3xx`, используется ответ без follow redirect.
- Расширено покрытие TLS-метрик/сертификатов для новых HTTP-типов (`http`, `http_keyword`, `http_json`).
- Нормализован аудит мониторинга: унифицированы action-коды и добавлены события для очистки метрик/событий и обновления notification-bindings.

### Безопасность
- Усилена серверная защита загрузок: добавлен лимит multipart-тел (`http.MaxBytesReader`) для документов, инцидентов, задач, комментариев и импорта аккаунтов.
- Для превышения лимита загрузки возвращается корректный `413 Payload Too Large`.
- Усилена защита от XSS в рендеринге markdown/docx: санитизация HTML-фрагментов и URL, фильтрация небезопасных схем ссылок/изображений.
- Подтверждено zero-trust поведение по грифам доступа (`userLevel >= docLevel` + `HasClearance`).
- При старте сервера выполняется принудительная ревокация активных сессий (`system_startup`).
- Добавлено автоматическое покрытие guard-проверок endpoints.

### Архитектура
- Инициализация приложения вынесена в явный bootstrap-композиционный слой (`core/appbootstrap`): БД, миграции, seed admin, конструирование API.
- Сборка зависимостей `store/service/worker` перенесена из `api` в `core/appbootstrap/compose.go` как единая composition-root точка.
- `api.NewServer` переведен на явный DI-контракт (`ServerDeps`), скрытая инициализация внутри `api` удалена.
- RBAC bootstrap/sync логика вынесена из `api.Server` в `core/rbac` (`EnsureBuiltInAndRefresh`, `RefreshFromStore`) для более чистой границы HTTP-слоя.
- Регистрация HTTP-роутов декомпозирована по модулям (`api/routes_*`, `api/routegroups`) с явными dependency bundles.
- Lifecycle фоновых воркеров отделен от `api.Server`:
  - выделен менеджер фоновых процессов;
  - scheduler/monitoring переведены на контекстное управление и корректный shutdown через `WaitGroup`;
  - orchestration запуска/остановки воркеров перенесен в `core/appbootstrap.Runtime`, `api.Server` оставлен HTTP-слоем.
- Логгер переведен на `slog` с совместимыми адаптерными методами.
- RBAC переведен на Casbin как единый runtime-движок с валидацией каталога прав и нормализацией permission-имен.

### База данных и миграции
- Переход на `goose`: SQL-миграции являются единственным источником инициализации схемы.
- Включена жесткая политика миграций: legacy-базы вне goose не поддерживаются.
- Автопрогон миграций убран из runtime-старта приложения; миграции запускаются отдельно явной командой (`make migrate` / `go run ./cmd/migrate`).
- Docker runtime синхронизирован с новой моделью: контейнер выполняет `berkut-migrate` в entrypoint перед стартом `berkut-scc`.
- Проект переведен на PostgreSQL-only runtime (`db_driver/db_url`, compose-сервис `postgres`).
- Добавлен dialect-aware выбор миграций и baseline для PostgreSQL (`core/store/migrations_pg/00001_init.sql`).
- Исправлена baseline-миграция PostgreSQL: удален ошибочный литерал `\n` между DDL-операторами `docs_fts`.
- Критичные счетчики регистрации переведены на атомарные UPSERT/RETURNING-паттерны для конкурентной записи.

### Конфигурация
- Загрузка конфигурации переведена на `cleanenv` с единой схемой YAML + ENV.
- Добавлены `env`/`env-default` теги и слой нормализации алиасов окружения (`ENV/APP_ENV`, `PORT`, `DATA_PATH` и др.).
- Обновлены валидации конфигурации под PostgreSQL-only модель.

### API и маршрутизация
- Выполнен полный переход проекта на `chi`:
  - core API/shell роутинг переведен на `chi`;
  - routegroups переписаны на `chi.Router` + `MethodFunc/Route`;
  - извлечение path-параметров в handlers переведено на `chi` route context (`urlParam/pathParams`).
- Удалена зависимость `gorilla/mux` из runtime-кода и тестов (в тестах `SetURLVars` заменен на helper с `chi.RouteContext`).
- При этом для основной API-части сохранены zero-trust проверки прав на каждом endpoint.

### Тестирование и стабильность
- Добавлен единый локальный quality pipeline (`Makefile`: `fmt`, `vet`, `test`, `lint`, `ci`) и CI-стадия `verify-go`.
- Расширено тестовое покрытие lifecycle-поведения scheduler/monitoring.
- Исправлены множественные деградации test harness:
  - сигнатуры интерфейсов store в тестах;
  - коллизии helper-функций;
  - несовместимости mock-реализаций `SessionStore`;
  - проблемы BOM в i18n JSON.
- Для `go test` добавлен безопасный fallback на SQLite только в тестовом окружении при отсутствии `DBURL`.

### Инфраструктура и сборка
- Docker builder обновлен до `golang:1.23-alpine` (совместимость с `go.mod` и toolchain).
- Проектная документация (`README.md`, `README.en.md`, `docs/*/architecture.md`, `docs/*/deploy.md`, `docs/*/runbook.md`, `config/app.yaml`) синхронизирована с PostgreSQL-only деплоем и явным migration flow.
- Восстановлена консистентность RU-документации: добавлен `docs/ru/roadmap.md` (на него есть ссылка из `docs/ru/README.md`).
- `docker-compose` приведен к актуальной схеме сервисов (`berkut`, `postgres`).

### Исправлено
- Исправлен поиск по markdown-содержимому в просмотрщике документов.
- Убрана зависимость от legacy-модалки при открытии документа в новом tab-интерфейсе.
- Исправлены ошибки сборки/веттинга, связанные с типизацией permission-guard адаптеров.
- Устранены проблемы старта в контейнере, вызванные ошибками baseline-миграции PostgreSQL.
