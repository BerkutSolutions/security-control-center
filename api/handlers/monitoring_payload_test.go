package handlers

import (
	"strings"
	"testing"

	"berkut-scc/core/store"
)

func TestPayloadToMonitorHTTPKeywordRequiresExpectedWord(t *testing.T) {
	_, err := payloadToMonitor(monitorPayload{
		Name:             "kw-check",
		Type:             "http_keyword",
		URL:              "https://example.com/health",
		Method:           "GET",
		RequestBody:      "",
		RequestBodyType:  "none",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
		AllowedStatus:    []string{"200-299"},
	}, &store.MonitorSettings{}, 1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "monitoring.error.keywordRequired") {
		t.Fatalf("expected keywordRequired error, got %v", err)
	}
}

func TestPayloadToMonitorDNSAllowsHostWithoutPort(t *testing.T) {
	m, err := payloadToMonitor(monitorPayload{
		Name:             "dns-check",
		Type:             "dns",
		Host:             "example.org",
		RequestBody:      "93.184.",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Port != 0 {
		t.Fatalf("expected zero port for dns, got %d", m.Port)
	}
}

func TestPayloadToMonitorPostgresURLValidation(t *testing.T) {
	_, err := payloadToMonitor(monitorPayload{
		Name:             "pg-bad",
		Type:             "postgres",
		URL:              "https://example.com/not-pg",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err == nil {
		t.Fatalf("expected error for non-postgres scheme")
	}

	m, err := payloadToMonitor(monitorPayload{
		Name:             "pg-ok",
		Type:             "postgres",
		URL:              "postgres://user:pass@db.local:5432/app?sslmode=disable",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Host != "db.local" {
		t.Fatalf("expected host db.local, got %q", m.Host)
	}
	if m.Port != 5432 {
		t.Fatalf("expected port 5432, got %d", m.Port)
	}
}

func TestPayloadToMonitorPushRequiresToken(t *testing.T) {
	_, err := payloadToMonitor(monitorPayload{
		Name:             "push-monitor",
		Type:             "push",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err == nil || !strings.Contains(err.Error(), "monitoring.error.pushTokenRequired") {
		t.Fatalf("expected push token error, got: %v", err)
	}
}

func TestPayloadToMonitorPingAllowsHostWithoutPort(t *testing.T) {
	m, err := payloadToMonitor(monitorPayload{
		Name:             "ping-monitor",
		Type:             "ping",
		Host:             "example.org",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Port != 0 {
		t.Fatalf("expected empty port for ping, got %d", m.Port)
	}
}

func TestPayloadToMonitorGRPCURLValidation(t *testing.T) {
	_, err := payloadToMonitor(monitorPayload{
		Name:             "grpc-bad",
		Type:             "grpc_keyword",
		URL:              "https://example.com/grpc",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err == nil {
		t.Fatalf("expected error for non-grpc scheme")
	}

	_, err = payloadToMonitor(monitorPayload{
		Name:             "grpc-ok",
		Type:             "grpc_keyword",
		URL:              "grpcs://grpc.example.local:9443",
		IntervalSec:      30,
		TimeoutSec:       10,
		Retries:          1,
		RetryIntervalSec: 5,
	}, &store.MonitorSettings{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
