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

func (h *AssetsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "assets.manage")
	q := r.URL.Query()
	filter := store.AssetFilter{
		Search:      q.Get("q"),
		Type:        q.Get("type"),
		Criticality: q.Get("criticality"),
		Env:         q.Get("env"),
		Status:      q.Get("status"),
		Tag:         q.Get("tag"),
		Limit:       parseIntDefault(q.Get("limit"), 5000),
		Offset:      0,
	}
	if canManage && (q.Get("include_deleted") == "1" || strings.ToLower(q.Get("include_deleted")) == "true") {
		filter.IncludeDeleted = true
	}
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}

	items, err := h.store.ListAssets(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	filename := fmt.Sprintf("assets_%s.csv", now.Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)

	h.logAudit(r.Context(), user.Username, "assets.export.csv", strconv.Itoa(len(items)))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"id", "name", "type", "description", "commissioned_at", "ip_addresses", "criticality", "owner", "administrator",
		"env", "status", "tags", "created_at", "updated_at", "deleted_at",
	})
	for _, a := range items {
		commissioned := ""
		if a.CommissionedAt != nil {
			commissioned = a.CommissionedAt.UTC().Format("2006-01-02")
		}
		deleted := ""
		if a.DeletedAt != nil {
			deleted = a.DeletedAt.UTC().Format(time.RFC3339)
		}
		_ = writer.Write([]string{
			strconv.FormatInt(a.ID, 10),
			a.Name,
			a.Type,
			a.Description,
			commissioned,
			strings.Join(a.IPAddresses, ";"),
			a.Criticality,
			a.Owner,
			a.Administrator,
			a.Env,
			a.Status,
			strings.Join(a.Tags, ";"),
			a.CreatedAt.UTC().Format(time.RFC3339),
			a.UpdatedAt.UTC().Format(time.RFC3339),
			deleted,
		})
	}
	writer.Flush()
}
