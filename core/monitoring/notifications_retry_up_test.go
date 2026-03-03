package monitoring

import (
	"context"
	"testing"
	"time"

	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type testTelegramSender struct {
	msgs []TelegramMessage
}

func (s *testTelegramSender) Send(ctx context.Context, msg TelegramMessage) error {
	s.msgs = append(s.msgs, msg)
	return nil
}

func TestNoUpNotificationWhenStatusCarriedAsUpDuringRetry(t *testing.T) {
	db := mustMonitoringTestDB(t)
	monStore := store.NewMonitoringStore(db)
	sender := &testTelegramSender{}
	enc, err := utils.NewEncryptorFromString("12345678901234567890123456789012")
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	engine := NewEngineWithDeps(monStore, nil, nil, "", enc, sender, nil)

	base := time.Date(2026, 3, 3, 11, 46, 0, 0, time.UTC)
	id, err := monStore.CreateMonitor(context.Background(), &store.Monitor{
		Name:             "Registry",
		Type:             "http",
		URL:              "https://registry.hantico.ru/",
		Method:           "GET",
		IntervalSec:      60,
		TimeoutSec:       10,
		Retries:          2,
		RetryIntervalSec: 5,
		AllowedStatus:    []string{"200-399"},
		IsActive:         true,
		IsPaused:         false,
		CreatedBy:        1,
		CreatedAt:        base.Add(-time.Hour),
		UpdatedAt:        base.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	mon, err := monStore.GetMonitor(context.Background(), id)
	if err != nil || mon == nil {
		t.Fatalf("get monitor: %v", err)
	}

	tokenBlob, err := enc.EncryptToBlob([]byte("test-token"))
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	_, err = monStore.CreateNotificationChannel(context.Background(), &store.NotificationChannel{
		Type:                "telegram",
		Name:                "default",
		TelegramBotTokenEnc: tokenBlob,
		TelegramChatID:      "1",
		TemplateText:        "",
		IsDefault:           true,
		CreatedBy:           1,
		IsActive:            true,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	lastDown := base.Add(-time.Minute)
	if err := monStore.UpsertNotificationState(context.Background(), &store.MonitorNotificationState{
		MonitorID:          id,
		LastDownNotifiedAt: &lastDown,
	}); err != nil {
		t.Fatalf("seed notification state: %v", err)
	}

	// Simulate a failed attempt that scheduled a retry and carried the previous status as "up" to avoid flapping.
	retryAt := base.Add(30 * time.Second)
	nextChecked := base
	prev := &store.MonitorState{MonitorID: id, LastResultStatus: "down"}
	next := &store.MonitorState{
		MonitorID:        id,
		LastResultStatus: "up",
		LastCheckedAt:    &nextChecked,
		RetryAt:          &retryAt,
		RetryAttempt:     1,
	}
	settings := store.MonitorSettings{NotifyUpConfirmations: 1}

	engine.handleAutomation(context.Background(), *mon, prev, next, CheckResult{
		CheckedAt:  base,
		OK:         false,
		LatencyMs:  10002,
		Error:      "monitoring.error.timeout",
		StatusCode: nil,
	}, nil, settings)

	if len(sender.msgs) != 0 {
		t.Fatalf("expected no up notification for retrying failure, got %d", len(sender.msgs))
	}
}

