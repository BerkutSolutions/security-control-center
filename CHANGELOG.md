# Журнал изменений

## 1.0.3 — 13.02.2026

### Архитектура и платформа
- Сформирован полноценный модуль бэкапов `.bscc` с разделением слоёв: `router -> handler -> service -> store`.
- Добавлены и применяются миграции PostgreSQL для бэкапов, restore-run, планов и scheduler-полей.
- Приложение переведено на единый источник версии: `core/appmeta.AppVersion = "1.0.3"` с поддержкой переопределения через `-ldflags`.

### Бэкапы (.bscc)
- Полный цикл `.bscc`: create/import/download/delete.
- Restore engine: dry-run и реальное восстановление с пошаговым прогрессом.
- Backup plan + scheduler + retention policy, включая пресеты расписаний и выборочный ручной запуск (`label/scope/include_files`).
- Проверка совместимости manifest и пост-проверка схемы после restore.

### Безопасность
- Zero-trust проверки прав сохранены на всех endpoint; права backup-модуля нормализованы.
- Усилен hardening upload/download/delete (лимиты, path validation, защита от traversal).
- Добавлены конкурентные блокировки create/import/restore/delete и корректные `409`.
- Maintenance mode строго активируется на время restore; ошибки API приведены к безопасному формату.

### Визуал и UX
- Добавлен раздел «Бэкапы» в меню с вкладками: `Обзор`, `История`, `Восстановление`, `Расписание`.
- Вкладки бэкапов подключены как отдельные маршруты (`/backups/*`) с поддержкой прямых ссылок.
- Восстановление и история получили рабочие действия и прогресс в UI.

### Мониторинг SLA
- В «Мониторинг» добавлена вкладка `SLA` с карточками по мониторам, статусами `24h/7d/30d` и настройками политики инцидентов.
- Добавлена история закрытых периодов (`day/week/month`) и coverage-aware модель (`ok/violated/unknown`).
- Запущен background SLA evaluator (закрытие периодов, идемпотентные результаты, без дублей).
- SLA-инциденты создаются только на закрытии выбранного периода и только при включенной policy.
- Добавлены endpoint’ы:
  - `GET /api/monitoring/sla/overview`
  - `GET /api/monitoring/sla/history`
  - `PUT /api/monitoring/monitors/{id}/sla-policy`

### Локализация (RU/EN)
- Обновлены и нормализованы i18n-ключи для backups/monitoring/SLA.
- Синхронизированы подписи действий в журнале аудита.

### Надёжность и исправления
- Исправлены ключевые сценарии restore: зависшие run-state, стабильность polling/status, cleanup fail-сценариев, корректное завершение maintenance mode.
- `pg_restore` переведён в строгий режим (`--clean --if-exists --exit-on-error`).
- Для мониторинга доработаны стабильность статусов/уведомлений и визуал графиков (time-axis, tooltip, down-zones, агрегация длинных диапазонов).
- Центр событий вынесен в отдельную вкладку `/monitoring/events`.
- Разрешение приватных сетей в мониторинге включено по умолчанию.

### Тесты и качество
- Добавлены unit/интеграционные проверки для backup-модуля, прав доступа и конкурентных сценариев.
- Критические сценарии 1.0.3 покрыты `make ci`.

### Документация
- Обновлены `README` и RU/EN docs под 1.0.3.
- Синхронизированы API/wiki: новые вкладки мониторинга, SLA-endpoint’ы, behavior закрытых периодов.

### Техобслуживание (мониторинг)
- Добавлена отдельная вкладка `/monitoring/maintenance` рядом с SLA.
- Реализованы стратегии окон обслуживания: `single`, `cron`, `interval`, `weekday`, `monthday`.
- Добавлены операции полного цикла: создание, редактирование, остановка, возобновление и удаление окон.
- Окна техобслуживания учитываются как accepted risk и исключаются из SLA-штрафов.
- UI окна техобслуживания обновлен: улучшена сетка модального окна, унифицированы чекбоксы, поля даты/времени и селекты.

## 1.0.5 - 14.02.2026

### Docs / OnlyOffice (foundation)
- Added containerized `OnlyOffice Document Server` service to `docker-compose.yaml`.
- Added nginx reverse-proxy route `location /office/` in `docker/nginx/default.conf`.
- Added docs-onlyoffice runtime config in `config/app.yaml` with JWT and endpoint settings.
- Added corresponding env vars in `.env.example` for `BERKUT_DOCS_ONLYOFFICE_*` and `ONLYOFFICE_JWT_*`.
- Added config normalization defaults for OnlyOffice in `config/manager.go`.
- Added config validation in `config/validate.go`: when enabled, `public_url` and `jwt_secret` are required.
- Added validation test coverage in `config/validate_test.go`.
- Added backend endpoints for OnlyOffice workflow:
  - `GET /api/docs/{id}/office/config` (session + `docs.edit` permission)
  - `GET /api/docs/{id}/office/file` (signed one-time style token)
  - `POST /api/docs/{id}/office/callback` (OnlyOffice JWT + signed callback token)
