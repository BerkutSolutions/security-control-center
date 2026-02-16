package charts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"berkut-scc/core/store"
)

type ChartData struct {
	Title   string
	Kind    Kind
	Labels  []string
	Values  []float64
	XLabel  string
	YLabel  string
}

func BuildChart(chart store.ReportChart, snapshot *store.ReportSnapshot, items []store.ReportSnapshotItem, lang string) (ChartData, error) {
	def, ok := DefinitionFor(chart.ChartType)
	if !ok {
		return ChartData{}, fmt.Errorf("unknown chart type")
	}
	cfg := NormalizeConfig(chart.ChartType, chart.Config)
	title := chart.Title
	if def.TitleKey != "" {
		title = Localized(lang, def.TitleKey)
	}
	now := time.Now().UTC()
	if snapshot != nil {
		if t, ok := parseSnapshotTime(snapshot, "generated_at"); ok {
			now = t
		}
	}
	from, to := snapshotPeriod(snapshot)
	switch chart.ChartType {
	case "incidents_severity_bar":
		return barFromCounts(title, def.Kind, incidentsSeverityCounts(items, lang), Localized(lang, "chart.axis.count"))
	case "incidents_status_bar":
		return barFromCounts(title, def.Kind, incidentsStatusCounts(items, lang), Localized(lang, "chart.axis.count"))
	case "incidents_weekly_line":
		dates := datesFromItems(items, "incident", "created_at")
		labels, values := weeklyBuckets(dates, from, to, cfg["weeks"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.week"), YLabel: Localized(lang, "chart.axis.count")}, nil
	case "tasks_status_bar":
		done, overdue, inProgress := taskStatusCounts(items, now)
		labels := []string{
			Localized(lang, "chart.label.done"),
			Localized(lang, "chart.label.overdue"),
			Localized(lang, "chart.label.in_progress"),
		}
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: []float64{float64(done), float64(overdue), float64(inProgress)}, YLabel: Localized(lang, "chart.axis.count")}, nil
	case "tasks_weekly_line":
		dates := taskCompletedDates(items)
		labels, values := weeklyBuckets(dates, from, to, cfg["weeks"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.week"), YLabel: Localized(lang, "chart.axis.count")}, nil
	case "docs_approvals_bar":
		labels := []string{
			Localized(lang, "chart.label.approved"),
			Localized(lang, "chart.label.returned"),
			Localized(lang, "chart.label.review"),
		}
		counts := approvalsCounts(items)
		values := []float64{float64(counts["approved"]), float64(counts["returned"]), float64(counts["review"])}
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, YLabel: Localized(lang, "chart.axis.count")}, nil
	case "docs_weekly_line":
		dates := datesFromItems(items, "doc", "created_at")
		labels, values := weeklyBuckets(dates, from, to, cfg["weeks"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.week"), YLabel: Localized(lang, "chart.axis.count")}, nil
	case "controls_status_bar":
		return barFromCounts(title, def.Kind, controlsStatusCounts(items, lang), Localized(lang, "chart.axis.count"))
	case "controls_domains_bar":
		labels, values := controlsDomainCounts(items, cfg["top_n"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.domain"), YLabel: Localized(lang, "chart.axis.count")}, nil
	case "monitoring_uptime_bar":
		labels, values := monitoringUptime(items, cfg["top_n"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.monitor"), YLabel: Localized(lang, "chart.axis.uptime")}, nil
	case "monitoring_downtime_line":
		dates := monitoringDownDates(items)
		labels, values := dailyBuckets(dates, now, cfg["days"].(int))
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, XLabel: Localized(lang, "chart.axis.day"), YLabel: Localized(lang, "chart.axis.count")}, nil
	case "monitoring_tls_bar":
		labels, values := monitoringTLSCounts(items, lang)
		return ChartData{Title: title, Kind: def.Kind, Labels: labels, Values: values, YLabel: Localized(lang, "chart.axis.count")}, nil
	}
	return ChartData{}, fmt.Errorf("unsupported chart type")
}

func barFromCounts(title string, kind Kind, counts map[string]int, yLabel string) (ChartData, error) {
	labels := make([]string, 0, len(counts))
	for key := range counts {
		labels = append(labels, key)
	}
	sort.Strings(labels)
	values := make([]float64, 0, len(labels))
	for _, key := range labels {
		values = append(values, float64(counts[key]))
	}
	return ChartData{Title: title, Kind: kind, Labels: labels, Values: values, YLabel: yLabel}, nil
}

func incidentsSeverityCounts(items []store.ReportSnapshotItem, lang string) map[string]int {
	counts := map[string]int{
		Localized(lang, "chart.severity.critical"): 0,
		Localized(lang, "chart.severity.high"):     0,
		Localized(lang, "chart.severity.medium"):   0,
		Localized(lang, "chart.severity.low"):      0,
		Localized(lang, "chart.label.unknown"):     0,
	}
	for _, item := range items {
		if item.EntityType != "incident" {
			continue
		}
		val := strings.ToLower(strings.TrimSpace(getString(item.Entity, "severity")))
		switch val {
		case "critical":
			counts[Localized(lang, "chart.severity.critical")]++
		case "high":
			counts[Localized(lang, "chart.severity.high")]++
		case "medium":
			counts[Localized(lang, "chart.severity.medium")]++
		case "low":
			counts[Localized(lang, "chart.severity.low")]++
		default:
			counts[Localized(lang, "chart.label.unknown")]++
		}
	}
	return counts
}

func incidentsStatusCounts(items []store.ReportSnapshotItem, lang string) map[string]int {
	counts := map[string]int{
		Localized(lang, "chart.status.open"):        0,
		Localized(lang, "chart.status.in_progress"): 0,
		Localized(lang, "chart.status.resolved"):    0,
		Localized(lang, "chart.status.closed"):      0,
		Localized(lang, "chart.status.draft"):       0,
		Localized(lang, "chart.label.unknown"):      0,
	}
	for _, item := range items {
		if item.EntityType != "incident" {
			continue
		}
		val := strings.ToLower(strings.TrimSpace(getString(item.Entity, "status")))
		switch val {
		case "open":
			counts[Localized(lang, "chart.status.open")]++
		case "in_progress":
			counts[Localized(lang, "chart.status.in_progress")]++
		case "resolved":
			counts[Localized(lang, "chart.status.resolved")]++
		case "closed":
			counts[Localized(lang, "chart.status.closed")]++
		case "draft":
			counts[Localized(lang, "chart.status.draft")]++
		default:
			counts[Localized(lang, "chart.label.unknown")]++
		}
	}
	return counts
}

func controlsStatusCounts(items []store.ReportSnapshotItem, lang string) map[string]int {
	counts := map[string]int{
		Localized(lang, "chart.label.ok"):      0,
		Localized(lang, "chart.label.failed"):  0,
		Localized(lang, "chart.label.unknown"): 0,
	}
	for _, item := range items {
		if item.EntityType != "control" {
			continue
		}
		val := strings.ToLower(strings.TrimSpace(getString(item.Entity, "status")))
		switch val {
		case "ok":
			counts[Localized(lang, "chart.label.ok")]++
		case "failed", "fail", "violation":
			counts[Localized(lang, "chart.label.failed")]++
		default:
			counts[Localized(lang, "chart.label.unknown")]++
		}
	}
	return counts
}

func controlsDomainCounts(items []store.ReportSnapshotItem, topN int) ([]string, []float64) {
	counts := map[string]int{}
	for _, item := range items {
		if item.EntityType != "control" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(getString(item.Entity, "status")))
		if status == "ok" {
			continue
		}
		domain := strings.TrimSpace(getString(item.Entity, "domain"))
		if domain == "" {
			domain = "unknown"
		}
		counts[domain]++
	}
	type pair struct {
		Key   string
		Value int
	}
	var pairs []pair
	for k, v := range counts {
		pairs = append(pairs, pair{Key: k, Value: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Value == pairs[j].Value {
			return pairs[i].Key < pairs[j].Key
		}
		return pairs[i].Value > pairs[j].Value
	})
	if topN > 0 && len(pairs) > topN {
		pairs = pairs[:topN]
	}
	labels := make([]string, 0, len(pairs))
	values := make([]float64, 0, len(pairs))
	for _, p := range pairs {
		labels = append(labels, p.Key)
		values = append(values, float64(p.Value))
	}
	return labels, values
}

func monitoringUptime(items []store.ReportSnapshotItem, topN int) ([]string, []float64) {
	type pair struct {
		Name  string
		Value float64
	}
	var pairs []pair
	for _, item := range items {
		if item.EntityType != "monitor" {
			continue
		}
		sev := strings.ToLower(strings.TrimSpace(getString(item.Entity, "incident_severity")))
		if sev != "critical" {
			continue
		}
		name := strings.TrimSpace(getString(item.Entity, "name"))
		if name == "" {
			name = "monitor"
		}
		val := getFloat(item.Entity, "uptime_24h")
		pairs = append(pairs, pair{Name: name, Value: val})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Value == pairs[j].Value {
			return pairs[i].Name < pairs[j].Name
		}
		return pairs[i].Value < pairs[j].Value
	})
	if topN > 0 && len(pairs) > topN {
		pairs = pairs[:topN]
	}
	labels := make([]string, 0, len(pairs))
	values := make([]float64, 0, len(pairs))
	for _, p := range pairs {
		labels = append(labels, p.Name)
		values = append(values, p.Value)
	}
	return labels, values
}

func monitoringDownDates(items []store.ReportSnapshotItem) []time.Time {
	var dates []time.Time
	for _, item := range items {
		if item.EntityType != "monitor_event" {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(getString(item.Entity, "event_type")))
		if typ == "" || (!strings.Contains(typ, "down") && !strings.Contains(typ, "fail")) {
			continue
		}
		if ts, ok := getTime(item.Entity, "ts"); ok {
			dates = append(dates, ts)
		}
	}
	return dates
}

func monitoringTLSCounts(items []store.ReportSnapshotItem, lang string) ([]string, []float64) {
	lt7, lt30, lt90 := 0, 0, 0
	for _, item := range items {
		if item.EntityType != "monitor" {
			continue
		}
		val := getInt(item.Entity, "tls_days_left")
		if val == nil {
			continue
		}
		switch {
		case *val < 7:
			lt7++
		case *val < 30:
			lt30++
		case *val < 90:
			lt90++
		}
	}
	labels := []string{
		Localized(lang, "chart.label.lt7"),
		Localized(lang, "chart.label.lt30"),
		Localized(lang, "chart.label.lt90"),
	}
	values := []float64{float64(lt7), float64(lt30), float64(lt90)}
	return labels, values
}

func approvalsCounts(items []store.ReportSnapshotItem) map[string]int {
	counts := map[string]int{"approved": 0, "returned": 0, "review": 0}
	for _, item := range items {
		if item.EntityType != "doc" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(getString(item.Entity, "approval_status")))
		if _, ok := counts[status]; ok {
			counts[status]++
		}
	}
	return counts
}

func taskStatusCounts(items []store.ReportSnapshotItem, now time.Time) (int, int, int) {
	done := 0
	overdue := 0
	inProgress := 0
	for _, item := range items {
		if item.EntityType != "task" {
			continue
		}
		if taskDone(item) {
			done++
			continue
		}
		if due, ok := getDate(item.Entity, "due_date"); ok && due.Before(now) {
			overdue++
			continue
		}
		inProgress++
	}
	return done, overdue, inProgress
}

func taskCompletedDates(items []store.ReportSnapshotItem) []time.Time {
	var dates []time.Time
	for _, item := range items {
		if item.EntityType != "task" {
			continue
		}
		if !taskDone(item) {
			continue
		}
		if ts, ok := getTime(item.Entity, "closed_at"); ok {
			dates = append(dates, ts)
		} else if ts, ok := getTime(item.Entity, "updated_at"); ok {
			dates = append(dates, ts)
		}
	}
	return dates
}

func taskDone(item store.ReportSnapshotItem) bool {
	if strings.ToLower(strings.TrimSpace(getString(item.Entity, "status"))) == "done" {
		return true
	}
	if strings.ToLower(strings.TrimSpace(getString(item.Entity, "status"))) == "closed" {
		return true
	}
	if strings.ToLower(strings.TrimSpace(getString(item.Entity, "status"))) == "completed" {
		return true
	}
	if getBool(item.Entity, "is_archived") {
		return true
	}
	if _, ok := getTime(item.Entity, "closed_at"); ok {
		return true
	}
	return false
}

func datesFromItems(items []store.ReportSnapshotItem, entityType, field string) []time.Time {
	var dates []time.Time
	for _, item := range items {
		if item.EntityType != entityType {
			continue
		}
		if ts, ok := getTime(item.Entity, field); ok {
			dates = append(dates, ts)
		}
	}
	return dates
}

func snapshotPeriod(snapshot *store.ReportSnapshot) (*time.Time, *time.Time) {
	if snapshot == nil || snapshot.Snapshot == nil {
		return nil, nil
	}
	var from, to *time.Time
	if t, ok := parseSnapshotTime(snapshot, "period_from"); ok {
		from = &t
	}
	if t, ok := parseSnapshotTime(snapshot, "period_to"); ok {
		to = &t
	}
	return from, to
}

func parseSnapshotTime(snapshot *store.ReportSnapshot, key string) (time.Time, bool) {
	if snapshot == nil || snapshot.Snapshot == nil {
		return time.Time{}, false
	}
	raw, ok := snapshot.Snapshot[key]
	if !ok {
		return time.Time{}, false
	}
	val, ok := raw.(string)
	if !ok || strings.TrimSpace(val) == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, val); err == nil {
		return parsed.UTC(), true
	}
	if parsed, err := time.Parse("2006-01-02", val); err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

func weeklyBuckets(dates []time.Time, from, to *time.Time, weeks int) ([]string, []float64) {
	if weeks <= 0 {
		weeks = 8
	}
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -7*(weeks-1))
	end := now
	if from != nil {
		start = *from
	}
	if to != nil {
		end = *to
	}
	start = startOfWeek(start)
	end = startOfWeek(end)
	totalWeeks := int(end.Sub(start).Hours()/(24*7)) + 1
	if totalWeeks > weeks {
		start = start.AddDate(0, 0, 7*(totalWeeks-weeks))
		totalWeeks = weeks
	}
	labels := make([]string, totalWeeks)
	values := make([]float64, totalWeeks)
	for i := 0; i < totalWeeks; i++ {
		labels[i] = start.AddDate(0, 0, 7*i).Format("2006-01-02")
	}
	for _, dt := range dates {
		if dt.Before(start) || dt.After(end.AddDate(0, 0, 6)) {
			continue
		}
		idx := int(dt.Sub(start).Hours() / (24 * 7))
		if idx >= 0 && idx < totalWeeks {
			values[idx]++
		}
	}
	return labels, values
}

func dailyBuckets(dates []time.Time, now time.Time, days int) ([]string, []float64) {
	if days <= 0 {
		days = 14
	}
	start := truncateDay(now.AddDate(0, 0, -(days-1)))
	labels := make([]string, days)
	values := make([]float64, days)
	for i := 0; i < days; i++ {
		labels[i] = start.AddDate(0, 0, i).Format("2006-01-02")
	}
	for _, dt := range dates {
		dt = truncateDay(dt)
		if dt.Before(start) || dt.After(start.AddDate(0, 0, days-1)) {
			continue
		}
		idx := int(dt.Sub(start).Hours() / 24)
		if idx >= 0 && idx < days {
			values[idx]++
		}
	}
	return labels, values
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return truncateDay(t.AddDate(0, 0, -(weekday - 1)))
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
