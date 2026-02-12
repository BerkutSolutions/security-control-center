package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

type maintenancePayload struct {
	Name        string    `json:"name"`
	MonitorID   *int64    `json:"monitor_id"`
	Tags        []string  `json:"tags"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Timezone    string    `json:"timezone"`
	IsRecurring *bool     `json:"is_recurring"`
	RRuleText   string    `json:"rrule_text"`
	IsActive    *bool     `json:"is_active"`
}

func (h *MonitoringHandler) ListMaintenance(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	var payload maintenancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := payloadToMaintenance(payload, sessionUserID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := h.store.CreateMaintenance(r.Context(), item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	item.ID = id
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.maintenance.create", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, item)
}

func (h *MonitoringHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetMaintenance(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload maintenancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := mergeMaintenance(existing, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateMaintenance(r.Context(), item); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.maintenance.update", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, item)
}

func (h *MonitoringHandler) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteMaintenance(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.maintenance.delete", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func payloadToMaintenance(payload maintenancePayload, createdBy int64) (*store.MonitorMaintenance, error) {
	item := &store.MonitorMaintenance{
		Name:        strings.TrimSpace(payload.Name),
		MonitorID:   payload.MonitorID,
		Tags:        payload.Tags,
		StartsAt:    payload.StartsAt.UTC(),
		EndsAt:      payload.EndsAt.UTC(),
		Timezone:    strings.TrimSpace(payload.Timezone),
		RRuleText:   strings.TrimSpace(payload.RRuleText),
		CreatedBy:   createdBy,
	}
	if payload.IsRecurring != nil {
		item.IsRecurring = *payload.IsRecurring
	}
	if payload.IsActive != nil {
		item.IsActive = *payload.IsActive
	} else {
		item.IsActive = true
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
	if payload.MonitorID != nil {
		item.MonitorID = payload.MonitorID
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
	if payload.IsRecurring != nil {
		item.IsRecurring = *payload.IsRecurring
	}
	if payload.RRuleText != "" || (payload.IsRecurring != nil && !*payload.IsRecurring) {
		item.RRuleText = strings.TrimSpace(payload.RRuleText)
	}
	if payload.IsActive != nil {
		item.IsActive = *payload.IsActive
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
	if item.StartsAt.IsZero() || item.EndsAt.IsZero() || !item.EndsAt.After(item.StartsAt) {
		return errors.New("monitoring.error.invalidWindow")
	}
	if item.IsRecurring {
		if item.RRuleText == "" {
			return errors.New("monitoring.error.invalidRRule")
		}
		if err := validateRRule(item.RRuleText); err != nil {
			return errors.New("monitoring.error.invalidRRule")
		}
	}
	return nil
}

func validateRRule(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("empty")
	}
	parts := strings.Split(raw, ";")
	var freq string
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return errors.New("invalid")
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		val := strings.ToUpper(strings.TrimSpace(kv[1]))
		switch key {
		case "FREQ":
			freq = val
		case "INTERVAL":
			if _, err := strconv.Atoi(val); err != nil {
				return errors.New("invalid")
			}
		case "BYDAY":
			if val == "" {
				continue
			}
			for _, day := range strings.Split(val, ",") {
				if _, ok := map[string]bool{"MO": true, "TU": true, "WE": true, "TH": true, "FR": true, "SA": true, "SU": true}[strings.TrimSpace(day)]; !ok {
					return errors.New("invalid")
				}
			}
		default:
			return errors.New("invalid")
		}
	}
	if freq != "DAILY" && freq != "WEEKLY" {
		return errors.New("invalid")
	}
	return nil
}