- Added secure callback processing:
  - verifies JWT signature from OnlyOffice server;
  - validates signed callback token (`doc/version/purpose/exp`);
  - saves returned DOCX as a new encrypted document version.
- Added audit events for OnlyOffice actions:
  - `doc.onlyoffice.config`
  - `doc.onlyoffice.callback_saved`
  - `doc.onlyoffice.callback_error`
- Added RU/EN localization keys for new OnlyOffice statuses/errors.
- Added Docs UI integration for DOCX:
  - local OnlyOffice API loader from `/office/web-apps/apps/api/documents/api.js` (no external CDN);
  - embedded `DocEditor` mount in `doc-editor` panel;
  - fallback to existing DOCX read-only viewer when OnlyOffice is unavailable.
- Added environment template and local `.env` fields for OnlyOffice app/internal URLs and JWT settings.
- Fixed OnlyOffice script loading for same-host deployments with different scheme/port by normalizing to same-origin `/office/...` path in UI loader.
- Extended RU/EN i18n for OnlyOffice:
  - UI keys (`docs.onlyoffice.*`)
  - audit action labels (`doc.onlyoffice.*`).
- Added dedicated HTTPS + OnlyOffice documentation:
  - `docs/ru/https_onlyoffice.md`
  - `docs/eng/https_onlyoffice.md`
  - includes Windows and Linux trust/certificate setup flows for localhost.
- Added standalone HTTPS compose example (without custom macvlan):
  - `docs/ru/docker-compose.https.yml`
- Fixed OnlyOffice binary chunk loading behind `/office/` by proxying `location /cache/` to Document Server in `docker/nginx/default.conf`.
- Updated docs editor UX for non-Markdown formats:
  - DOCX now opens in dedicated embedded OnlyOffice viewer/editor (no markdown fallback pane).
  - PDF/DOCX no longer expose markdown save/toolbar actions in the editor panel.
  - editor reason input is kept visible in view mode.
- Fixed stale unsaved text in docs editor after closing panel: editor state is now cleared on close, so reopened edit mode starts from persisted version.
- Hardened OnlyOffice security and save flow:
  - disabled DOCX print/download permissions in editor config (`document.permissions.print/download=false`);
  - added backend endpoint `POST /api/docs/{id}/office/forcesave` (requires `docs.edit`, reason required);
  - wired DOCX save button to explicit force-save request with audit event `doc.onlyoffice.forcesave`;
  - callback now prefers `userdata` reason from force-save request and skips duplicate version creation when content is unchanged.
- Improved proxy compatibility for embedded OnlyOffice runtime:
  - added reverse-proxy routes for `/printfile/` and `/downloadas/` to Document Server;
  - added one-shot retry when opening embedded DOCX editor to avoid transient startup failures.
- Updated editor UX:
  - reason field is shown only in edit mode;
  - removed fallback DOCX/PDF action buttons from markdown-oriented panel (`open/download/convert`);
  - increased embedded editor panel height to use available viewport space.
- Fixed docs tab CSP warning by removing inline `style.*` mutations in docs editor runtime (`gui/static/js/editor.js`), relying on class/hidden state instead.
- Fixed OnlyOffice explicit save authorization for command service by sending JWT with `payload` envelope (`api/handlers/docs_onlyoffice.go`).
- Improved OnlyOffice config endpoint security/UX:
  - `GET /api/docs/{id}/office/config` now accepts `mode=view|edit` and performs ACL check per requested mode;
  - route guard switched to `docs.view` with server-side ACL enforcement for zero-trust behavior.
- Improved DOCX edit transition from view mode:
  - added loader stub ("Загрузка редактора...") and two-step switch (close viewer -> delayed editor open) to reduce transient init failures.
- Updated OnlyOffice force-save command format to token-in-body mode for JWT-enabled CommandService and included `iss/aud` claims when configured.
- Fixed DOCX mode switching reliability in both directions (view <-> edit): now always uses loader + delayed re-init to avoid transient "misconfigured/unavailable" errors after previous session state.
- Fixed force-save key mismatch on long editor sessions: frontend now sends active OnlyOffice document key, backend validates doc key prefix and uses that key for `forcesave`.
- Improved CommandService compatibility: force-save request now sends signed token both in body and header together with command fields.
- Improved OnlyOffice readiness handling:
  - docs editor waits for `onAppReady/onDocumentReady` before treating embedded editor as ready;
  - DOCX save is blocked until editor ready and protected against duplicate concurrent clicks.
