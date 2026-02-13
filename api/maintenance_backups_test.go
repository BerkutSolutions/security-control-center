package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/backups"
)

type maintenanceRepo struct {
	enabled bool
}

func (m *maintenanceRepo) ListArtifacts(ctx context.Context, filter backups.ListArtifactsFilter) ([]backups.BackupArtifact, error) {
	return nil, nil
}
func (m *maintenanceRepo) GetArtifact(ctx context.Context, id int64) (*backups.BackupArtifact, error) {
	return nil, backups.ErrNotFound
}
func (m *maintenanceRepo) ListSuccessfulArtifacts(ctx context.Context, limit int) ([]backups.BackupArtifact, error) {
	return nil, nil
}
func (m *maintenanceRepo) DeleteArtifact(ctx context.Context, id int64) error { return nil }
func (m *maintenanceRepo) HasRunningBackupRun(ctx context.Context) (bool, error) {
	return false, nil
}
func (m *maintenanceRepo) HasRunningRestoreRun(ctx context.Context) (bool, error) {
	return false, nil
}
func (m *maintenanceRepo) GetRestoreRun(ctx context.Context, id int64) (*backups.RestoreRun, error) {
	return nil, backups.ErrNotFound
}
func (m *maintenanceRepo) CreateRestoreRun(ctx context.Context, run *backups.RestoreRun) (*backups.RestoreRun, error) {
	return run, nil
}
func (m *maintenanceRepo) UpdateRestoreRun(ctx context.Context, run *backups.RestoreRun) error {
	return nil
}
func (m *maintenanceRepo) GetBackupPlan(ctx context.Context) (*backups.BackupPlan, error) {
	return &backups.BackupPlan{}, nil
}
func (m *maintenanceRepo) UpsertBackupPlan(ctx context.Context, plan *backups.BackupPlan) (*backups.BackupPlan, error) {
	return plan, nil
}
func (m *maintenanceRepo) SetBackupPlanEnabled(ctx context.Context, enabled bool) error { return nil }
func (m *maintenanceRepo) UpdateBackupPlanLastAutoRunAt(ctx context.Context, at time.Time) error {
	return nil
}
func (m *maintenanceRepo) GetMaintenanceMode(ctx context.Context) (bool, error) {
	return m.enabled, nil
}
func (m *maintenanceRepo) SetMaintenanceMode(ctx context.Context, enabled bool, reason string) error {
	m.enabled = enabled
	return nil
}
func (m *maintenanceRepo) CreateRun(ctx context.Context, run *backups.BackupRun) (*backups.BackupRun, error) {
	return run, nil
}
func (m *maintenanceRepo) UpdateRun(ctx context.Context, run *backups.BackupRun) error { return nil }
func (m *maintenanceRepo) CreateArtifact(ctx context.Context, artifact *backups.BackupArtifact) (*backups.BackupArtifact, error) {
	return artifact, nil
}
func (m *maintenanceRepo) AttachArtifactToRun(ctx context.Context, runID, artifactID int64) error {
	return nil
}
func (m *maintenanceRepo) GetGooseVersion(ctx context.Context) (int64, error) { return 0, nil }

func TestMaintenanceModeBlocksBackupsEndpointsExceptRestoreStatus(t *testing.T) {
	repo := &maintenanceRepo{enabled: true}
	s := &Server{
		backupsSvc: backups.NewService(&config.AppConfig{}, nil, repo, nil, nil),
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mw := s.maintenanceModeMiddleware(next)

	blockedReq := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	blockedRec := httptest.NewRecorder()
	mw.ServeHTTP(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", blockedRec.Code)
	}
	var payload map[string]map[string]string
	if err := json.Unmarshal(blockedRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload["error"]["i18n_key"] != "common.error.maintenanceMode" {
		t.Fatalf("unexpected i18n key: %s", payload["error"]["i18n_key"])
	}

	allowedRestoreReq := httptest.NewRequest(http.MethodGet, "/api/backups/restores/99", nil)
	allowedRestoreRec := httptest.NewRecorder()
	mw.ServeHTTP(allowedRestoreRec, allowedRestoreReq)
	if allowedRestoreRec.Code != http.StatusNoContent {
		t.Fatalf("restore status must be allowed, got %d", allowedRestoreRec.Code)
	}

	allowedAuthReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	allowedAuthRec := httptest.NewRecorder()
	mw.ServeHTTP(allowedAuthRec, allowedAuthReq)
	if allowedAuthRec.Code != http.StatusNoContent {
		t.Fatalf("auth endpoint must be allowed, got %d", allowedAuthRec.Code)
	}
}

func TestMaintenanceModeDisabledPassesRequests(t *testing.T) {
	repo := &maintenanceRepo{enabled: false}
	s := &Server{
		backupsSvc: backups.NewService(&config.AppConfig{}, nil, repo, nil, nil),
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rr := httptest.NewRecorder()
	s.maintenanceModeMiddleware(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected pass-through status, got %d body=%s", rr.Code, strings.TrimSpace(rr.Body.String()))
	}
}
