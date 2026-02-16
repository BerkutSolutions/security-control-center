package backups

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/core/auth"
	corebackups "berkut-scc/core/backups"
	"berkut-scc/core/store"
	"github.com/go-chi/chi/v5"
)

type mockService struct {
	downloadFn func(ctx context.Context, id int64) (*corebackups.DownloadArtifact, error)
}

func (m *mockService) ListArtifacts(ctx context.Context, filter corebackups.ListArtifactsFilter) ([]corebackups.BackupArtifact, error) {
	return nil, nil
}
func (m *mockService) GetArtifact(ctx context.Context, id int64) (*corebackups.BackupArtifact, error) {
	return nil, nil
}
func (m *mockService) DownloadArtifact(ctx context.Context, id int64) (*corebackups.DownloadArtifact, error) {
	if m.downloadFn == nil {
		return nil, corebackups.NewDomainError(corebackups.ErrorCodeNotFound, corebackups.ErrorKeyNotFound)
	}
	return m.downloadFn(ctx, id)
}
func (m *mockService) CreateBackup(ctx context.Context) (*corebackups.CreateBackupResult, error) {
	return nil, nil
}
func (m *mockService) CreateBackupWithOptions(ctx context.Context, opts corebackups.CreateBackupOptions) (*corebackups.CreateBackupResult, error) {
	return m.CreateBackup(ctx)
}
func (m *mockService) ImportBackup(ctx context.Context, req corebackups.ImportBackupRequest) (*corebackups.BackupArtifact, error) {
	return nil, nil
}
func (m *mockService) DeleteBackup(ctx context.Context, id int64) error {
	return nil
}
func (m *mockService) StartRestore(ctx context.Context, artifactID int64, requestedBy string) (*corebackups.RestoreRun, error) {
	return nil, nil
}
func (m *mockService) StartRestoreDryRun(ctx context.Context, artifactID int64, requestedBy string) (*corebackups.RestoreRun, error) {
	return nil, nil
}
func (m *mockService) GetRestoreRun(ctx context.Context, id int64) (*corebackups.RestoreRun, error) {
	return nil, nil
}
func (m *mockService) GetPlan(ctx context.Context) (*corebackups.BackupPlan, error) {
	return nil, nil
}
func (m *mockService) UpdatePlan(ctx context.Context, plan corebackups.BackupPlan, requestedBy string) (*corebackups.BackupPlan, error) {
	return nil, nil
}
func (m *mockService) EnablePlan(ctx context.Context, requestedBy string) (*corebackups.BackupPlan, error) {
	return nil, nil
}
func (m *mockService) DisablePlan(ctx context.Context, requestedBy string) (*corebackups.BackupPlan, error) {
	return nil, nil
}
func (m *mockService) IsMaintenanceMode(ctx context.Context) bool {
	return false
}
func (m *mockService) RunAutoBackup(ctx context.Context) error {
	return nil
}
func (m *mockService) UploadMaxBytes() int64 {
	return 1024
}

func TestDownloadBackupNotFound(t *testing.T) {
	h := NewHandler(&mockService{
		downloadFn: func(ctx context.Context, id int64) (*corebackups.DownloadArtifact, error) {
			return nil, corebackups.NewDomainError(corebackups.ErrorCodeNotFound, corebackups.ErrorKeyNotFound)
		},
	}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/backups/10/download", nil)
	req = withSession(req, "alice", 7)
	req = withURLParams(req, map[string]string{"id": "10"})
	rr := httptest.NewRecorder()

	h.DownloadBackup(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["error"]["code"] != corebackups.ErrorCodeNotFound {
		t.Fatalf("unexpected code: %s", body["error"]["code"])
	}
	if body["error"]["i18n_key"] != corebackups.ErrorKeyNotFound {
		t.Fatalf("unexpected key: %s", body["error"]["i18n_key"])
	}
}

func TestDownloadBackupNotReady(t *testing.T) {
	h := NewHandler(&mockService{
		downloadFn: func(ctx context.Context, id int64) (*corebackups.DownloadArtifact, error) {
			return nil, corebackups.NewDomainError(corebackups.ErrorCodeNotReady, corebackups.ErrorKeyNotReady)
		},
	}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/backups/10/download", nil)
	req = withSession(req, "alice", 7)
	req = withURLParams(req, map[string]string{"id": "10"})
	rr := httptest.NewRecorder()

	h.DownloadBackup(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestDownloadBackupSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.bscc")
	payload := []byte("bscc-data")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	h := NewHandler(&mockService{
		downloadFn: func(ctx context.Context, id int64) (*corebackups.DownloadArtifact, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			return &corebackups.DownloadArtifact{
				ID:       id,
				Filename: "mybackup",
				Size:     int64(len(payload)),
				ModTime:  time.Now().UTC(),
				Reader:   f,
			}, nil
		},
	}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/backups/11/download", nil)
	req = withSession(req, "alice", 7)
	req = withURLParams(req, map[string]string{"id": "11"})
	rr := httptest.NewRecorder()

	h.DownloadBackup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("unexpected content-type: %s", got)
	}
	if got := rr.Header().Get("Content-Disposition"); got != "attachment; filename=\"mybackup.bscc\"" {
		t.Fatalf("unexpected content-disposition: %s", got)
	}
	if got := rr.Header().Get("Content-Length"); got != "9" {
		t.Fatalf("unexpected content-length: %s", got)
	}
	if rr.Body.String() != string(payload) {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func withURLParams(req *http.Request, vars map[string]string) *http.Request {
	rc := chi.NewRouteContext()
	for k, v := range vars {
		rc.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rc)
	return req.WithContext(ctx)
}

func withSession(req *http.Request, username string, userID int64) *http.Request {
	sess := &store.SessionRecord{Username: username, UserID: userID}
	ctx := context.WithValue(req.Context(), auth.SessionContextKey, sess)
	return req.WithContext(ctx)
}
