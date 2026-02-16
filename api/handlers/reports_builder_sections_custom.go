package handlers

import (
	"fmt"
	"strings"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildCustomSection(sec store.ReportSection) reportSectionResult {
	res := reportSectionResult{Section: sec}
	md := configString(sec.Config, "markdown")
	if md == "" {
		md = configString(sec.Config, "content")
	}
	if md == "" {
		md = fmt.Sprintf("## %s\n\n_No content._", sectionTitle(sec, "Notes"))
	} else if !strings.HasPrefix(strings.TrimSpace(md), "#") {
		md = fmt.Sprintf("## %s\n\n%s", sectionTitle(sec, "Notes"), md)
	}
	res.Markdown = md
	return res
}
