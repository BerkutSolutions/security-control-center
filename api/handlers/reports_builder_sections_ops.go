package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildControlsSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "controls.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Controls"))
		return res
	}
	if h.controls == nil {
		res.Error = "controls unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 20)
	filter := store.ControlFilter{
		Status:    configString(sec.Config, "status"),
		RiskLevel: configString(sec.Config, "risk"),
		Domain:    configString(sec.Config, "domain"),
		Tag:       configString(sec.Config, "tag"),
	}
	items, err := h.controls.ListControls(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	if len(items) > limit && limit > 0 {
		items = items[:limit]
	}
	statusCounts := map[string]int{}
	riskCounts := map[string]int{}
	failedCount := 0
	for _, c := range items {
		status := strings.ToLower(c.Status)
		statusCounts[status]++
		riskCounts[strings.ToLower(c.RiskLevel)]++
		if status == "failed" || status == "violation" || status == "fail" {
			failedCount++
		}
	}
	res.ItemCount = len(items)
	res.Summary = map[string]any{
		"controls":        len(items),
		"controls_failed": failedCount,
	}
	totals["controls"] += len(items)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Controls")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(items)))
	for key, count := range statusCounts {
		if key == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(key), count))
	}
	if len(items) == 0 {
		b.WriteString("\n_No controls for selected filters._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Code | Title | Status | Risk | Domain |\n|---|---|---|---|---|\n")
	for _, c := range items {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			escapePipes(c.Code),
			escapePipes(c.Title),
			escapePipes(c.Status),
			escapePipes(c.RiskLevel),
			escapePipes(c.Domain),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "control",
			EntityID:   fmt.Sprintf("%d", c.ID),
			Entity: map[string]any{
				"id":         c.ID,
				"code":       c.Code,
				"title":      c.Title,
				"status":     c.Status,
				"risk_level": c.RiskLevel,
				"domain":     c.Domain,
				"created_at": c.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at": c.UpdatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildMonitoringSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "monitoring.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Monitoring"))
		return res
	}
	if h.monitoring == nil {
		res.Error = "monitoring unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 20)
	onlyDown := configBool(sec.Config, "only_down")
	onlyCritical := configBool(sec.Config, "only_critical")
	filter := store.MonitorFilter{}
	if onlyDown {
		filter.Status = "down"
	}
	monitors, err := h.monitoring.ListMonitors(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	var rows []store.MonitorSummary
	downCount := 0
	for _, m := range monitors {
		if onlyCritical && strings.ToLower(m.IncidentSeverity) != "critical" {
			continue
		}
		if strings.ToLower(m.Status) == "down" {
			downCount++
		}
		rows = append(rows, m)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}
	stateByID := map[int64]*store.MonitorState{}
	if len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, m := range rows {
			ids = append(ids, m.ID)
		}
		if states, err := h.monitoring.ListMonitorStates(ctx, ids); err == nil {
			for i := range states {
				state := states[i]
				stateByID[state.MonitorID] = &state
			}
		}
	}
	tlsExpiring := 0
	tlsDays := configInt(sec.Config, "tls_expiring_days", 0)
	if tlsDays > 0 {
		certs, _ := h.monitoring.ListCerts(ctx, store.CertFilter{ExpiringLt: tlsDays})
		tlsExpiring = len(certs)
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"monitors":      len(rows),
		"monitors_down": downCount,
		"tls_expiring":  tlsExpiring,
	}
	totals["monitors"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Monitoring")))
	b.WriteString(fmt.Sprintf("- Total monitors: %d\n", len(rows)))
	b.WriteString(fmt.Sprintf("- Down: %d\n", downCount))
	if tlsDays > 0 {
		b.WriteString(fmt.Sprintf("- TLS expiring (< %d days): %d\n", tlsDays, tlsExpiring))
	}
	if len(rows) == 0 {
		b.WriteString("\n_No monitors for selected filters._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Name | Status | Last Down | Last Error |\n|---|---|---|---|\n")
	for _, m := range rows {
		lastDown := "-"
		if m.LastDownAt != nil {
			lastDown = m.LastDownAt.UTC().Format("2006-01-02 15:04")
		}
		lastErr := m.LastError
		if strings.TrimSpace(lastErr) == "" {
			lastErr = "-"
		}
		state := stateByID[m.ID]
		uptime24 := 0.0
		uptime30 := 0.0
		var tlsLeft any
		if state != nil {
			uptime24 = state.Uptime24h
			uptime30 = state.Uptime30d
			if state.TLSDaysLeft != nil {
				tlsLeft = *state.TLSDaysLeft
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			escapePipes(m.Name),
			escapePipes(m.Status),
			escapePipes(lastDown),
			escapePipes(lastErr),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "monitor",
			EntityID:   fmt.Sprintf("%d", m.ID),
			Entity: map[string]any{
				"id":                m.ID,
				"name":              m.Name,
				"status":            m.Status,
				"incident_severity": m.IncidentSeverity,
				"last_down_at":      lastDown,
				"last_error":        lastErr,
				"uptime_24h":        uptime24,
				"uptime_30d":        uptime30,
				"tls_days_left":     tlsLeft,
			},
		})
	}
	if h.policy.Allowed(roles, "monitoring.events.view") {
		from, _ := periodOverride(sec.Config, fallbackFrom, fallbackTo)
		since := time.Now().AddDate(0, 0, -30).UTC()
		if from != nil {
			since = *from
		}
		evLimit := configInt(sec.Config, "events_limit", 20)
		events, _ := h.monitoring.ListEventsFeed(ctx, store.EventFilter{Since: since, Limit: evLimit})
		if len(events) > 0 {
			b.WriteString("\n### Recent events\n\n| Time | Monitor | Type | Message |\n|---|---|---|---|\n")
			monitorNames := map[int64]string{}
			for _, m := range rows {
				monitorNames[m.ID] = m.Name
			}
			for _, ev := range events {
				name := monitorNames[ev.MonitorID]
				if name == "" {
					name = fmt.Sprintf("#%d", ev.MonitorID)
				}
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
					ev.TS.UTC().Format("2006-01-02 15:04"),
					escapePipes(name),
					escapePipes(ev.EventType),
					escapePipes(ev.Message),
				))
				res.Items = append(res.Items, store.ReportSnapshotItem{
					EntityType: "monitor_event",
					EntityID:   fmt.Sprintf("%d", ev.ID),
					Entity: map[string]any{
						"id":         ev.ID,
						"monitor_id": ev.MonitorID,
						"event_type": ev.EventType,
						"message":    ev.Message,
						"ts":         ev.TS.UTC().Format(time.RFC3339),
					},
				})
			}
		}
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildAccessesSection(ctx context.Context, sec store.ReportSection, _ *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	lang := sectionLang(sec)
	isRu := lang == "ru"
	canViewAccesses := h.policy.Allowed(roles, "accesses.view") ||
		h.policy.Allowed(roles, "accounts.view") ||
		h.policy.Allowed(roles, "accounts.manage") ||
		h.policy.Allowed(roles, "app.view")
	if !canViewAccesses {
		res.Denied = true
		if isRu {
			res.Markdown = fmt.Sprintf("## %s\n\n_Нет доступа._", sectionTitle(sec, "Доступы"))
		} else {
			res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Accesses"))
		}
		return res
	}
	if h.modules == nil {
		res.Error = "accesses unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 200)
	if limit <= 0 {
		limit = 200
	}
	topLimit := configInt(sec.Config, "top_limit", 10)
	if topLimit <= 0 {
		topLimit = 10
	}
	from, to := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	statusFilter := strings.ToLower(strings.TrimSpace(configString(sec.Config, "status")))
	userFilter := strings.ToLower(strings.TrimSpace(configString(sec.Config, "user")))
	serviceFilter := splitCSV(configString(sec.Config, "service"))
	serviceNeed := make(map[string]struct{}, len(serviceFilter))
	for _, svc := range serviceFilter {
		serviceNeed[strings.ToUpper(strings.TrimSpace(svc))] = struct{}{}
	}
	st, err := h.modules.Get(ctx, accessesModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		if isRu {
			res.Markdown = fmt.Sprintf("## %s\n\n_Нет данных по доступам._", sectionTitle(sec, "Доступы"))
		} else {
			res.Markdown = fmt.Sprintf("## %s\n\n_No accesses data available._", sectionTitle(sec, "Accesses"))
		}
		return res
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &raw); err != nil {
		res.Error = "load failed"
		return res
	}
	type accessRow struct {
		User       string
		Position   string
		Department string
		Services   []string
		Blocked    bool
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}
	parseTime := func(v any) time.Time {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" {
			return time.Time{}
		}
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			return ts.UTC()
		}
		return time.Time{}
	}
	parseServices := func(v any) []string {
		var out []string
		switch t := v.(type) {
		case []any:
			for _, item := range t {
				val := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", item)))
				if val != "" {
					out = append(out, val)
				}
			}
		case []string:
			for _, item := range t {
				val := strings.ToUpper(strings.TrimSpace(item))
				if val != "" {
					out = append(out, val)
				}
			}
		}
		sort.Strings(out)
		return out
	}
	var rows []accessRow
	for _, item := range raw.Items {
		user := strings.TrimSpace(fmt.Sprintf("%v", item["user"]))
		if user == "" {
			continue
		}
		row := accessRow{
			User:       user,
			Position:   strings.TrimSpace(fmt.Sprintf("%v", item["position"])),
			Department: strings.TrimSpace(fmt.Sprintf("%v", item["department"])),
			Services:   parseServices(item["services"]),
			Blocked:    strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", item["blocked"])), "true"),
			CreatedAt:  parseTime(item["created_at"]),
			UpdatedAt:  parseTime(item["updated_at"]),
		}
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = row.CreatedAt
		}
		if from != nil && !row.UpdatedAt.IsZero() && row.UpdatedAt.Before(*from) {
			continue
		}
		if to != nil && !row.UpdatedAt.IsZero() && row.UpdatedAt.After(*to) {
			continue
		}
		if statusFilter != "" {
			switch statusFilter {
			case "active":
				if row.Blocked {
					continue
				}
			case "blocked":
				if !row.Blocked {
					continue
				}
			}
		}
		if userFilter != "" && !strings.Contains(strings.ToLower(row.User), userFilter) {
			continue
		}
		if len(serviceNeed) > 0 {
			matched := false
			for _, svc := range row.Services {
				if _, ok := serviceNeed[svc]; ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].User == rows[j].User {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return strings.ToLower(rows[i].User) < strings.ToLower(rows[j].User)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	serviceCounts := map[string]int{}
	activeCount := 0
	blockedCount := 0
	for _, row := range rows {
		if row.Blocked {
			blockedCount++
		} else {
			activeCount++
		}
		for _, svc := range row.Services {
			serviceCounts[svc]++
		}
	}
	type serviceStat struct {
		Name  string
		Count int
	}
	stats := make([]serviceStat, 0, len(serviceCounts))
	for name, count := range serviceCounts {
		stats = append(stats, serviceStat{Name: name, Count: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	if len(stats) > topLimit {
		stats = stats[:topLimit]
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"accesses_users":        len(rows),
		"accesses_active":       activeCount,
		"accesses_blocked":      blockedCount,
		"accesses_top_services": len(stats),
	}
	totals["accesses_users"] += len(rows)
	totals["accesses_active"] += activeCount
	totals["accesses_blocked"] += blockedCount
	sectionName := "Accesses"
	usersInScopeLabel := "Users in scope"
	activeLabel := "Active"
	blockedLabel := "Blocked"
	noRowsText := "_No accesses for selected period._"
	topServicesTitle := "Top services"
	serviceCol := "Service"
	usersCol := "Users"
	usersAndServicesTitle := "Users and services"
	userCol := "User"
	statusCol := "Status"
	positionCol := "Position"
	departmentCol := "Department"
	servicesCol := "Services"
	updatedCol := "Updated"
	activeStatusLabel := "Active"
	blockedStatusLabel := "Blocked"
	if isRu {
		sectionName = "Доступы"
		usersInScopeLabel = "Пользователей в выборке"
		activeLabel = "Активных"
		blockedLabel = "Заблокированных"
		noRowsText = "_Нет доступов за выбранный период._"
		topServicesTitle = "Топ сервисов"
		serviceCol = "Сервис"
		usersCol = "Пользователей"
		usersAndServicesTitle = "Пользователи и сервисы"
		userCol = "Пользователь"
		statusCol = "Статус"
		positionCol = "Должность"
		departmentCol = "Отдел"
		servicesCol = "Сервисы"
		updatedCol = "Обновлено"
		activeStatusLabel = "Активен"
		blockedStatusLabel = "Заблокирован"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, sectionName)))
	b.WriteString(fmt.Sprintf("- %s: %d\n", usersInScopeLabel, len(rows)))
	b.WriteString(fmt.Sprintf("- %s: %d\n", activeLabel, activeCount))
	b.WriteString(fmt.Sprintf("- %s: %d\n", blockedLabel, blockedCount))
	if len(rows) == 0 {
		b.WriteString("\n" + noRowsText + "\n")
		res.Markdown = b.String()
		return res
	}
	if len(stats) > 0 {
		b.WriteString(fmt.Sprintf("\n### %s\n\n", topServicesTitle))
		b.WriteString(fmt.Sprintf("| %s | %s |\n|---|---|\n", serviceCol, usersCol))
		for _, st := range stats {
			b.WriteString(fmt.Sprintf("| %s | %d |\n", escapePipes(st.Name), st.Count))
		}
	}
	b.WriteString(fmt.Sprintf("\n### %s\n\n", usersAndServicesTitle))
	b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n|---|---|---|---|---|---|\n", userCol, statusCol, positionCol, departmentCol, servicesCol, updatedCol))
	for _, row := range rows {
		status := activeStatusLabel
		if row.Blocked {
			status = blockedStatusLabel
		}
		updated := "-"
		if !row.UpdatedAt.IsZero() {
			updated = row.UpdatedAt.UTC().Format("2006-01-02 15:04")
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			escapePipes(row.User),
			escapePipes(status),
			escapePipes(emptyDash(row.Position)),
			escapePipes(emptyDash(row.Department)),
			escapePipes(strings.Join(row.Services, ", ")),
			escapePipes(updated),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "access_user",
			EntityID:   row.User,
			Entity: map[string]any{
				"user":       row.User,
				"position":   row.Position,
				"department": row.Department,
				"services":   row.Services,
				"blocked":    row.Blocked,
				"created_at": row.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at": row.UpdatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildAuditSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "logs.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Audit events"))
		return res
	}
	if h.audits == nil {
		res.Error = "audit unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 50)
	from, _ := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	since := time.Now().AddDate(0, 0, -30).UTC()
	if from != nil {
		since = *from
	}
	records, err := h.audits.ListFiltered(ctx, since, limit*2)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	lang := sectionLang(sec)
	importantOnly := configBool(sec.Config, "important_only")
	relatedTo := strings.ToLower(strings.TrimSpace(configString(sec.Config, "related_to")))
	if relatedTo == "" {
		relatedTo = strings.ToLower(strings.TrimSpace(configString(sec.Config, "scope")))
	}
	actionContains := strings.ToLower(strings.TrimSpace(configString(sec.Config, "action")))
	actorFilter := strings.ToLower(strings.TrimSpace(configString(sec.Config, "actor")))
	var rows []store.AuditRecord
	for _, rec := range records {
		if importantOnly && !isImportantAudit(rec.Action) {
			continue
		}
		if actorFilter != "" && !strings.Contains(strings.ToLower(rec.Username), actorFilter) {
			continue
		}
		if actionContains != "" && !strings.Contains(strings.ToLower(rec.Action), actionContains) {
			continue
		}
		if isNoisyAuditEvent(rec.Action) {
			continue
		}
		if relatedTo == "accesses" && !isAccessRelatedAuditAction(rec.Action) {
			continue
		}
		rows = append(rows, rec)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{"audit_events": len(rows)}
	totals["audit_events"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Audit events")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(rows)))
	if len(rows) == 0 {
		b.WriteString("\n_No audit events for selected period._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Time | User | Action | Details |\n|---|---|---|---|\n")
	for _, rec := range rows {
		actionLabel := localizedAuditActionLabel(lang, rec.Action)
		details := localizedAuditDetails(lang, rec.Action, rec.Details)
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			rec.CreatedAt.UTC().Format("2006-01-02 15:04"),
			escapePipes(rec.Username),
			escapePipes(actionLabel),
			escapePipes(details),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "audit",
			EntityID:   fmt.Sprintf("%d", rec.ID),
			Entity: map[string]any{
				"id":           rec.ID,
				"username":     rec.Username,
				"action":       rec.Action,
				"action_label": actionLabel,
				"details":      details,
				"created_at":   rec.CreatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	res.Markdown = b.String()
	return res
}

func sectionLang(sec store.ReportSection) string {
	lang := strings.ToLower(strings.TrimSpace(configString(sec.Config, "lang")))
	if lang == "ru" || lang == "en" {
		return lang
	}
	title := strings.TrimSpace(sec.Title)
	if title == "" {
		return "en"
	}
	for _, r := range title {
		if unicode.Is(unicode.Cyrillic, r) {
			return "ru"
		}
	}
	return "en"
}

func isNoisyAuditEvent(action string) bool {
	act := strings.ToLower(strings.TrimSpace(action))
	noisy := map[string]struct{}{
		"report.list":          {},
		"report.template.list": {},
		"report.settings.view": {},
		"doc.list":             {},
		"folder.list":          {},
		"auth.login_success":   {},
	}
	_, exists := noisy[act]
	return exists
}

func isAccessRelatedAuditAction(action string) bool {
	act := strings.ToLower(strings.TrimSpace(action))
	if strings.HasPrefix(act, "accesses.") {
		return true
	}
	if strings.HasPrefix(act, "notifications.accesses.") {
		return true
	}
	if act == "services.catalog.update" {
		return true
	}
	return false
}

func localizedAuditActionLabel(lang, action string) string {
	act := strings.TrimSpace(action)
	key := strings.ToLower(act)
	ru := map[string]string{
		"accesses.create":             "Доступы: создание",
		"accesses.edit":               "Доступы: редактирование",
		"accesses.supplement":         "Доступы: дополнение",
		"accesses.blocked":            "Доступы: блокировка",
		"accesses.unblocked":          "Доступы: разблокировка",
		"accesses.delete":             "Доступы: удаление",
		"accesses.dismissal":          "Доступы: увольнение",
		"accesses.cleanup":            "Доступы: синхронизация",
		"notifications.accesses.send": "Уведомления: отправка события по доступам",
		"services.catalog.update":     "Справочник сервисов: обновление",
	}
	en := map[string]string{
		"accesses.create":             "Accesses: create",
		"accesses.edit":               "Accesses: edit",
		"accesses.supplement":         "Accesses: supplement",
		"accesses.blocked":            "Accesses: block",
		"accesses.unblocked":          "Accesses: unblock",
		"accesses.delete":             "Accesses: delete",
		"accesses.dismissal":          "Accesses: dismissal",
		"accesses.cleanup":            "Accesses: sync",
		"notifications.accesses.send": "Notifications: access event sent",
		"services.catalog.update":     "Services catalog: updated",
	}
	if lang == "ru" {
		if v, ok := ru[key]; ok {
			return v
		}
	}
	if v, ok := en[key]; ok {
		return v
	}
	return act
}

func localizedAuditDetails(lang, action, details string) string {
	trimmed := strings.TrimSpace(details)
	if trimmed == "" {
		return ""
	}
	act := strings.ToLower(strings.TrimSpace(action))
	if act == "notifications.accesses.send" {
		return localizedAccessDetails(lang, trimmed)
	}
	if strings.HasPrefix(act, "accesses.") {
		return localizedAccessDetails(lang, trimmed)
	}
	return trimmed
}

func localizedAccessDetails(lang, details string) string {
	dictRu := map[string]string{
		"items_count":    "Записей",
		"user":           "Пользователь",
		"services_count": "Сервисов",
		"event":          "Событие",
		"status":         "Статус",
	}
	dictEn := map[string]string{
		"items_count":    "Items",
		"user":           "User",
		"services_count": "Services",
		"event":          "Event",
		"status":         "Status",
	}
	dict := dictEn
	if lang == "ru" {
		dict = dictRu
	}
	parts := strings.Fields(details)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		eq := strings.Index(part, "=")
		if eq <= 0 {
			out = append(out, part)
			continue
		}
		key := strings.TrimSpace(part[:eq])
		val := strings.TrimSpace(part[eq+1:])
		if key == "" {
			continue
		}
		label := dict[key]
		if label == "" {
			label = key
		}
		out = append(out, fmt.Sprintf("%s: %s", label, val))
	}
	return strings.Join(out, " | ")
}

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return strings.TrimSpace(v)
}

func (h *ReportsHandler) buildSummarySection(sec store.ReportSection, totals map[string]int, now time.Time) reportSectionResult {
	res := reportSectionResult{Section: sec}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Executive summary")))
	if len(totals) == 0 {
		b.WriteString("_No summary data._\n")
		res.Markdown = b.String()
		return res
	}
	executive := configBool(sec.Config, "executive")
	if executive {
		b.WriteString("### Key KPIs\n\n")
		b.WriteString(fmt.Sprintf("- Critical incidents: %d\n", totals["incidents_critical"]))
		b.WriteString(fmt.Sprintf("- Overdue tasks: %d\n", totals["tasks_overdue"]))
		b.WriteString(fmt.Sprintf("- Control violations: %d\n", totals["controls_failed"]))
		b.WriteString(fmt.Sprintf("- Monitoring downtime: %d\n", totals["monitors_down"]))
		b.WriteString(fmt.Sprintf("- TLS expiring: %d\n\n", totals["tls_expiring"]))
		if totals["incidents_critical"]+totals["incidents_high"] > 0 {
			b.WriteString("### Top risks\n\n")
			b.WriteString(fmt.Sprintf("- Critical/high incidents present: %d\n\n", totals["incidents_critical"]+totals["incidents_high"]))
		}
	}
	if v := totals["incidents"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Incidents in period: %d\n", v))
	}
	if v := totals["tasks"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Tasks touched: %d\n", v))
	}
	if v := totals["tasks_overdue"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Overdue tasks: %d\n", v))
	}
	if v := totals["docs"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Documents updated: %d\n", v))
	}
	if v := totals["controls"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Controls in scope: %d\n", v))
	}
	if v := totals["monitors"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Monitors tracked: %d\n", v))
	}
	if v := totals["audit_events"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Audit events: %d\n", v))
	}
	if v := totals["accesses_users"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Access users: %d\n", v))
	}
	if v := totals["accesses_active"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Active accesses: %d\n", v))
	}
	if v := totals["accesses_blocked"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Blocked accesses: %d\n", v))
	}
	res.Markdown = b.String()
	return res
}

func isImportantAudit(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return false
	}
	importantPrefixes := []string{
		"incident.", "incidents.", "report.", "reports.", "docs.", "tasks.",
		"controls.", "monitoring.", "accounts.", "accesses.", "approval.", "auth.",
	}
	for _, prefix := range importantPrefixes {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	if strings.Contains(action, "delete") || strings.Contains(action, "export") || strings.Contains(action, "create") {
		return true
	}
	return false
}
