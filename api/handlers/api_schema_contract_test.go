package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestAPISchemaContractAccessesGetAndPut(t *testing.T) {
	_, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	h := NewAccessesHandler(modules, audits)

	// PUT contract
	putBody := []byte(`{"items":[{"user":"u1","services":["svc-a"]}]}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/accesses", bytes.NewReader(putBody))
	putRR := httptest.NewRecorder()
	h.Put(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("PUT /api/accesses expected 200, got %d: %s", putRR.Code, putRR.Body.String())
	}
	putJSON := decodeJSONObj(t, putRR.Body.Bytes())
	assertExactTopLevelKeys(t, putJSON, []string{"ok"})
	if ok, _ := putJSON["ok"].(bool); !ok {
		t.Fatalf("PUT /api/accesses expected ok=true, got %#v", putJSON["ok"])
	}

	// GET contract
	getReq := httptest.NewRequest(http.MethodGet, "/api/accesses", nil)
	getRR := httptest.NewRecorder()
	h.Get(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GET /api/accesses expected 200, got %d: %s", getRR.Code, getRR.Body.String())
	}
	getJSON := decodeJSONObj(t, getRR.Body.Bytes())
	assertExactTopLevelKeys(t, getJSON, []string{"items"})
	if _, ok := getJSON["items"].([]any); !ok {
		t.Fatalf("GET /api/accesses expected items array, got %#v", getJSON["items"])
	}
}

func TestAPISchemaContractNotificationsSettingsGetAndPut(t *testing.T) {
	_, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	h := NewNotificationsSettingsHandler(modules, audits, nil, nil, nil)

	// GET default contract
	getReq := httptest.NewRequest(http.MethodGet, "/api/notifications/settings", nil)
	getRR := httptest.NewRecorder()
	h.Get(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GET /api/notifications/settings expected 200, got %d: %s", getRR.Code, getRR.Body.String())
	}
	getJSON := decodeJSONObj(t, getRR.Body.Bytes())
	assertHasTopLevelKeys(t, getJSON, []string{"monitoring_enabled", "accesses_enabled", "accesses_types"})
	assertAllowedTopLevelKeys(t, getJSON, []string{"monitoring_enabled", "accesses_enabled", "accesses_types", "accesses_channel_id"})
	if _, ok := getJSON["monitoring_enabled"].(bool); !ok {
		t.Fatalf("monitoring_enabled must be bool, got %#v", getJSON["monitoring_enabled"])
	}
	if _, ok := getJSON["accesses_enabled"].(bool); !ok {
		t.Fatalf("accesses_enabled must be bool, got %#v", getJSON["accesses_enabled"])
	}
	if _, ok := getJSON["accesses_types"].([]any); !ok {
		t.Fatalf("accesses_types must be array, got %#v", getJSON["accesses_types"])
	}

	// PUT contract with channel id
	putBody := []byte(`{
		"monitoring_enabled": true,
		"accesses_enabled": true,
		"accesses_types": ["create", "dismissal", "test"],
		"accesses_channel_id": 5
	}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/notifications/settings", bytes.NewReader(putBody))
	putRR := httptest.NewRecorder()
	h.Put(putRR, putReq)
	if putRR.Code != http.StatusOK {
		t.Fatalf("PUT /api/notifications/settings expected 200, got %d: %s", putRR.Code, putRR.Body.String())
	}
	putJSON := decodeJSONObj(t, putRR.Body.Bytes())
	assertHasTopLevelKeys(t, putJSON, []string{"monitoring_enabled", "accesses_enabled", "accesses_types", "accesses_channel_id"})
	assertAllowedTopLevelKeys(t, putJSON, []string{"monitoring_enabled", "accesses_enabled", "accesses_types", "accesses_channel_id"})
	if _, ok := putJSON["accesses_channel_id"].(float64); !ok {
		t.Fatalf("accesses_channel_id must be number, got %#v", putJSON["accesses_channel_id"])
	}
}

func TestAPISchemaContractServicesAndTagsCatalogGet(t *testing.T) {
	_, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	services := NewServicesCatalogHandler(modules, audits)
	tags := NewTagsCatalogHandler(modules, audits)
	handlers := []struct {
		name string
		get  func(http.ResponseWriter, *http.Request)
		path string
	}{
		{name: "services", get: services.Get, path: "/api/services"},
		{name: "tags", get: tags.Get, path: "/api/catalog/tags"},
	}

	for _, tc := range handlers {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		tc.get(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s expected 200, got %d: %s", tc.path, rr.Code, rr.Body.String())
		}
		payload := decodeJSONObj(t, rr.Body.Bytes())
		assertExactTopLevelKeys(t, payload, []string{"items"})
		if _, ok := payload["items"].([]any); !ok {
			t.Fatalf("%s items must be array, got %#v", tc.name, payload["items"])
		}
	}
}

func TestAPISchemaContractClassificationsCatalogGet(t *testing.T) {
	_, modules, audits, cleanup := setupSchemaStores(t)
	defer cleanup()

	h := NewClassificationsCatalogHandler(modules, audits)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/catalog/classifications", nil)
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/catalog/classifications expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	payload := decodeJSONObj(t, rr.Body.Bytes())
	assertExactTopLevelKeys(t, payload, []string{"labels", "order"})
	if _, ok := payload["labels"].(map[string]any); !ok {
		t.Fatalf("classifications labels must be object, got %#v", payload["labels"])
	}
	if _, ok := payload["order"].([]any); !ok {
		t.Fatalf("classifications order must be array, got %#v", payload["order"])
	}
}

func setupSchemaStores(t *testing.T) (*sql.DB, store.AppModuleStateStore, store.AuditStore, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "api_schema_contract.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", err)
	}
	cleanup := func() { _ = db.Close() }
	return db, store.NewAppModuleStateStore(db), store.NewAuditStore(db), cleanup
}

func decodeJSONObj(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode JSON: %v; body=%s", err, string(raw))
	}
	return out
}

func assertHasTopLevelKeys(t *testing.T, payload map[string]any, keys []string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing top-level key %q in payload: %#v", key, payload)
		}
	}
}

func assertAllowedTopLevelKeys(t *testing.T, payload map[string]any, allowed []string) {
	t.Helper()
	allowedSet := map[string]struct{}{}
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	var unexpected []string
	for key := range payload {
		if _, ok := allowedSet[key]; !ok {
			unexpected = append(unexpected, key)
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Fatalf("unexpected top-level keys: %v; payload=%#v", unexpected, payload)
	}
}

func assertExactTopLevelKeys(t *testing.T, payload map[string]any, expected []string) {
	t.Helper()
	assertHasTopLevelKeys(t, payload, expected)
	assertAllowedTopLevelKeys(t, payload, expected)
}
