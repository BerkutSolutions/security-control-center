# Wiki: UI tabs

## Dashboard
Operational overview and saved layout.

## Tasks
Spaces/boards/tasks, parent-child links, comments/files, archive, recurring.
Routes: `/tasks`, `/tasks/space/{id}`, `/tasks/space/{space}/task/{task}`.

## Registries
Registries umbrella: controls, checks/violations, frameworks, plus Assets/Software/Findings registries and their relations.
Tab routes:
- `/registry/overview`
- `/registry/controls`
- `/registry/checks`
- `/registry/violations`
- `/registry/frameworks`
- `/registry/assets`
- `/registry/software`
- `/registry/findings`

Legacy routes `/controls/...` and direct `/assets`, `/software`, `/findings` may still exist, but the canonical UI entry is the Registries tab (`/registry/...`).

## Monitoring
Tab routes:
- `/monitoring`
- `/monitoring/events`
- `/monitoring/sla`
- `/monitoring/maintenance`
- `/monitoring/certs`
- `/monitoring/notifications`
- `/monitoring/settings`

SLA tab summary:
- monitor SLA cards (24h/7d/30d, status, incident policy);
- closed-period history;
- period closure calculations are asynchronous via background evaluator.

Maintenance tab summary:
- dedicated tab for scheduling/editing/stopping maintenance windows;
- strategies: `single`, `cron`, `interval`, `weekday`, `monthday`;
- maintenance windows are excluded from SLA penalties as accepted risk.

## Docs
Documents, versions, ACL, classification, export, templates.

## Approvals
Approval queue, approve/reject decisions, comments.

## Incidents
Incident records, stages, attachments, timeline, links, export.

## Reports
Builder, sections, charts, snapshots, templates, export.

## Backups
Tab routes:
- `/backups`
- `/backups/history`
- `/backups/restore`
- `/backups/plan`

## Accounts
Users, roles, groups, sessions, import.

## Settings
General/Advanced/HTTPS/Tags/Incidents/Registries/About.

## Logs
Audit log viewer.
