package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
	"github.com/robfig/cron/v3"
)

type maintenanceSchedulePayload struct {
	CronExpression string    `json:"cron_expression"`
	DurationMin    int       `json:"duration_min"`
	IntervalDays   int       `json:"interval_days"`
	Weekdays       []int     `json:"weekdays"`
	MonthDays      []int     `json:"month_days"`
	UseLastDay     bool      `json:"use_last_day"`
	WindowStart    string    `json:"window_start"`
	WindowEnd      string    `json:"window_end"`
	ActiveFrom     time.Time `json:"active_from"`
	ActiveUntil    time.Time `json:"active_until"`
}

type maintenancePayload struct {
	Name          string                     `json:"name"`
	DescriptionMD string                     `json:"description_md"`
	MonitorID     *int64                     `json:"monitor_id"`
	MonitorIDs    []int64                    `json:"monitor_ids"`
	Tags          []string                   `json:"tags"`
	StartsAt      time.Time                  `json:"starts_at"`
	EndsAt        time.Time                  `json:"ends_at"`
	Timezone      string                     `json:"timezone"`
	Strategy      string                     `json:"strategy"`
	Schedule      maintenanceSchedulePayload `json:"schedule"`
	IsRecurring   *bool                      `json:"is_recurring"`
	RRuleText     string                     `json:"rrule_text"`
	IsActive      *bool                      `json:"is_active"`
}

func (h *MonitoringHandler) ListMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.maintenance.view") {
		return
	}
	q := r.URL.Query()
	filter := store.MaintenanceFilter{}
	if val := strings.TrimSpace(q.Get("active")); val != "" {
		b := val == "1" || strings.ToLower(val) == "true"
		filter.Active = &b
	}
	if val := strings.TrimSpace(q.Get("monitor_id")); val != "" {
		if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			filter.MonitorID = &id
		}
	}
	items, err := h.store.ListMaintenance(r.Context(), filter)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.maintenance.manage") {
		return
	}
	var payload maintenancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	item, err := payloadToMaintenance(payload, sessionUserID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.ensureMaintenanceMonitorIDs(r, item.MonitorIDs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := h.store.CreateMaintenance(r.Context(), item)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	item.ID = id
	h.audit(r, monitorAuditMaintenanceCreate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, item)
}

func (h *MonitoringHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.maintenance.manage") {
		return
	}
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetMaintenance(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	var payload maintenancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	item, err := mergeMaintenance(existing, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.ensureMaintenanceMonitorIDs(r, item.MonitorIDs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateMaintenance(r.Context(), item); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMaintenanceUpdate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, item)
}

func (h *MonitoringHandler) StopMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.maintenance.manage") {
		return
	}
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if err := h.store.StopMaintenance(r.Context(), id, sessionUserID(r)); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMaintenanceStop, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.maintenance.manage") {
		return
	}
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteMaintenance(r.Context(), id); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMaintenanceDelete, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func payloadToMaintenance(payload maintenancePayload, createdBy int64) (*store.MonitorMaintenance, error) {
	item := &store.MonitorMaintenance{
		Name:          strings.TrimSpace(payload.Name),
		DescriptionMD: strings.TrimSpace(payload.DescriptionMD),
		MonitorID:     payload.MonitorID,
		MonitorIDs:    payload.MonitorIDs,
		Tags:          payload.Tags,
		StartsAt:      payload.StartsAt.UTC(),
		EndsAt:        payload.EndsAt.UTC(),
		Timezone:      strings.TrimSpace(payload.Timezone),
		Strategy:      strings.ToLower(strings.TrimSpace(payload.Strategy)),
		Schedule:      payloadToMaintenanceSchedule(payload.Schedule),
		RRuleText:     strings.TrimSpace(payload.RRuleText),
		CreatedBy:     createdBy,
	}
	if payload.IsRecurring != nil {
		item.IsRecurring = *payload.IsRecurring
	}
	if payload.IsActive != nil {
		item.IsActive = *payload.IsActive
	} else {
		item.IsActive = true
	}
	if item.Strategy == "" {
		item.Strategy = defaultMaintenanceStrategy(item)
	}
	if err := validateMaintenance(item); err != nil {
		return nil, err
	}
	return item, nil
}

