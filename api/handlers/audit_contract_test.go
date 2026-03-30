package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestAuditContractAccessesCriticalActions(t *testing.T) {
	_, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	h := NewAccessesHandler(modules, audits)

	cases := []struct {
		name        string
		eventType   string
		wantAction  string
		wantDetails []string
	}{
		{name: "create", eventType: "create", wantAction: "accesses.create", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "edit", eventType: "edit", wantAction: "accesses.edit", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "supplement", eventType: "supplement", wantAction: "accesses.supplement", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "blocked", eventType: "blocked", wantAction: "accesses.blocked", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "unblocked", eventType: "unblocked", wantAction: "accesses.unblocked", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "delete", eventType: "delete", wantAction: "accesses.delete", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "dismissal", eventType: "dismissal", wantAction: "accesses.dismissal", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "test", eventType: "test", wantAction: "accesses.test", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
		{name: "cleanup", eventType: "cleanup", wantAction: "accesses.cleanup", wantDetails: []string{"items_count=1", "user=Иван", "services_count=2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"items": []map[string]any{{"user": "Иван", "services": []string{"svc-a", "svc-b"}}},
				"event": map[string]any{
					"type":     tc.eventType,
					"user":     "Иван",
					"services": []string{"svc-a", "svc-b"},
				},
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPut, "/api/accesses", bytes.NewReader(body))
			req = withAuditUser(req, "admin")
			rr := httptest.NewRecorder()
			h.Put(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("PUT /api/accesses failed: %d %s", rr.Code, rr.Body.String())
			}

			row := mustFindAuditByAction(t, audits, tc.wantAction)
			if row.Username != "admin" {
				t.Fatalf("unexpected actor for %s: got %q", tc.wantAction, row.Username)
			}
			for _, frag := range tc.wantDetails {
				if !strings.Contains(row.Details, frag) {
					t.Fatalf("audit details for %s missing %q: %q", tc.wantAction, frag, row.Details)
				}
			}
		})
	}
}

func TestAuditContractNotificationsSettingsAndAccessesEvent(t *testing.T) {
	db, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	monitoringStore := store.NewMonitoringStore(db)
	enc, err := utils.NewEncryptorFromString("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	engine := monitoring.NewEngine(monitoringStore, utils.NewLogger())
	h := NewNotificationsSettingsHandler(modules, audits, monitoringStore, engine, enc)

	putBody := []byte(`{"monitoring_enabled":true,"accesses_enabled":true,"accesses_types":["create","dismissal"]}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/notifications/settings", bytes.NewReader(putBody))
	putReq = withAuditUser(putReq, "admin")
	putRR := httptest.NewRecorder()
	h.Put(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("PUT /api/notifications/settings failed: %d %s", putRR.Code, putRR.Body.String())
	}
	settingsAudit := mustFindAuditByAction(t, audits, "notifications.settings.update")
	if settingsAudit.Username != "admin" {
		t.Fatalf("unexpected actor for notifications.settings.update: %q", settingsAudit.Username)
	}
	if !strings.Contains(settingsAudit.Details, `"accesses_enabled":true`) {
		t.Fatalf("settings update audit missing payload details: %q", settingsAudit.Details)
	}

	// No channel selected -> skipped with explicit reason; must be in audit.
	eventBody := []byte(`{"event_type":"create","user":"Иван","services":["svc-a"],"actor":"admin"}`)
	eventReq := httptest.NewRequest(http.MethodPost, "/api/notifications/accesses/event", bytes.NewReader(eventBody))
	eventReq = withAuditUser(eventReq, "admin")
	eventRR := httptest.NewRecorder()
	h.HandleAccessesEvent(eventRR, eventReq)
	if eventRR.Code != http.StatusOK {
		t.Fatalf("POST /api/notifications/accesses/event failed: %d %s", eventRR.Code, eventRR.Body.String())
	}
	sendAudit := mustFindAuditByAction(t, audits, "notifications.accesses.send")
	if sendAudit.Username != "admin" {
		t.Fatalf("unexpected actor for notifications.accesses.send: %q", sendAudit.Username)
	}
	for _, frag := range []string{"event=create", "status=skipped", "reason=channel_missing"} {
		if !strings.Contains(sendAudit.Details, frag) {
			t.Fatalf("notifications.accesses.send details missing %q: %q", frag, sendAudit.Details)
		}
	}
}

func withAuditUser(r *http.Request, username string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: username,
		Roles:    []string{"admin"},
	})
	return r.WithContext(ctx)
}

func mustFindAuditByAction(t *testing.T, audits store.AuditStore, action string) store.AuditRecord {
	t.Helper()
	rows, err := audits.List(context.Background())
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	for _, row := range rows {
		if row.Action == action {
			return row
		}
	}
	t.Fatalf("audit action %q not found", action)
	return store.AuditRecord{}
}
