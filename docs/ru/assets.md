# Активы (MVP-спека)

Модуль «Активы» — реестр систем/сервисов и хостов, которые используются в других доменах (инциденты, мониторинг, отчёты, документы/связи, findings/controls).

## Термины

- **Актив (Asset)** — сущность, описывающая *хост* или *систему/сервис/приложение*.
- Цель MVP: дать единый справочник + стабильные идентификаторы для связей, поиска и отчётности (без внешних синхронизаций на первом этапе).

## Модель данных (поля)

Обязательные поля:
- `name` — название (уникальность обеспечивается на уровне UX/поиска; строгий уникальный индекс добавляется только при явном требовании).
- `type` — тип актива: `host | service | application | network | other`.
- `criticality` — критичность: `low | medium | high | critical`.

Рекомендуемые поля MVP:
- `description` — описание/назначение.
- `commissioned_at` — дата ввода в эксплуатацию (date).
- `ip_addresses` — список IP (IPv4/IPv6), храним как массив; в UI ввод через разделители/строки.
- `owner` — владелец (на MVP: текст; позже возможно связывание с пользователем/группой).
- `administrator` — администратор (на MVP: текст; позже возможно связывание).
- `env` — окружение: `prod | stage | dev | test | other`.
- `status` — статус: `active | decommissioned`.
- `tags` — теги (массив строк, нормализация trim/lower опциональна; отображение как чипсы/список).

Служебные поля (как и в других доменах):
- `id` (int64), `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `deleted_at` (soft-delete/archive).

## RBAC и zero-trust

Новые permissions (deny-by-default):
- `assets.view` — просмотр списка/карточки/поиска.
- `assets.manage` — создание/редактирование/архивирование/восстановление.

Каждый endpoint `/api/assets*` обязан проверять permissions на сервере. UI/меню не является контролем безопасности.

## Вкладка/доступ через меню (menu_permissions)

- Новая вкладка: `assets`
- UI route: `/registry/assets` (вкладка «Реестры»). Legacy прямой маршрут: `/assets`
- API route group: `/api/assets`

Доступ к вкладке дополнительно ограничивается `menu_permissions` (как и для остальных вкладок).

## Аудит (audit_log)

Ключевые события для логирования:
- `assets.create`, `assets.update`, `assets.archive`, `assets.restore`.
- `assets.export.csv`.

## Export / autocomplete (stage 4)

- `GET /api/assets/export.csv` — CSV export (filters as in list; `limit` up to 5000).
- `GET /api/assets/autocomplete` — field suggestions (`field=all|owners|administrators|tags`, `q`, `limit`; `include_deleted=1` requires `assets.manage`).

Аудит обязателен даже при наличии client-side UX-контролей (сервер — источник истины).

## Интеграции (требования)

Цель: использовать `asset_id` в связях/фильтрах вместо свободного текста.

План интеграций по доменам (в следующих этапах):
- Incidents: привязка инцидента к активам (многие-ко-многим) + миграция/сосуществование с `meta.assets` (текст).
- Monitoring: привязка монитора к активам (многие-ко-многим) + фильтры.
- Findings/Controls/Tasks/Docs/Reports: добавление актива как связанной сущности (link/relationship) + поиск/переходы.

## Валидации (MVP)

- `name`: обязательное, 1..200, trim.
- `ip_addresses`: каждый элемент — валидный IPv4/IPv6 (пустые/дубликаты удаляются).
- `tags`: 0..N, каждый тег 1..50, trim (дубликаты удаляются).
- `commissioned_at`: корректная дата, допускается пустое.
