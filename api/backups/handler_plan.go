package backups

import (
	"encoding/json"
	"net/http"
	"strconv"

	corebackups "berkut-scc/core/backups"
)

type planPayload struct {
	Enabled             bool   `json:"enabled"`
	CronExpression      string `json:"cron_expression"`
	ScheduleType        string `json:"schedule_type"`
	ScheduleWeekday     int    `json:"schedule_weekday"`
	ScheduleMonthAnchor string `json:"schedule_month_anchor"`
	ScheduleHour        int    `json:"schedule_hour"`
	ScheduleMinute      int    `json:"schedule_minute"`
	RetentionDays       int    `json:"retention_days"`
	KeepLastSuccessful  int    `json:"keep_last_successful"`
	IncludeFiles        bool   `json:"include_files"`
}

func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	item, err := h.svc.GetPlan(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditPlanRead, "success", "")
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	payload := planPayload{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, corebackups.ErrorCodeInvalidRequest, corebackups.ErrorKeyInvalidRequest)
		return
	}
	item, err := h.svc.UpdatePlan(r.Context(), corebackups.BackupPlan{
		Enabled:             payload.Enabled,
		CronExpression:      payload.CronExpression,
		ScheduleType:        payload.ScheduleType,
		ScheduleWeekday:     payload.ScheduleWeekday,
		ScheduleMonthAnchor: payload.ScheduleMonthAnchor,
		ScheduleHour:        payload.ScheduleHour,
		ScheduleMinute:      payload.ScheduleMinute,
		RetentionDays:       payload.RetentionDays,
		KeepLastSuccessful:  payload.KeepLastSuccessful,
		IncludeFiles:        payload.IncludeFiles,
	}, session.Username)
	if err != nil {
		if de, ok := corebackups.AsDomainError(err); ok {
			writeError(w, http.StatusBadRequest, de.Code, de.I18NKey)
			return
		}
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditPlanUpdate, "success", "enabled="+strconv.FormatBool(item.Enabled))
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) EnablePlan(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	item, err := h.svc.EnablePlan(r.Context(), session.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditPlanEnable, "success", "event=backups.plan.enabled")
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) DisablePlan(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	item, err := h.svc.DisablePlan(r.Context(), session.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditPlanDisable, "success", "event=backups.plan.disabled")
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}
