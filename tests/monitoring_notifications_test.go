package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type mockTelegramSender struct {
	sent []monitoring.TelegramMessage
}

func (m *mockTelegramSender) Send(ctx context.Context, msg monitoring.TelegramMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func setupMonitoringDeps(t *testing.T) (store.MonitoringStore, store.IncidentsStore, *utils.Encryptor, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "monitoring_notify.db")}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	enc, err := utils.NewEncryptorFromString("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	return store.NewMonitoringStore(db), store.NewIncidentsStore(db), enc, func() { db.Close() }
}

func addTelegramChannel(t *testing.T, ms store.MonitoringStore, enc *utils.Encryptor) int64 {
	t.Helper()
	tokenEnc, err := enc.EncryptToBlob([]byte("test-token"))
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	ch := &store.NotificationChannel{
		Type:                "telegram",
		Name:                "Ops",
		TelegramBotTokenEnc: tokenEnc,
		TelegramChatID:      "12345",
		Silent:              true,
		ProtectContent:      true,
		IsDefault:           true,
		IsActive:            true,
		CreatedBy:           1,
	}
	id, err := ms.CreateNotificationChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return id
}

func TestMonitoringTelegramDownUp(t *testing.T) {
	ms, is, enc, cleanup := setupMonitoringDeps(t)
	defer cleanup()
	settings, _ := ms.GetSettings(context.Background())
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	settings.NotifySuppressMinutes = 0
	settings.NotifyRepeatDownMinutes = 10
	if err := ms.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("settings update: %v", err)
	}
	addTelegramChannel(t, ms, enc)
	var code int32 = 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()
	mon := &store.Monitor{
		Name:          "Down monitor",
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
		CreatedBy:     1,
	}
	id, err := ms.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	sender := &mockTelegramSender{}
	engine := monitoring.NewEngineWithDeps(ms, is, nil, "INC-{seq}", enc, sender, utils.NewLogger())
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check down: %v", err)
	}
	atomic.StoreInt32(&code, 200)
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check up: %v", err)
	}
	if len(sender.sent) < 2 {
		t.Fatalf("expected 2 notifications, got %d", len(sender.sent))
	}
	if !containsText(sender.sent[0].Text, "Монитор недоступен") {
		t.Fatalf("expected down notification text")
	}
	if !containsText(sender.sent[1].Text, "Монитор восстановлен") {
		t.Fatalf("expected up notification text")
	}
}

func TestMonitoringSuppression(t *testing.T) {
	ms, is, enc, cleanup := setupMonitoringDeps(t)
	defer cleanup()
	settings, _ := ms.GetSettings(context.Background())
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	settings.NotifySuppressMinutes = 10
	settings.NotifyRepeatDownMinutes = 30
	if err := ms.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("settings update: %v", err)
	}
	addTelegramChannel(t, ms, enc)
	var code int32 = 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()
	mon := &store.Monitor{
		Name:          "Suppress monitor",
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
		CreatedBy:     1,
	}
	id, err := ms.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	sender := &mockTelegramSender{}
	engine := monitoring.NewEngineWithDeps(ms, is, nil, "INC-{seq}", enc, sender, utils.NewLogger())
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check down: %v", err)
	}
	atomic.StoreInt32(&code, 200)
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check up: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected suppression to block second notification, got %d", len(sender.sent))
	}
}

func TestMonitoringAutoIncidentCreateAndClose(t *testing.T) {
	ms, is, enc, cleanup := setupMonitoringDeps(t)
	defer cleanup()
	settings, _ := ms.GetSettings(context.Background())
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	if err := ms.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("settings update: %v", err)
	}
	var code int32 = 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()
	mon := &store.Monitor{
		Name:             "Incident monitor",
		Type:             "http",
		URL:              srv.URL,
		Method:           "GET",
		AllowedStatus:    []string{"200-299"},
		IntervalSec:      60,
		TimeoutSec:       2,
		IsActive:         true,
		CreatedBy:        1,
		AutoIncident:     true,
		IncidentSeverity: "high",
	}
	id, err := ms.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	sender := &mockTelegramSender{}
	engine := monitoring.NewEngineWithDeps(ms, is, nil, "INC-{seq}", enc, sender, utils.NewLogger())
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check down: %v", err)
	}
	inc, err := is.FindOpenIncidentBySource(context.Background(), "monitoring", id)
	if err != nil || inc == nil {
		t.Fatalf("expected auto incident to be created")
	}
	if inc.Severity != "high" {
		t.Fatalf("expected incident severity to be high")
	}
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check down again: %v", err)
	}
	inc2, _ := is.FindOpenIncidentBySource(context.Background(), "monitoring", id)
	if inc2 == nil || inc2.ID != inc.ID {
		t.Fatalf("expected no duplicate incident")
	}
	atomic.StoreInt32(&code, 200)
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check up: %v", err)
	}
	closed, err := is.GetIncident(context.Background(), inc.ID)
	if err != nil || closed == nil || closed.Status != "closed" {
		t.Fatalf("expected incident to be closed")
	}
}

func TestMonitoringMaintenanceSuppression(t *testing.T) {
	ms, is, enc, cleanup := setupMonitoringDeps(t)
	defer cleanup()
	settings, _ := ms.GetSettings(context.Background())
	settings.AllowPrivateNetworks = true
	settings.EngineEnabled = true
	if err := ms.UpdateSettings(context.Background(), settings); err != nil {
		t.Fatalf("settings update: %v", err)
	}
	addTelegramChannel(t, ms, enc)
	var code int32 = 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&code)))
	}))
	defer srv.Close()
	mon := &store.Monitor{
		Name:          "Maintenance monitor",
		Type:          "http",
		URL:           srv.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		IntervalSec:   60,
		TimeoutSec:    2,
		IsActive:      true,
		CreatedBy:     1,
		AutoIncident:  true,
	}
	id, err := ms.CreateMonitor(context.Background(), mon)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	now := time.Now().UTC()
	window := &store.MonitorMaintenance{
		Name:      "Maint",
		MonitorID: &id,
		StartsAt:  now.Add(-10 * time.Minute),
		EndsAt:    now.Add(10 * time.Minute),
		IsActive:  true,
	}
	if _, err := ms.CreateMaintenance(context.Background(), window); err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	sender := &mockTelegramSender{}
	engine := monitoring.NewEngineWithDeps(ms, is, nil, "INC-{seq}", enc, sender, utils.NewLogger())
	if err := engine.CheckNow(context.Background(), id); err != nil {
		t.Fatalf("check down: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("expected no notifications during maintenance")
	}
	inc, _ := is.FindOpenIncidentBySource(context.Background(), "monitoring", id)
	if inc != nil {
		t.Fatalf("expected no auto incident during maintenance")
	}
}

func containsText(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}
