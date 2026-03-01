package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestMetricsEndpointDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath:         filepath.Join(dir, "metrics.db"),
		Pepper:         "pepper",
		DeploymentMode: "home",
		RunMode:        "worker",
		Observability:  config.ObservabilityConfig{MetricsEnabled: false},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	s := NewServer(cfg, logger, ServerDeps{DB: db, Roles: store.NewRolesStore(db)})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestMetricsEndpointRequiresTokenByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath:         filepath.Join(dir, "metrics.db"),
		Pepper:         "pepper",
		DeploymentMode: "enterprise",
		RunMode:        "worker",
		Observability:  config.ObservabilityConfig{MetricsEnabled: true},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	s := NewServer(cfg, logger, ServerDeps{DB: db, Roles: store.NewRolesStore(db)})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestMetricsEndpointAllowsUnauthInHomeWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath:         filepath.Join(dir, "metrics.db"),
		Pepper:         "pepper",
		DeploymentMode: "home",
		RunMode:        "worker",
		Observability: config.ObservabilityConfig{
			MetricsEnabled:           true,
			MetricsAllowUnauthInHome: true,
		},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	s := NewServer(cfg, logger, ServerDeps{DB: db, Roles: store.NewRolesStore(db)})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); body == "" {
		t.Fatalf("expected metrics response body")
	}
}

func TestMetricsEndpointTokenAuth(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath:         filepath.Join(dir, "metrics.db"),
		Pepper:         "pepper",
		DeploymentMode: "enterprise",
		RunMode:        "worker",
		Observability: config.ObservabilityConfig{
			MetricsEnabled: true,
			MetricsToken:   "0123456789abcdef0123456789abcdef",
		},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	s := NewServer(cfg, logger, ServerDeps{DB: db, Roles: store.NewRolesStore(db)})

	rrDenied := httptest.NewRecorder()
	reqDenied := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	s.router.ServeHTTP(rrDenied, reqDenied)
	if rrDenied.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rrDenied.Code)
	}

	rrOK := httptest.NewRecorder()
	reqOK := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	reqOK.Header.Set("Authorization", "Bearer "+cfg.Observability.MetricsToken)
	s.router.ServeHTTP(rrOK, reqOK)
	if rrOK.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rrOK.Code, rrOK.Body.String())
	}
}