func mergeMaintenance(existing *store.MonitorMaintenance, payload maintenancePayload) (*store.MonitorMaintenance, error) {
	item := *existing
	if payload.Name != "" {
		item.Name = strings.TrimSpace(payload.Name)
	}
	if payload.DescriptionMD != "" || (payload.DescriptionMD == "" && strings.TrimSpace(payload.Name) != "") {
		item.DescriptionMD = strings.TrimSpace(payload.DescriptionMD)
	}
	if payload.MonitorID != nil {
		item.MonitorID = payload.MonitorID
	}
	if payload.MonitorIDs != nil {
		item.MonitorIDs = payload.MonitorIDs
	}
	if payload.Tags != nil {
		item.Tags = payload.Tags
	}
	if !payload.StartsAt.IsZero() {
		item.StartsAt = payload.StartsAt.UTC()
	}
	if !payload.EndsAt.IsZero() {
		item.EndsAt = payload.EndsAt.UTC()
	}
	if payload.Timezone != "" {
		item.Timezone = strings.TrimSpace(payload.Timezone)
	}
	if payload.Strategy != "" {
		item.Strategy = strings.ToLower(strings.TrimSpace(payload.Strategy))
	}
	if scheduleChanged(payload.Schedule) {
		item.Schedule = payloadToMaintenanceSchedule(payload.Schedule)
	}
	if payload.IsRecurring != nil {
		item.IsRecurring = *payload.IsRecurring
	}
	if payload.RRuleText != "" || (payload.IsRecurring != nil && !*payload.IsRecurring) {
		item.RRuleText = strings.TrimSpace(payload.RRuleText)
	}
	if payload.IsActive != nil {
		item.IsActive = *payload.IsActive
	}
	if item.Strategy == "" {
		item.Strategy = defaultMaintenanceStrategy(&item)
	}
	if err := validateMaintenance(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func validateMaintenance(item *store.MonitorMaintenance) error {
	if item == nil {
		return errors.New("monitoring.error.invalidMaintenance")
	}
	if item.Name == "" {
		return errors.New("monitoring.error.nameRequired")
	}
	if strings.TrimSpace(item.Timezone) == "" {
		item.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return errors.New("monitoring.maintenance.error.invalidTimezone")
	}
	if len(item.MonitorIDs) == 0 && item.MonitorID == nil && len(item.Tags) == 0 {
		return errors.New("monitoring.maintenance.error.monitorRequired")
	}
	switch strings.ToLower(strings.TrimSpace(item.Strategy)) {
	case "single":
		if item.StartsAt.IsZero() || item.EndsAt.IsZero() || !item.EndsAt.After(item.StartsAt) {
			return errors.New("monitoring.error.invalidWindow")
		}
	case "cron":
		if strings.TrimSpace(item.Schedule.CronExpression) == "" || item.Schedule.DurationMin <= 0 {
			return errors.New("monitoring.maintenance.error.invalidCron")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(strings.TrimSpace(item.Schedule.CronExpression)); err != nil {
			return errors.New("monitoring.maintenance.error.invalidCron")
		}
	case "interval":
		if item.Schedule.IntervalDays <= 0 || !validHHMM(item.Schedule.WindowStart) || !validHHMM(item.Schedule.WindowEnd) {
			return errors.New("monitoring.maintenance.error.invalidInterval")
		}
	case "weekday":
		if len(item.Schedule.Weekdays) == 0 || !validHHMM(item.Schedule.WindowStart) || !validHHMM(item.Schedule.WindowEnd) {
			return errors.New("monitoring.maintenance.error.invalidWeekday")
		}
	case "monthday":
		if (len(item.Schedule.MonthDays) == 0 && !item.Schedule.UseLastDay) || !validHHMM(item.Schedule.WindowStart) || !validHHMM(item.Schedule.WindowEnd) {
			return errors.New("monitoring.maintenance.error.invalidMonthday")
		}
	case "rrule":
		if !item.StartsAt.IsZero() && !item.EndsAt.IsZero() && !item.EndsAt.After(item.StartsAt) {
			return errors.New("monitoring.error.invalidWindow")
		}
		if item.IsRecurring {
			if item.RRuleText == "" {
				return errors.New("monitoring.error.invalidRRule")
			}
		}
	default:
		return errors.New("monitoring.maintenance.error.invalidStrategy")
	}
	if item.Schedule.ActiveFrom != nil && item.Schedule.ActiveUntil != nil && !item.Schedule.ActiveUntil.After(*item.Schedule.ActiveFrom) {
		return errors.New("monitoring.error.invalidWindow")
	}
	return nil
}

func payloadToMaintenanceSchedule(in maintenanceSchedulePayload) store.MaintenanceSchedule {
	out := store.MaintenanceSchedule{
		CronExpression: strings.TrimSpace(in.CronExpression),
		DurationMin:    in.DurationMin,
		IntervalDays:   in.IntervalDays,
		Weekdays:       in.Weekdays,
		MonthDays:      in.MonthDays,
		UseLastDay:     in.UseLastDay,
		WindowStart:    strings.TrimSpace(in.WindowStart),
		WindowEnd:      strings.TrimSpace(in.WindowEnd),
	}
	if !in.ActiveFrom.IsZero() {
		val := in.ActiveFrom.UTC()
		out.ActiveFrom = &val
	}
	if !in.ActiveUntil.IsZero() {
		val := in.ActiveUntil.UTC()
		out.ActiveUntil = &val
	}
	return out
}

func scheduleChanged(in maintenanceSchedulePayload) bool {
	return strings.TrimSpace(in.CronExpression) != "" || in.DurationMin != 0 || in.IntervalDays != 0 || len(in.Weekdays) > 0 || len(in.MonthDays) > 0 || in.UseLastDay || strings.TrimSpace(in.WindowStart) != "" || strings.TrimSpace(in.WindowEnd) != "" || !in.ActiveFrom.IsZero() || !in.ActiveUntil.IsZero()
}

func defaultMaintenanceStrategy(item *store.MonitorMaintenance) string {
	if item == nil {
		return "single"
	}
	if item.IsRecurring || strings.TrimSpace(item.RRuleText) != "" {
		return "rrule"
	}
	return "single"
}

func validHHMM(raw string) bool {
	_, err := time.Parse("15:04", strings.TrimSpace(raw))
	return err == nil
}

func (h *MonitoringHandler) ensureMaintenanceMonitorIDs(r *http.Request, ids []int64) error {
	for _, id := range ids {
		if id <= 0 {
			return errors.New("monitoring.maintenance.error.monitorRequired")
		}
		item, err := h.store.GetMonitor(r.Context(), id)
		if err != nil {
			return errors.New("monitoring.error.monitorNotFound")
		}
		if item == nil {
			return errors.New("monitoring.error.monitorNotFound")
		}
	}
	return nil
}
