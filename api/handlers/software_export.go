package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *SoftwareHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "software.manage")
	q := r.URL.Query()
	filter := store.SoftwareFilter{
		Search: q.Get("q"),
		Vendor: q.Get("vendor"),
		Tag:    q.Get("tag"),
		Limit:  parseIntDefault(q.Get("limit"), 5000),
		Offset: 0,
	}
	if canManage && parseBool(q.Get("include_deleted")) {
		filter.IncludeDeleted = true
	}
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}

	items, err := h.store.ListProducts(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	filename := fmt.Sprintf("software_%s.csv", now.Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)

	h.audit(r, softwareAuditExportCSV, strconv.Itoa(len(items)))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "name", "vendor", "description", "tags", "created_at", "updated_at", "deleted_at"})
	for _, p := range items {
		deleted := ""
		if p.DeletedAt != nil {
			deleted = p.DeletedAt.UTC().Format(time.RFC3339)
		}
		_ = writer.Write([]string{
			strconv.FormatInt(p.ID, 10),
			p.Name,
			p.Vendor,
			p.Description,
			strings.Join(p.Tags, ";"),
			p.CreatedAt.UTC().Format(time.RFC3339),
			p.UpdatedAt.UTC().Format(time.RFC3339),
			deleted,
		})
	}
	writer.Flush()
}