- Fixed force-save transport retries by recreating command requests per attempt (fresh body per retry).
- Enabled OnlyOffice autosync transport (`autosave=true`) while keeping version persistence restricted to explicit `forcesave` callback status (`status=6`).
- Fixed docs RU i18n regressions for OnlyOffice loading/error messages (`docs.onlyoffice.loading`, `docs.onlyoffice.forceSaveFailed`, etc.).
- Added OnlyOffice refresh handling for version invalidation: switched to `onRequestRefreshFile` + `refreshFile` flow and removed deprecated `onOutdatedVersion` event usage in embedded config.
- Stabilized force-save on active editing sessions:
  - increased transient `error=4` retry window for CommandService;
  - keep automatic editor sync enabled but persist backend versions only for explicit UI-triggered force-save callbacks (`status=6` with non-empty `userdata`).
- Fixed OnlyOffice callback resilience:
  - ignore stale/mismatched callback keys with `error=0` (instead of failing editor flow);
  - ignore empty-url callback payloads with `error=0` to avoid breaking active sessions.
- Namespaced explicit save reason payload with `berkut:` prefix and persist new DOCX versions only for prefixed explicit saves.
- Restored full RU localization text for all `docs/doc.onlyoffice.*` keys in `gui/static/i18n/ru.json`.
- Docs UI polish:
  - fixed sidebar action button alignment: `Шаблоны` now aligned with `Импорт`;
  - fixed long document titles in docs list table by enabling wrapping and fixed table layout to prevent overflow beyond viewport.
- Reduced audit noise for docs editor internal polling:
  - `GET /api/docs/{id}/content` now supports `?audit=0` to skip `doc.view` audit spam for internal refresh/version checks;
  - docs editor switched internal content/version polling calls to `?audit=0`.
- Fixed OnlyOffice save callback download path in containerized setup:
  - callback result URL is now rewritten from public `localhost` host to internal OnlyOffice service URL before backend download, preventing `download_failed` on explicit save.
- Restored RU translations for all OnlyOffice save/status/error keys, including `docs.onlyoffice.forceSaveNoVersion`.
- Hardened OnlyOffice editor config to prevent implicit in-editor saves:
  - keep `editorConfig.customization.autosave=true` for transport sync required by modal force-save flow
  - set `editorConfig.customization.forcesave=false`
  - SCC modal save remains the only supported save path for DOCX version persistence.
- Disabled OnlyOffice runtime autosave in editor config (`autosave=false`) to prevent implicit save behavior.
- Removed autosave-like UI feedback path: no longer treat document state-change event as successful save; success message now comes only from explicit `Сохранить`.
- Fixed modal `Сохранить` flow for DOCX/OnlyOffice:
  - removed `autosave` customization from OnlyOffice editor config entirely;
  - explicit save now reports success only after backend detects a new document version;
  - if no new version appears after explicit save request, UI shows a dedicated error and does not claim success.

- Fixed DOCX view->edit race in embedded OnlyOffice:
  - mode switch is now queued while the current editor re-init is in flight;
  - after first open completes, pending mode is applied with full editor re-open, preventing stale readonly session config and version-mismatch failures on save.

- Hardened OnlyOffice explicit-save policy: callback now persists DOCX changes only when it carries a one-time SCC save token created by modal ""���������""; in-editor save/Ctrl+S callbacks without this token are ignored.

- OnlyOffice hardening + mode switch fix:
  - modal ""�������������"" now reopens DOC tab via the same path as context-menu Edit (clean editor boot, no gray stale session);
  - callback persistence is additionally restricted to orcesavetype=0 (CommandService), so in-editor Save/Ctrl+S is ignored for versioning.

- DOCX modal mode switch hard-reset:
  - ""�������������"" / ""��������"" in doc modal now prefers full tab reopen flow for DOCX (teardown + fresh init), matching context-menu behavior and avoiding gray stale OnlyOffice state;
  - direct DocsPage.openEditor/viewer edit path now opens DOCX in edit mode when OnlyOffice is available.

- OnlyOffice session reset fix for DOCX modal transitions:
  - GET /api/docs/{id}/office/config now issues a unique per-open document key (doc-{id}-v{version}-s*) so View/Edit toggles always start a fresh OnlyOffice session;
  - callback key validation switched from exact match to version-prefix match, preserving security while allowing per-session keys.

- Fixed OnlyOffice per-session key charset: session suffix is now hex-only ([0-9a-f]) to satisfy Document Server key pattern and prevent websocket/session rejection during View<->Edit switches.
