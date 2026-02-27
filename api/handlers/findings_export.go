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

func (h *FindingsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.view")
	if !ok {
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(sess.Roles, "findings.manage")
	q := r.URL.Query()
	filter := store.FindingFilter{
		Search:   q.Get("q"),
		Status:   strings.ToLower(strings.TrimSpace(q.Get("status"))),
		Severity: strings.ToLower(strings.TrimSpace(q.Get("severity"))),
		Type:     strings.ToLower(strings.TrimSpace(q.Get("type"))),
		Tag:      q.Get("tag"),
		Limit:    parseIntDefault(q.Get("limit"), 5000),
		Offset:   0,
	}
	if canManage && parseBool(q.Get("include_deleted")) {
		filter.IncludeDeleted = true
	}
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}

	items, err := h.store.ListFindings(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	filename := fmt.Sprintf("findings_%s.csv", now.Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)

	h.audit(r, "finding.export.csv", strconv.Itoa(len(items)))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"id", "title", "description_md", "status", "severity", "type", "owner", "due_at", "tags",
		"created_at", "updated_at", "deleted_at",
	})
	for _, f := range items {
		due := ""
		if f.DueAt != nil {
			due = f.DueAt.UTC().Format("2006-01-02")
		}
		deleted := ""
		if f.DeletedAt != nil {
			deleted = f.DeletedAt.UTC().Format(time.RFC3339)
		}
		_ = writer.Write([]string{
			strconv.FormatInt(f.ID, 10),
			f.Title,
			f.DescriptionMD,
			f.Status,
			f.Severity,
			f.FindingType,
			f.Owner,
			due,
			strings.Join(f.Tags, ";"),
			f.CreatedAt.UTC().Format(time.RFC3339),
			f.UpdatedAt.UTC().Format(time.RFC3339),
			deleted,
		})
	}
	writer.Flush()
}
