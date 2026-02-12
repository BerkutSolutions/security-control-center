package charts

import (
	"strings"
	"testing"

	"berkut-scc/core/store"
)

func TestRenderSVGReturnsSVG(t *testing.T) {
	data := ChartData{
		Title:  "Chart",
		Kind:   KindBar,
		Labels: []string{"A", "B"},
		Values: []float64{2, 5},
		YLabel: "Count",
	}
	out, err := RenderSVG(data)
	if err != nil {
		t.Fatalf("render svg: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "<svg") || !strings.Contains(text, "</svg>") {
		t.Fatalf("expected svg output")
	}
	if !strings.Contains(text, "Chart") {
		t.Fatalf("expected title in svg")
	}
}

func TestBuildChartUsesSnapshotItems(t *testing.T) {
	ch := store.ReportChart{ChartType: "incidents_severity_bar"}
	items := []store.ReportSnapshotItem{
		{EntityType: "incident", Entity: map[string]any{"severity": "critical"}},
		{EntityType: "incident", Entity: map[string]any{"severity": "high"}},
	}
	data, err := BuildChart(ch, &store.ReportSnapshot{}, items, "en")
	if err != nil {
		t.Fatalf("build chart: %v", err)
	}
	sum := 0.0
	for _, v := range data.Values {
		sum += v
	}
	if sum != 2 {
		t.Fatalf("expected sum 2, got %.0f", sum)
	}
}
