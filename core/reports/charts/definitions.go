package charts

import "berkut-scc/core/store"

type Kind string

const (
	KindBar  Kind = "bar"
	KindLine Kind = "line"
)

type Definition struct {
	Type          string
	TitleKey      string
	SectionType   string
	Kind          Kind
	DefaultConfig map[string]any
}

var definitions = map[string]Definition{
	"incidents_severity_bar": {
		Type:        "incidents_severity_bar",
		TitleKey:    "chart.title.incidents_severity",
		SectionType: "incidents",
		Kind:        KindBar,
	},
	"incidents_status_bar": {
		Type:        "incidents_status_bar",
		TitleKey:    "chart.title.incidents_status",
		SectionType: "incidents",
		Kind:        KindBar,
	},
	"incidents_weekly_line": {
		Type:        "incidents_weekly_line",
		TitleKey:    "chart.title.incidents_weekly",
		SectionType: "incidents",
		Kind:        KindLine,
		DefaultConfig: map[string]any{"weeks": 8},
	},
	"tasks_status_bar": {
		Type:        "tasks_status_bar",
		TitleKey:    "chart.title.tasks_status",
		SectionType: "tasks",
		Kind:        KindBar,
	},
	"tasks_weekly_line": {
		Type:        "tasks_weekly_line",
		TitleKey:    "chart.title.tasks_weekly",
		SectionType: "tasks",
		Kind:        KindLine,
		DefaultConfig: map[string]any{"weeks": 8},
	},
	"docs_approvals_bar": {
		Type:        "docs_approvals_bar",
		TitleKey:    "chart.title.docs_approvals",
		SectionType: "docs",
		Kind:        KindBar,
	},
	"docs_weekly_line": {
		Type:        "docs_weekly_line",
		TitleKey:    "chart.title.docs_weekly",
		SectionType: "docs",
		Kind:        KindLine,
		DefaultConfig: map[string]any{"weeks": 8},
	},
	"controls_status_bar": {
		Type:        "controls_status_bar",
		TitleKey:    "chart.title.controls_status",
		SectionType: "controls",
		Kind:        KindBar,
	},
	"controls_domains_bar": {
		Type:        "controls_domains_bar",
		TitleKey:    "chart.title.controls_domains",
		SectionType: "controls",
		Kind:        KindBar,
		DefaultConfig: map[string]any{"top_n": 6},
	},
	"monitoring_uptime_bar": {
		Type:        "monitoring_uptime_bar",
		TitleKey:    "chart.title.monitoring_uptime",
		SectionType: "monitoring",
		Kind:        KindBar,
		DefaultConfig: map[string]any{"top_n": 6},
	},
	"monitoring_downtime_line": {
		Type:        "monitoring_downtime_line",
		TitleKey:    "chart.title.monitoring_downtime",
		SectionType: "monitoring",
		Kind:        KindLine,
		DefaultConfig: map[string]any{"days": 14},
	},
	"monitoring_tls_bar": {
		Type:        "monitoring_tls_bar",
		TitleKey:    "chart.title.monitoring_tls",
		SectionType: "monitoring",
		Kind:        KindBar,
	},
}

func DefinitionFor(chartType string) (Definition, bool) {
	def, ok := definitions[chartType]
	return def, ok
}

func DefaultCharts() []store.ReportChart {
	order := []string{
		"incidents_severity_bar",
		"incidents_status_bar",
		"incidents_weekly_line",
		"tasks_status_bar",
		"tasks_weekly_line",
		"docs_approvals_bar",
		"docs_weekly_line",
		"controls_status_bar",
		"controls_domains_bar",
		"monitoring_uptime_bar",
		"monitoring_downtime_line",
		"monitoring_tls_bar",
	}
	out := make([]store.ReportChart, 0, len(order))
	for _, key := range order {
		def, ok := definitions[key]
		if !ok {
			continue
		}
		cfg := map[string]any{}
		for k, v := range def.DefaultConfig {
			cfg[k] = v
		}
		out = append(out, store.ReportChart{
			ChartType:  def.Type,
			Title:      def.TitleKey,
			SectionType: def.SectionType,
			Config:     cfg,
			IsEnabled:  true,
		})
	}
	return out
}

func NormalizeConfig(chartType string, cfg map[string]any) map[string]any {
	def, ok := definitions[chartType]
	if !ok {
		return cfg
	}
	out := map[string]any{}
	for k, v := range def.DefaultConfig {
		out[k] = v
	}
	switch chartType {
	case "controls_domains_bar", "monitoring_uptime_bar":
		out["top_n"] = clampInt(cfg, "top_n", intValue(out["top_n"]), 3, 12)
	case "incidents_weekly_line", "tasks_weekly_line", "docs_weekly_line":
		out["weeks"] = clampInt(cfg, "weeks", intValue(out["weeks"]), 4, 16)
	case "monitoring_downtime_line":
		out["days"] = clampInt(cfg, "days", intValue(out["days"]), 7, 31)
	}
	return out
}

func clampInt(cfg map[string]any, key string, def, min, max int) int {
	val := def
	if cfg != nil {
		if raw, ok := cfg[key]; ok {
			switch v := raw.(type) {
			case float64:
				val = int(v)
			case int:
				val = v
			case string:
				if parsed, err := parseInt(v); err == nil {
					val = parsed
				}
			}
		}
	}
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func intValue(val any) int {
	if v, ok := val.(int); ok {
		return v
	}
	if v, ok := val.(float64); ok {
		return int(v)
	}
	return 0
}
