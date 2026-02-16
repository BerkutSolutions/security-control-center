package handlers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestAuthLockoutIncidentLifecycle(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath: filepath.Join(dir, "auth_lockout_incident.db"),
		Incidents: config.IncidentsConfig{
			RegNoFormat: "INC-{year}-{seq:05}",
		},
		Security: config.SecurityConfig{
			AuthLockoutIncident: true,
		},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	incidents := store.NewIncidentsStore(db)
	audits := store.NewAuditStore(db)
	h := &AuthHandler{
		cfg:       cfg,
		incidents: incidents,
		audits:    audits,
		logger:    logger,
	}
	user := &store.User{ID: 42, Username: "alice"}
	now := time.Now().UTC()

	h.ensureAuthLockoutIncident(context.Background(), user, 1, now)
	open, err := incidents.FindOpenIncidentBySource(context.Background(), authLockoutSource, user.ID)
	if err != nil || open == nil {
		t.Fatalf("expected open incident, err=%v", err)
	}

	h.ensureAuthLockoutIncident(context.Background(), user, 2, now.Add(1*time.Minute))
	list, err := incidents.ListIncidents(context.Background(), store.IncidentFilter{IncludeDeleted: true, Limit: 10})
	if err != nil {
		t.Fatalf("list incidents: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected single open incident, got %d", len(list))
	}

	h.resolveAuthLockoutIncident(context.Background(), user, now.Add(2*time.Minute))
	updated, err := incidents.GetIncident(context.Background(), open.ID)
	if err != nil || updated == nil {
		t.Fatalf("get incident: %v", err)
	}
	if updated.Status != "closed" {
		t.Fatalf("expected closed incident status, got %q", updated.Status)
	}
}
