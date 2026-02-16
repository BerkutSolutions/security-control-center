package handlers

import (
	"net/http"
	"strings"

	"berkut-scc/core/store"
)

func (h *LogsHandler) filteredLogs(r *http.Request, filter logFilter) ([]store.AuditRecord, error) {
	raw, err := h.audits.ListFiltered(r.Context(), filter.Since, filter.Limit*3)
	if err != nil {
		return nil, err
	}
	out := make([]store.AuditRecord, 0, min(filter.Limit, len(raw)))
	for i := range raw {
		item := raw[i]
		if filter.To != nil && item.CreatedAt.After(*filter.To) {
			continue
		}
		action := strings.ToLower(strings.TrimSpace(item.Action))
		user := strings.ToLower(strings.TrimSpace(item.Username))
		details := strings.ToLower(strings.TrimSpace(item.Details))
		section := logCategory(action)
		if filter.Section != "" && section != filter.Section {
			continue
		}
		if filter.Action != "" && !strings.Contains(action, filter.Action) {
			continue
		}
		if filter.User != "" && !strings.Contains(user, filter.User) {
			continue
		}
		if filter.Query != "" && !strings.Contains(action, filter.Query) && !strings.Contains(details, filter.Query) {
			continue
		}
		out = append(out, item)
		if len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func logCategory(action string) string {
	val := strings.ToLower(strings.TrimSpace(action))
	switch {
	case strings.HasPrefix(val, "auth."), strings.HasPrefix(val, "session."):
		return "auth"
	case strings.HasPrefix(val, "doc."), strings.HasPrefix(val, "folder."), strings.HasPrefix(val, "approval."):
		return "docs"
	case strings.HasPrefix(val, "incident."), strings.HasPrefix(val, "incidents."):
		return "incidents"
	case strings.HasPrefix(val, "monitoring."):
		return "monitoring"
	case strings.HasPrefix(val, "backups."):
		return "backups"
	case strings.HasPrefix(val, "task."):
		return "tasks"
	case strings.HasPrefix(val, "report."), strings.HasPrefix(val, "reports."):
		return "reports"
	default:
		return "other"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
