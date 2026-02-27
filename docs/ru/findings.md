# Находки (Findings)

Модуль **«Находки»** — реестр технических/процессных/комплаенс-находок с тегами и связями с другими сущностями (активы, контроли, инциденты, задачи, документы).

## Поля (MVP)

- `title` — название (обязательно).
- `description_md` — описание (Markdown).
- `status` — `open | in_progress | resolved | accepted_risk | false_positive`.
- `severity` — `low | medium | high | critical`.
- `finding_type` — `technical | config | process | compliance | other`.
- `owner` — владелец (текст, MVP).
- `due_at` — срок (date/ISO time).
- `tags` — массив строк (нормализация: trim, upper).
- служебные: `id`, `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at`.

## RBAC и zero-trust

Новые permissions (deny-by-default):
- `findings.view` — просмотр списка/карточки/поиска.
- `findings.manage` — создание/редактирование/архив/восстановление + управление связями.

Каждый endpoint `/api/findings*` обязан проверять permissions на сервере. UI/меню не является контролем безопасности.

## Вкладка/доступ через меню (menu_permissions)

- вкладка: `findings`
- UI route: `/registry/findings` (вкладка «Реестры»). Legacy прямой маршрут: `/findings`
- API route group: `/api/findings`

Доступ к вкладке дополнительно ограничивается `menu_permissions` (как и для остальных вкладок).

## API (основное)

- `GET /api/findings` — список/поиск (фильтры: `q`, `status`, `severity`, `type`, `tag`; `include_deleted=1` только при `findings.manage`).
- `GET /api/findings/{id}` — карточка.
- `POST /api/findings` — создание.
- `PUT /api/findings/{id}` — обновление (optimistic lock через `version`).
- `DELETE /api/findings/{id}` — архивирование (soft-delete).
- `POST /api/findings/{id}/restore` — восстановление.

## API (экспорт/автодополнение)

- `GET /api/findings/export.csv` — CSV экспорт (фильтры как в list; `limit` до 5000).
- `GET /api/findings/autocomplete` — подсказки для полей (параметры: `field=all|titles|owners|tags`, `q`, `limit`, `include_deleted=1` только при `findings.manage`).

## Связи (relations)

Связи реализованы через `entity_links` (`source_type="finding"`):
- `GET /api/findings/{id}/links`
- `POST /api/findings/{id}/links`
- `DELETE /api/findings/{id}/links/{link_id}`

Для связей типа `asset`/`control` сервер дополнительно проверяет права (`assets.view`/`controls.view`) и доступ к вкладкам через `menu_permissions`.

## Audit (логирование)

Ключевые события:
- `finding.create`, `finding.update`, `finding.archive`, `finding.restore`
- `finding.link.add`, `finding.link.remove`
- `finding.export.csv`
