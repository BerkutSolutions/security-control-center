package tests

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func setupMonitoringStore(t *testing.T) (store.MonitoringStore, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "monitoring.db")}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.NewMonitoringStore(db), func() { db.Close() }
}

func TestMonitoringHTTPCheckOKDown(t *testing.T) {
	var mode int32 = 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := atomic.LoadInt32(&mode)
		w.WriteHeader(int(code))
	}))
	defer srv.Close()

	settings := store.MonitorSettings{
		DefaultTimeoutSec:  2,
		DefaultIntervalSec: 60,
		EngineEnabled:      true,
		AllowPrivateNetworks: true,
	}
	okMon := store.Monitor{
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    2,
		IntervalSec:   60,
	}
	res := monitoring.CheckMonitor(context.Background(), okMon, settings)
	if !res.OK {
		t.Fatalf("expected OK, got error=%s", res.Error)
	}
	atomic.StoreInt32(&mode, 500)
	res = monitoring.CheckMonitor(context.Background(), okMon, settings)
	if res.OK {
		t.Fatalf("expected DOWN for 500 response")
	}
}

func TestMonitoringTCPCheckOKDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	settings := store.MonitorSettings{
		DefaultTimeoutSec:    2,
		DefaultIntervalSec:   60,
		EngineEnabled:        true,
		AllowPrivateNetworks: true,
	}
	mon := store.Monitor{
		Type:        "tcp",
		Host:        "127.0.0.1",
		Port:        addr.Port,
		TimeoutSec:  2,
		IntervalSec: 60,
	}
	res := monitoring.CheckMonitor(context.Background(), mon, settings)
	if !res.OK {
		t.Fatalf("expected TCP OK, got error=%s", res.Error)
	}
	_ = ln.Close()
	time.Sleep(50 * time.Millisecond)
	res = monitoring.CheckMonitor(context.Background(), mon, settings)
	if res.OK {
		t.Fatalf("expected TCP DOWN after listener closed")
	}
}

func TestMonitoringStateTransitionsCreateEvents(t *testing.T) {
	storeSvc, cleanup := setupMonitoringStore(t)
	defer cleanup()

	settings, err := storeSvc.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	if err := storeSvc.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("settings update: %v", err)
	}

	var code int32 = 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()

	mon := &store.Monitor{
		Name:          "Test",
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
	}
	id, err := storeSvc.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	mon.ID = id
	engine := monitoring.NewEngine(storeSvc, utils.NewLogger())

	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check now 1: %v", err)
	}
	state, _ := storeSvc.GetMonitorState(context.Background(), id)
	if state == nil || state.Status != "up" {
		t.Fatalf("expected up state after first check")
	}
	atomic.StoreInt32(&code, 500)
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check now 2: %v", err)
	}
	events, _ := storeSvc.ListEvents(context.Background(), id, time.Now().UTC().Add(-24*time.Hour))
	if len(events) == 0 || events[0].EventType != "down" {
		t.Fatalf("expected down event")
	}
	atomic.StoreInt32(&code, 200)
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check now 3: %v", err)
	}
	events, _ = storeSvc.ListEvents(context.Background(), id, time.Now().UTC().Add(-24*time.Hour))
	foundUp := false
	for _, ev := range events {
		if ev.EventType == "up" {
			foundUp = true
		}
	}
	if !foundUp {
		t.Fatalf("expected up event after recovery")
	}
}

func TestMonitoringMetricsRetention(t *testing.T) {
	storeSvc, cleanup := setupMonitoringStore(t)
	defer cleanup()
	mon := &store.Monitor{
		Name:          "Retention",
		Type:          "http",
		URL:           "https://example.com",
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
	}
	id, err := storeSvc.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	oldTS := time.Now().UTC().Add(-40 * 24 * time.Hour)
	newTS := time.Now().UTC().Add(-2 * time.Hour)
	_, _ = storeSvc.AddMetric(context.Background(), &store.MonitorMetric{MonitorID: id, TS: oldTS, LatencyMs: 120, OK: true})
	_, _ = storeSvc.AddMetric(context.Background(), &store.MonitorMetric{MonitorID: id, TS: newTS, LatencyMs: 90, OK: true})
	_, err = storeSvc.DeleteMetricsBefore(context.Background(), time.Now().UTC().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("retention delete: %v", err)
	}
	items, err := storeSvc.ListMetrics(context.Background(), id, time.Now().UTC().Add(-365*24*time.Hour))
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 metric after retention, got %d", len(items))
	}
}

func TestMonitoringPermissions(t *testing.T) {
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	if !policy.Allowed([]string{"admin"}, "monitoring.view") {
		t.Fatalf("admin should view monitoring")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.manage") {
		t.Fatalf("admin should manage monitoring")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.settings.manage") {
		t.Fatalf("admin should manage monitoring settings")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.certs.view") {
		t.Fatalf("admin should view monitoring certs")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.maintenance.manage") {
		t.Fatalf("admin should manage maintenance")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.notifications.manage") {
		t.Fatalf("admin should manage notifications")
	}
	if !policy.Allowed([]string{"admin"}, "monitoring.incidents.link") {
		t.Fatalf("admin should link incidents")
	}
	if policy.Allowed([]string{"analyst"}, "monitoring.view") {
		t.Fatalf("analyst must not view monitoring by default")
	}
}
