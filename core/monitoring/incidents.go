package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (e *Engine) handleAutoIncident(ctx context.Context, m store.Monitor, prev, next *store.MonitorState, rawStatus string, now time.Time) {
	if e.incidents == nil || !m.AutoIncident {
		return
	}
	if next == nil || m.IsPaused || next.MaintenanceActive {
		return
	}
	prevStatus := ""
	if prev != nil {
		prevStatus = strings.ToLower(strings.TrimSpace(prev.LastResultStatus))
	}
	if rawStatus == "down" && prevStatus != "down" {
		existing, _ := e.incidents.FindOpenIncidentBySource(ctx, "monitoring", m.ID)
		if existing != nil {
			return
		}
		owner := m.CreatedBy
		sev := strings.ToLower(strings.TrimSpace(m.IncidentSeverity))
		if sev == "" {
			sev = "low"
		}
		monitorName := strings.TrimSpace(m.Name)
		if monitorName == "" {
			monitorName = fmt.Sprintf("Монитор #%d", m.ID)
		}
		title := fmt.Sprintf("%s: %s", notifyText("ru", "monitoring.notify.downTitle"), monitorName)
		desc := "🚨 Монитор недоступен"
		detectedAt := now.Format("2006-01-02T15:04:05-07:00")
		incident := &store.Incident{
			Title:       title,
			Description: desc,
			Severity:    sev,
			Status:      "open",
			OwnerUserID: owner,
			CreatedBy:   owner,
			UpdatedBy:   owner,
			Source:      "monitoring",
			SourceRefID: &m.ID,
			Meta: store.IncidentMeta{
				IncidentType:          "Отказ сервиса",
				DetectionSource:       "Мониторинг",
				SLAResponse:           "1 час",
				FirstResponseDeadline: "8 часов",
				WhatHappened:          fmt.Sprintf("Недоступен монитор %s", monitorName),
				DetectedAt:            detectedAt,
				AffectedSystems:       monitorName,
				Risk:                  "да",
				ActionsTaken:          "Направлено уведомление ответственным и создан инцидент",
			},
		}
		id, err := e.incidents.CreateIncident(ctx, incident, nil, nil, e.incidentRegFormat)
		if err != nil {
			if e.logger != nil {
				e.logger.Errorf("monitoring auto incident create: %v", err)
			}
			return
		}
		_, _ = e.incidents.AddIncidentTimeline(ctx, &store.IncidentTimelineEvent{
			IncidentID: id,
			EventType:  "monitoring.auto_create",
			Message:    strings.TrimSpace(m.Name),
			CreatedBy:  owner,
			EventAt:    now.UTC(),
		})
		if e.audits != nil {
			_ = e.audits.Log(ctx, "system", "monitoring.incident.auto_create", fmt.Sprintf("%d", id))
		}
		return
	}
	if rawStatus == "up" && prevStatus == "down" {
		existing, _ := e.incidents.FindOpenIncidentBySource(ctx, "monitoring", m.ID)
		if existing == nil {
			return
		}
		owner := existing.OwnerUserID
		closed, err := e.incidents.CloseIncident(ctx, existing.ID, owner)
		if err != nil || closed == nil {
			if e.logger != nil && err != nil {
				e.logger.Errorf("monitoring auto incident close: %v", err)
			}
			return
		}
		_, _ = e.incidents.AddIncidentTimeline(ctx, &store.IncidentTimelineEvent{
			IncidentID: existing.ID,
			EventType:  "monitoring.auto_close",
			Message:    strings.TrimSpace(m.Name),
			CreatedBy:  owner,
			EventAt:    now.UTC(),
		})
		if e.audits != nil {
			_ = e.audits.Log(ctx, "system", "monitoring.incident.auto_close", fmt.Sprintf("%d", existing.ID))
		}
	}
}
