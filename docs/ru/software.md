# Реестр ПО (MVP)

Модуль "ПО" - это реестр программных продуктов и версий, а также связь "Актив -> ПО (установки)".

Цели:
- единые идентификаторы для связей типа `software` во всех доменах SCC;
- инвентаризация ПО на активах и удобная отчётность;
- deny-by-default RBAC и серверная проверка прав на каждом endpoint.

## Модель данных (MVP)

### Продукт

Обязательные поля:
- `name` - название продукта.

Рекомендуемые:
- `vendor` - вендор/издатель (текст).
- `product_type` - `os | middleware | database | application | library | agent | other`.
- `description` - заметки.
- `tags` - массив строк.

Системные поля:
- `id` (int64), `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at` (архивирование через soft-delete).

### Версия

Обязательные поля:
- `version` - строка версии (trim, 1..100).

Рекомендуемые:
- `release_date` - дата релиза (опционально).
- `eol_date` - дата окончания поддержки (опционально).
- `notes` - комментарий.

Системные поля:
- `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

### Установка (asset -> software)

Обязательные поля:
- `asset_id`
- `product_id`

Рекомендуемые:
- `version_id` - ссылка на версию из реестра (опционально, если используем `version_text`).
- `version_text` - строка версии, если нужной версии нет в реестре (опционально).
- `installed_at` - дата установки (опционально).
- `source` - источник инвентаризации (текст, опционально).
- `notes` - примечание.

Системные поля:
- `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

## RBAC и zero-trust

Новые permissions (deny-by-default):
- `software.view` - просмотр продуктов/версий/установок.
- `software.manage` - создание/редактирование/архив/восстановление + управление версиями и установками.

Каждый endpoint `/api/software*` обязан проверять права на сервере. UI/меню не является контролом безопасности.

## Вкладка/доступ через меню (menu_permissions)

- tab key: `software`
- UI маршрут: `/registry/software` (вкладка «Реестры»). Legacy прямой маршрут: `/software`
- API группа: `/api/software`

Доступ к вкладке дополнительно ограничивается `menu_permissions`.

Установки ПО находятся в Assets API:
- `/api/assets/{id}/software*`

Для установок сервер проверяет сразу оба домена:
- права по активам (`assets.view` / `assets.manage`) + доступ к вкладке `assets` через `menu_permissions`
- права по ПО (`software.view` / `software.manage`) + доступ к вкладке `software` через `menu_permissions`

Это предотвращает обход ограничений вкладки "ПО" через endpoints активов.

## API (ядро)

Продукты:
- `GET /api/software` - список/поиск (фильтры: `q`, `vendor`, `type`, `tag`, `include_deleted=1` требует `software.manage`).
- `GET /api/software/{id}` - карточка.
- `POST /api/software` - создание.
- `PUT /api/software/{id}` - обновление (optimistic lock через `version`).
- `DELETE /api/software/{id}` - архив.
- `POST /api/software/{id}/restore` - восстановление.

Версии:
- `GET /api/software/{id}/versions` - список версий.
- `POST /api/software/{id}/versions` - создание версии.
- `PUT /api/software/{id}/versions/{version_id}` - обновление версии.
- `DELETE /api/software/{id}/versions/{version_id}` - архив версии.
- `POST /api/software/{id}/versions/{version_id}/restore` - восстановление версии.

Установки:
- `GET /api/assets/{id}/software` - список установок на активе.
- `POST /api/assets/{id}/software` - добавить установку.
- `PUT /api/assets/{id}/software/{install_id}` - обновить установку.
- `DELETE /api/assets/{id}/software/{install_id}` - архивировать установку.
- `POST /api/assets/{id}/software/{install_id}/restore` - восстановить установку.

## Экспорт и автодополнение

- `GET /api/software/export.csv` - экспорт продуктов в CSV (фильтры как в list; `limit` до 5000).
- `GET /api/software/autocomplete` - подсказки (параметры: `field=all|names|vendors|tags`, `q`, `limit`, `include_deleted=1` требует `software.manage`).

## Связи/интеграции

Система связей (`entity_links`) поддерживает тип цели `software`.

При создании связей на ПО сервер дополнительно проверяет:
- права (`software.view`);
- доступность вкладки через `menu_permissions`.

## Audit

Ключевые действия:
- `software.create`, `software.update`, `software.archive`, `software.restore`
- `software.version.create`, `software.version.update`, `software.version.archive`, `software.version.restore`
- `software.export.csv`
- `assets.software.add`, `assets.software.update`, `assets.software.archive`, `assets.software.restore`
