package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

type reportBuildPayload struct {
	Reason     string `json:"reason"`
	Mode       string `json:"mode"`
	PeriodFrom string `json:"period_from"`
	PeriodTo   string `json:"period_to"`
}

func (h *ReportsHandler) Build(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, roles, ok := h.loadReportForAccess(w, r, "edit")
	if !ok {
		return
	}
	u, rRoles, groups, eff, err := h.currentUserWithAccess(r)
	if err == nil && u != nil {
		user = u
		roles = rRoles
	} else {
		eff.ClearanceLevel = user.ClearanceLevel
		eff.ClearanceTags = user.ClearanceTags
		eff.Roles = roles
	}
	var payload reportBuildPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Reason) == "" {
		http.Error(w, localized(preferredLang(r), "editor.reasonRequired"), http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode == "" {
		mode = "markers"
	}
	if meta == nil {
		meta = &store.ReportMeta{DocID: doc.ID, Status: "draft"}
	}
	if payload.PeriodFrom != "" || payload.PeriodTo != "" {
		start, err := parseDateStrict(payload.PeriodFrom)
		if err != nil && payload.PeriodFrom != "" {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidPeriod"), http.StatusBadRequest)
			return
		}
		end, err := parseDateStrict(payload.PeriodTo)
		if err != nil && payload.PeriodTo != "" {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidPeriod"), http.StatusBadRequest)
			return
		}
		meta.PeriodFrom = start
		meta.PeriodTo = end
		if err := h.reports.UpsertReportMeta(r.Context(), meta); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	sections, err := h.reports.ListReportSections(r.Context(), doc.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if len(sections) == 0 {
		sections = defaultReportSections()
	}
	if (meta == nil || (meta.PeriodFrom == nil && meta.PeriodTo == nil)) && requiresReportPeriod(sections) {
		http.Error(w, localized(preferredLang(r), "reports.error.periodRequired"), http.StatusBadRequest)
		return
	}
	sectionResults, snapshotItems, snapshotPayload, err := h.buildReportSections(r.Context(), doc, meta, sections, user, roles, groups, eff)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var baseContent string
	if mode == "markers" {
		if ver, err := h.docs.GetVersion(r.Context(), doc.ID, doc.CurrentVersion); err == nil && ver != nil {
			if content, err := h.svc.LoadContent(r.Context(), ver); err == nil {
				baseContent = string(content)
			}
		}
	}
	nextContent := applySectionContent(mode, baseContent, sectionResults)
	v, err := h.svc.SaveVersion(r.Context(), docs.SaveRequest{
		Doc:      doc,
		Author:   user,
		Format:   docs.FormatMarkdown,
		Content:  []byte(nextContent),
		Reason:   payload.Reason,
		IndexFTS: true,
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	doc.CurrentVersion = v.Version
	_ = h.docs.UpdateDocument(r.Context(), doc)

	payloadBytes, _ := json.Marshal(snapshotPayload)
	sum := sha256.Sum256(payloadBytes)
	snapshot := &store.ReportSnapshot{
		ReportID:      doc.ID,
		CreatedAt:     time.Now().UTC(),
		CreatedBy:     user.ID,
		Reason:        payload.Reason,
		Snapshot:      snapshotPayload,
		SnapshotJSON:  string(payloadBytes),
		Sha256:        hex.EncodeToString(sum[:]),
	}
	if _, err := h.reports.CreateReportSnapshot(r.Context(), snapshot, snapshotItems); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.build", doc.RegNumber)
	h.log(r.Context(), user.Username, "report.snapshot.create", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  v.Version,
		"snapshot": snapshot.ID,
	})
}

func (h *ReportsHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	items, err := h.reports.ListReportSnapshots(r.Context(), doc.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.snapshots.view", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ReportsHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	id := parseInt64Default(r.URL.Query().Get("snapshot_id"), 0)
	if id == 0 {
		if idStr := mux.Vars(r)["snapshot_id"]; idStr != "" {
			id = parseInt64Default(idStr, 0)
		}
	}
	if id == 0 {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	snap, items, err := h.reports.GetReportSnapshot(r.Context(), id)
	if err != nil || snap == nil || snap.ReportID != doc.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.log(r.Context(), user.Username, "report.snapshot.view", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": snap, "items": items})
}

func requiresReportPeriod(sections []store.ReportSection) bool {
	for _, sec := range sections {
		if !sec.IsEnabled {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(sec.SectionType)) {
		case "custom_md":
			continue
		default:
			return true
		}
	}
	return false
}
