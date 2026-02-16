# Wiki: вкладки UI

## Dashboard
Оперативная панель с ключевыми метриками и layout.

## Tasks
Spaces/boards/tasks, parent-child связи, комментарии, файлы, архив, recurring.
Маршруты: `/tasks`, `/tasks/space/{id}`, `/tasks/space/{space}/task/{task}`.

## Controls
Реестр контролей, checks/violations, комментарии и связи.

## Monitoring
Маршруты вкладок:
- `/monitoring`
- `/monitoring/events`
- `/monitoring/sla`
- `/monitoring/maintenance`
- `/monitoring/certs`
- `/monitoring/notifications`
- `/monitoring/settings`

Кратко по SLA:
- карточки SLA по мониторам (24h/7d/30d, статус, policy инцидентов);
- история закрытых периодов;
- расчет закрытых периодов выполняется асинхронно background evaluator.

Кратко по техобслуживанию:
- отдельная вкладка с планированием/редактированием/остановкой окон;
- стратегии расписаний: `single`, `cron`, `interval`, `weekday`, `monthday`;
- окна техобслуживания исключаются из SLA-штрафов как accepted risk.

## Docs
Документы, версии, ACL, классификация, экспорт, шаблоны.

## Approvals
Очередь согласований, решения approve/reject, комментарии.

## Incidents
Карточки инцидентов, этапы, вложения, timeline, связи, export.

## Reports
Builder, sections, charts, snapshots, templates, export.

## Backups
Маршруты вкладок:
- `/backups`
- `/backups/history`
- `/backups/restore`
- `/backups/plan`

## Accounts
Пользователи, роли, группы, сессии, импорт.

## Settings
General/Advanced/HTTPS/Tags/Incidents/Controls/About.

## Logs
Просмотр аудита операций.
