package handlers

import (
	"encoding/json"
	"net/http"
)

func (h *ReportsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	settings, err := h.reports.GetReportSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.settings.view", "")
	writeJSON(w, http.StatusOK, settings)
}

func (h *ReportsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		DefaultClassification string `json:"default_classification"`
		DefaultTemplateID     *int64 `json:"default_template_id"`
		HeaderEnabled         bool   `json:"header_enabled"`
		HeaderLogoPath        string `json:"header_logo_path"`
		HeaderTitle           string `json:"header_title"`
		WatermarkThreshold    string `json:"watermark_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	settings, err := h.reports.GetReportSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if payload.DefaultClassification != "" {
		settings.DefaultClassification = payload.DefaultClassification
	}
	settings.DefaultTemplateID = payload.DefaultTemplateID
	settings.HeaderEnabled = payload.HeaderEnabled
	if payload.HeaderLogoPath != "" {
		settings.HeaderLogoPath = payload.HeaderLogoPath
	}
	if payload.HeaderTitle != "" {
		settings.HeaderTitle = payload.HeaderTitle
	}
	settings.WatermarkThreshold = payload.WatermarkThreshold
	if err := h.reports.UpdateReportSettings(r.Context(), settings); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.settings.update", "")
	writeJSON(w, http.StatusOK, settings)
}
