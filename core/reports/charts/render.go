package charts

import (
	"bytes"
	"fmt"
	"math"
	"strings"
)

const (
	chartWidth  = 640
	chartHeight = 320
	paddingTop  = 30
	paddingRight = 16
	paddingBottom = 52
	paddingLeft = 52
)

func RenderSVG(data ChartData) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">", chartWidth, chartHeight, chartWidth, chartHeight))
	buf.WriteString("<rect width=\"100%\" height=\"100%\" fill=\"#ffffff\"/>")
	buf.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"18\" font-family=\"Arial, sans-serif\" font-size=\"14\" fill=\"#111827\">%s</text>", paddingLeft, escapeXML(data.Title)))
	plotW := chartWidth - paddingLeft - paddingRight
	plotH := chartHeight - paddingTop - paddingBottom
	if plotW <= 0 || plotH <= 0 {
		buf.WriteString("</svg>")
		return buf.Bytes(), nil
	}
	maxVal := maxValue(data.Values)
	if maxVal <= 0 {
		maxVal = 1
	}
	drawAxes(&buf, plotW, plotH, maxVal, data)
	switch data.Kind {
	case KindLine:
		drawLineSeries(&buf, plotW, plotH, data.Values)
	default:
		drawBars(&buf, plotW, plotH, data.Values)
	}
	drawLabels(&buf, plotW, plotH, data.Labels)
	buf.WriteString("</svg>")
	return buf.Bytes(), nil
}

func drawAxes(buf *bytes.Buffer, plotW, plotH int, maxVal float64, data ChartData) {
	x0 := paddingLeft
	y0 := paddingTop + plotH
	buf.WriteString(fmt.Sprintf("<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#d1d5db\" stroke-width=\"1\"/>", x0, y0, x0+plotW, y0))
	buf.WriteString(fmt.Sprintf("<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#d1d5db\" stroke-width=\"1\"/>", x0, paddingTop, x0, y0))
	steps := 4
	for i := 0; i <= steps; i++ {
		val := maxVal * float64(i) / float64(steps)
		y := y0 - int((val/maxVal)*float64(plotH))
		buf.WriteString(fmt.Sprintf("<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#eef2f7\" stroke-width=\"1\"/>", x0, y, x0+plotW, y))
		buf.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" font-family=\"Arial, sans-serif\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"end\">%s</text>", x0-6, y+4, formatNumber(val)))
	}
	if strings.TrimSpace(data.YLabel) != "" {
		buf.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" font-family=\"Arial, sans-serif\" font-size=\"11\" fill=\"#6b7280\">%s</text>", x0, paddingTop-8, escapeXML(data.YLabel)))
	}
	if strings.TrimSpace(data.XLabel) != "" {
		buf.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" font-family=\"Arial, sans-serif\" font-size=\"11\" fill=\"#6b7280\" text-anchor=\"end\">%s</text>", x0+plotW, y0+38, escapeXML(data.XLabel)))
	}
}

func drawBars(buf *bytes.Buffer, plotW, plotH int, values []float64) {
	if len(values) == 0 {
		return
	}
	maxVal := maxValue(values)
	if maxVal <= 0 {
		maxVal = 1
	}
	count := len(values)
	barGap := 6.0
	barW := (float64(plotW) - barGap*float64(count-1)) / float64(count)
	if barW < 6 {
		barW = 6
	}
	for i, v := range values {
		height := (v / maxVal) * float64(plotH)
		x := float64(paddingLeft) + float64(i)*(barW+barGap)
		y := float64(paddingTop) + float64(plotH) - height
		buf.WriteString(fmt.Sprintf("<rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"#5d86ff\"/>", x, y, barW, height))
	}
}

func drawLineSeries(buf *bytes.Buffer, plotW, plotH int, values []float64) {
	if len(values) == 0 {
		return
	}
	maxVal := maxValue(values)
	if maxVal <= 0 {
		maxVal = 1
	}
	step := float64(plotW)
	if len(values) > 1 {
		step = float64(plotW) / float64(len(values)-1)
	}
	var path strings.Builder
	for i, v := range values {
		x := float64(paddingLeft) + step*float64(i)
		y := float64(paddingTop) + float64(plotH) - (v/maxVal)*float64(plotH)
		if i == 0 {
			path.WriteString(fmt.Sprintf("M %.1f %.1f", x, y))
		} else {
			path.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
		}
		buf.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"2.5\" fill=\"#5d86ff\"/>", x, y))
	}
	buf.WriteString(fmt.Sprintf("<path d=\"%s\" fill=\"none\" stroke=\"#5d86ff\" stroke-width=\"2\"/>", path.String()))
}

func drawLabels(buf *bytes.Buffer, plotW, plotH int, labels []string) {
	if len(labels) == 0 {
		return
	}
	count := len(labels)
	step := float64(plotW)
	if count > 1 {
		step = float64(plotW) / float64(count-1)
	}
	y := paddingTop + plotH + 18
	for i, label := range labels {
		x := float64(paddingLeft) + step*float64(i)
		buf.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%d\" font-family=\"Arial, sans-serif\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"middle\">%s</text>", x, y, escapeXML(trimLabel(label, 12))))
	}
}

func trimLabel(label string, limit int) string {
	if len(label) <= limit {
		return label
	}
	if limit <= 3 {
		return label[:limit]
	}
	return label[:limit-3] + "..."
}

func maxValue(values []float64) float64 {
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func formatNumber(val float64) string {
	if math.Abs(val-math.Round(val)) < 0.001 {
		return fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%.1f", val)
}

func escapeXML(val string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(val)
}
