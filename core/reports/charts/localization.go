package charts

var ru = map[string]string{
	"chart.title.incidents_severity":  "Инциденты по критичности",
	"chart.title.incidents_status":    "Инциденты по статусам",
	"chart.title.incidents_weekly":    "Инциденты по неделям",
	"chart.title.tasks_status":        "Выполнение задач",
	"chart.title.tasks_weekly":        "Динамика выполнения по неделям",
	"chart.title.docs_approvals":      "Согласования документов",
	"chart.title.docs_weekly":         "Новые документы по неделям",
	"chart.title.controls_status":     "Контроли по статусу",
	"chart.title.controls_domains":    "Нарушения по доменам",
	"chart.title.monitoring_uptime":   "Uptime критичных мониторингов",
	"chart.title.monitoring_downtime": "Падения по дням",
	"chart.title.monitoring_tls":      "TLS истекает",
	"chart.axis.count":                "Количество",
	"chart.axis.week":                 "Неделя",
	"chart.axis.day":                  "День",
	"chart.axis.uptime":               "Uptime (%)",
	"chart.axis.domain":               "Домен",
	"chart.axis.monitor":              "Монитор",
	"chart.label.done":                "Выполнено",
	"chart.label.overdue":             "Просрочено",
	"chart.label.in_progress":         "В работе",
	"chart.label.approved":            "Одобрено",
	"chart.label.returned":            "Возвращено",
	"chart.label.review":              "На рассмотрении",
	"chart.label.ok":                  "OK",
	"chart.label.failed":              "Нарушено",
	"chart.label.unknown":             "Неизвестно",
	"chart.label.lt7":                 "< 7 дней",
	"chart.label.lt30":                "< 30 дней",
	"chart.label.lt90":                "< 90 дней",
	"chart.severity.critical":         "Критично",
	"chart.severity.high":             "Высокая",
	"chart.severity.medium":           "Средняя",
	"chart.severity.low":              "Низкая",
	"chart.status.open":               "Открыт",
	"chart.status.in_progress":        "В работе",
	"chart.status.resolved":           "Решен",
	"chart.status.closed":             "Закрыт",
	"chart.status.draft":              "Черновик",
}

var en = map[string]string{
	"chart.title.incidents_severity":  "Incidents by severity",
	"chart.title.incidents_status":    "Incidents by status",
	"chart.title.incidents_weekly":    "Incidents by week",
	"chart.title.tasks_status":        "Task completion",
	"chart.title.tasks_weekly":        "Task completion by week",
	"chart.title.docs_approvals":      "Document approvals",
	"chart.title.docs_weekly":         "New documents by week",
	"chart.title.controls_status":     "Controls by status",
	"chart.title.controls_domains":    "Violations by domain",
	"chart.title.monitoring_uptime":   "Uptime for critical monitors",
	"chart.title.monitoring_downtime": "Downtime by day",
	"chart.title.monitoring_tls":      "TLS expiring",
	"chart.axis.count":                "Count",
	"chart.axis.week":                 "Week",
	"chart.axis.day":                  "Day",
	"chart.axis.uptime":               "Uptime (%)",
	"chart.axis.domain":               "Domain",
	"chart.axis.monitor":              "Monitor",
	"chart.label.done":                "Done",
	"chart.label.overdue":             "Overdue",
	"chart.label.in_progress":         "In progress",
	"chart.label.approved":            "Approved",
	"chart.label.returned":            "Returned",
	"chart.label.review":              "In review",
	"chart.label.ok":                  "OK",
	"chart.label.failed":              "FAILED",
	"chart.label.unknown":             "UNKNOWN",
	"chart.label.lt7":                 "< 7 days",
	"chart.label.lt30":                "< 30 days",
	"chart.label.lt90":                "< 90 days",
	"chart.severity.critical":         "Critical",
	"chart.severity.high":             "High",
	"chart.severity.medium":           "Medium",
	"chart.severity.low":              "Low",
	"chart.status.open":               "Open",
	"chart.status.in_progress":        "In progress",
	"chart.status.resolved":           "Resolved",
	"chart.status.closed":             "Closed",
	"chart.status.draft":              "Draft",
}

func Localized(lang, key string) string {
	if lang == "ru" {
		if val, ok := ru[key]; ok {
			return val
		}
	}
	if val, ok := en[key]; ok {
		return val
	}
	return key
}
