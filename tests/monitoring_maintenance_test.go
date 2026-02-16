package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestMaintenanceActiveWindow(t *testing.T) {
	storeSvc, cleanup := setupMonitoringStore(t)
	defer cleanup()
	mon := &store.Monitor{
		Name:        "Maint",
		Type:        "http",
		URL:         "https://example.com",
		Method:      "GET",
		IntervalSec: 60,
		TimeoutSec:  2,
		IsActive:    true,
		Tags:        []string{"OPS"},
	}
	id, err := storeSvc.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	now := time.Now().UTC()
	window := &store.MonitorMaintenance{
		Name:      "Window",
		MonitorID: &id,
		Tags:      []string{"OPS"},
		StartsAt:  now.Add(-1 * time.Hour),
		EndsAt:    now.Add(1 * time.Hour),
		IsActive:  true,
	}
	if _, err := storeSvc.CreateMaintenance(context.Background(), window); err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	active, err := storeSvc.ActiveMaintenanceFor(context.Background(), id, mon.Tags, now)
	if err != nil {
		t.Fatalf("active maintenance: %v", err)
	}
	if len(active) == 0 {
		t.Fatalf("expected active maintenance")
	}
	inactive, err := storeSvc.ActiveMaintenanceFor(context.Background(), id, mon.Tags, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("active maintenance future: %v", err)
	}
	if len(inactive) != 0 {
		t.Fatalf("expected maintenance to be inactive")
	}
}

func TestMaintenanceEvents(t *testing.T) {
	storeSvc, cleanup := setupMonitoringStore(t)
	defer cleanup()
	settings, _ := storeSvc.GetSettings(context.Background())
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	_ = storeSvc.UpdateSettings(context.Background(), settings)

	var code int32 = 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()

	mon := &store.Monitor{
		Name:          "Maint monitor",
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
		Tags:          []string{"OPS"},
	}
	id, err := storeSvc.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	state := &store.MonitorState{
		MonitorID:         id,
		Status:            "down",
		LastResultStatus:  "down",
		MaintenanceActive: false,
	}
	if err := storeSvc.UpsertMonitorState(context.Background(), state); err != nil {
		t.Fatalf("state: %v", err)
	}
	now := time.Now().UTC()
	window := &store.MonitorMaintenance{
		Name:      "Window",
		MonitorID: &id,
		StartsAt:  now.Add(-1 * time.Hour),
		EndsAt:    now.Add(1 * time.Hour),
		IsActive:  true,
	}
	if _, err := storeSvc.CreateMaintenance(context.Background(), window); err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	engine := monitoring.NewEngine(storeSvc, utils.NewLogger())
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check now: %v", err)
	}
	events, _ := storeSvc.ListEvents(context.Background(), id, time.Now().UTC().Add(-24*time.Hour))
	foundStart := false
	for _, ev := range events {
		if ev.EventType == "maintenance_start" {
			foundStart = true
		}
	}
	if !foundStart {
		t.Fatalf("expected maintenance_start event")
	}
	window.IsActive = false
	if err := storeSvc.UpdateMaintenance(context.Background(), window); err != nil {
		t.Fatalf("update maintenance: %v", err)
	}
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check now 2: %v", err)
	}
	events, _ = storeSvc.ListEvents(context.Background(), id, time.Now().UTC().Add(-24*time.Hour))
	foundEnd := false
	for _, ev := range events {
		if ev.EventType == "maintenance_end" {
			foundEnd = true
		}
	}
	if !foundEnd {
		t.Fatalf("expected maintenance_end event")
	}
}

func TestEventsFeedFilters(t *testing.T) {
	storeSvc, cleanup := setupMonitoringStore(t)
	defer cleanup()
	mon := &store.Monitor{
		Name:        "Feed",
		Type:        "http",
		URL:         "https://example.com",
		Method:      "GET",
		IntervalSec: 60,
		TimeoutSec:  2,
		IsActive:    true,
		Tags:        []string{"CORE"},
	}
	id, err := storeSvc.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	now := time.Now().UTC()
	_, _ = storeSvc.AddEvent(context.Background(), &store.MonitorEvent{MonitorID: id, TS: now.Add(-10 * time.Minute), EventType: "up", Message: ""})
	_, _ = storeSvc.AddEvent(context.Background(), &store.MonitorEvent{MonitorID: id, TS: now.Add(-5 * time.Minute), EventType: "down", Message: ""})
	filter := store.EventFilter{
		Since: now.Add(-1 * time.Hour),
		Types: []string{"down"},
		Tags:  []string{"CORE"},
		Limit: 10,
	}
	items, err := storeSvc.ListEventsFeed(context.Background(), filter)
	if err != nil {
		t.Fatalf("events feed: %v", err)
	}
	if len(items) != 1 || items[0].EventType != "down" {
		t.Fatalf("expected filtered down event")
	}
}
