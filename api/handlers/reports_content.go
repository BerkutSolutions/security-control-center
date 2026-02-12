package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"berkut-scc/core/docs"
)

func (h *ReportsHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	ver, err := h.docs.GetVersion(r.Context(), doc.ID, doc.CurrentVersion)
	if err != nil || ver == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	content, err := h.svc.LoadContent(r.Context(), ver)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.view", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"format":  ver.Format,
		"version": ver.Version,
		"content": string(content),
	})
}

func (h *ReportsHandler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "edit")
	if !ok {
		return
	}
	var payload struct {
		Content string `json:"content"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Reason) == "" {
		http.Error(w, localized(preferredLang(r), "editor.reasonRequired"), http.StatusBadRequest)
		return
	}
	v, err := h.svc.SaveVersion(r.Context(), docs.SaveRequest{
		Doc:      doc,
		Author:   user,
		Format:   docs.FormatMarkdown,
		Content:  []byte(payload.Content),
		Reason:   payload.Reason,
		IndexFTS: true,
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	doc.CurrentVersion = v.Version
	_ = h.docs.UpdateDocument(r.Context(), doc)
	h.log(r.Context(), user.Username, "report.edit", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"version": v.Version})
}
