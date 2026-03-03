# Changelog

## 1.0.16 — 03.03.2026

### Ядро / Monitoring
- Надёжность мониторинга: добавлен статус `ISSUE` для transient сетевых ошибок (timeout/TLS/connect/unexpected EOF и т.п.), чтобы они не давали ложный `DOWN`.
- Эскалация `ISSUE → DOWN`: если нет успешного `UP` дольше порога, монитор считается `DOWN` и уходят обычные уведомления/автоматизации. Порог настраивается в Monitoring Settings (`issue_escalate_minutes`), значение по умолчанию: 10 минут.
