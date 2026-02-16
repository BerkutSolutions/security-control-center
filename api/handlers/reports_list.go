package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
)

func (h *ReportsHandler) List(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	filter := store.ReportFilter{
		Search:   strings.TrimSpace(r.URL.Query().Get("q")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Tags:     splitCSV(r.URL.Query().Get("tag")),
		Limit:    parseIntDefault(r.URL.Query().Get("limit"), 0),
		Offset:   parseIntDefault(r.URL.Query().Get("offset"), 0),
		DateFrom: parseDateParam(r.URL.Query().Get("date_from")),
		DateTo:   parseDateParam(r.URL.Query().Get("date_to")),
	}
	filter.ClassificationLevel = -1
	if r.URL.Query().Get("mine") == "1" {
		filter.MineUserID = user.ID
	}
	if cls := strings.TrimSpace(r.URL.Query().Get("classification")); cls != "" {
		level, err := docs.ParseLevel(cls)
		if err != nil {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidClassification"), http.StatusBadRequest)
			return
		}
		filter.ClassificationLevel = int(level)
	}
	items, err := h.reports.ListReports(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var out []store.ReportRecord
	for _, item := range items {
		if !h.isReport(&item.Document) {
			continue
		}
		docACL, _ := h.docs.GetDocACL(r.Context(), item.Document.ID)
		if !h.svc.CheckACL(user, roles, &item.Document, docACL, nil, "view") {
			continue
		}
		out = append(out, item)
	}
	h.log(r.Context(), user.Username, "report.list", fmt.Sprintf("%d", len(out)))
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      out,
		"converters": h.svc.ConvertersStatus(),
	})
}
