package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/store"
)

const authLockoutSource = "auth_lockout"

func (h *AuthHandler) ensureAuthLockoutIncident(ctx context.Context, user *store.User, stage int, now time.Time) {
	if h == nil || h.incidents == nil || user == nil {
		return
	}
	if h.cfg != nil && !h.cfg.Security.AuthLockoutIncident {
		return
	}
	existing, err := h.incidents.FindOpenIncidentBySource(ctx, authLockoutSource, user.ID)
	if err == nil && existing != nil {
		return
	}
	refID := user.ID
	title := fmt.Sprintf("Repeated auth failures: %s", strings.TrimSpace(user.Username))
	meta := store.IncidentMeta{
		IncidentType:    "Authentication",
		DetectionSource: "Auth lockout automation",
		WhatHappened:    fmt.Sprintf("User %s reached lockout stage %d", strings.TrimSpace(user.Username), stage),
		DetectedAt:      now.Format(time.RFC3339),
		ActionsTaken:    "Account lockout applied automatically.",
		Tags:            []string{"auth", "lockout", "bruteforce"},
	}
	inc := &store.Incident{
		Title:       title,
		Description: "Repeated failed login attempts detected.",
		Severity:    "high",
		Status:      "open",
		OwnerUserID: user.ID,
		CreatedBy:   user.ID,
		UpdatedBy:   user.ID,
		Source:      authLockoutSource,
		SourceRefID: &refID,
		Meta:        meta,
	}
	id, err := h.incidents.CreateIncident(ctx, inc, nil, nil, h.cfg.Incidents.RegNoFormat)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("auth auto incident create: %v", err)
		}
		return
	}
	_, _ = h.incidents.AddIncidentTimeline(ctx, &store.IncidentTimelineEvent{
		IncidentID: id,
		EventType:  "auth.lockout.auto_create",
		Message:    strings.TrimSpace(user.Username),
		CreatedBy:  user.ID,
		EventAt:    now.UTC(),
	})
	if h.audits != nil {
		_ = h.audits.Log(ctx, "system", "auth.lockout.incident.auto_create", fmt.Sprintf("incident_id=%d user_id=%d stage=%d", id, user.ID, stage))
	}
}

func (h *AuthHandler) resolveAuthLockoutIncident(ctx context.Context, user *store.User, now time.Time) {
	if h == nil || h.incidents == nil || user == nil {
		return
	}
	existing, err := h.incidents.FindOpenIncidentBySource(ctx, authLockoutSource, user.ID)
	if err != nil || existing == nil {
		return
	}
	closed, err := h.incidents.CloseIncident(ctx, existing.ID, user.ID)
	if err != nil || closed == nil {
		if h.logger != nil && err != nil {
			h.logger.Errorf("auth auto incident close: %v", err)
		}
		return
	}
	_, _ = h.incidents.AddIncidentTimeline(ctx, &store.IncidentTimelineEvent{
		IncidentID: existing.ID,
		EventType:  "auth.lockout.auto_close",
		Message:    strings.TrimSpace(user.Username),
		CreatedBy:  user.ID,
		EventAt:    now.UTC(),
	})
	if h.audits != nil {
		_ = h.audits.Log(ctx, "system", "auth.lockout.incident.auto_close", fmt.Sprintf("incident_id=%d user_id=%d", existing.ID, user.ID))
	}
}
