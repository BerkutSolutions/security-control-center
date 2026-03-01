package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/appjobs"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"github.com/go-chi/chi/v5"
)

func withChiURLParamAppJobs(r *http.Request, key, val string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func TestUIFlowCompatThenStartJobThenPollProgress(t *testing.T) {
	db := mustTestDB(t)
	jobs := store.NewAppJobsStore(db)
	modules := store.NewAppModuleStateStore(db)
	audits := store.NewAuditStore(db)

	cfg := &config.AppConfig{
		Docs:      config.DocsConfig{StorageDir: filepath.Join(t.TempDir(), "docs")},
		Incidents: config.IncidentsConfig{StorageDir: filepath.Join(t.TempDir(), "incidents")},
		Backups:   config.BackupsConfig{Path: filepath.Join(t.TempDir(), "backups")},
	}
	logger := utils.NewLogger()
	worker := appjobs.NewWorker(cfg, db, jobs, modules, audits, logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	worker.StartWithContext(ctx)
	t.Cleanup(func() { _ = worker.StopWithContext(context.Background()) })

	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "admin", Permissions: []rbac.Permission{
			"app.compat.view",
			"app.compat.manage.partial",
			"settings.advanced",
			"monitoring.manage",
		}},
	})
	sess := &store.SessionRecord{UserID: 1, Username: "u1", Roles: []string{"admin"}}

	// 1) UI after login: compat report.
	compatH := NewAppCompatHandler(modules, policy)
	reqCompat := httptest.NewRequest(http.MethodGet, "/api/app/compat", nil)
	reqCompat = reqCompat.WithContext(context.WithValue(reqCompat.Context(), auth.SessionContextKey, sess))
	rrCompat := httptest.NewRecorder()
	compatH.Report(rrCompat, reqCompat)
	if rrCompat.Code != http.StatusOK {
		t.Fatalf("compat expected 200, got %d (%s)", rrCompat.Code, rrCompat.Body.String())
	}

	// 2) Start a job (partial adapt/reinit).
	jobsH := NewAppJobsHandler(jobs, policy)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"monitoring","mode":"partial"}`))
	reqCreate = reqCreate.WithContext(context.WithValue(reqCreate.Context(), auth.SessionContextKey, sess))
	rrCreate := httptest.NewRecorder()
	jobsH.Create(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d (%s)", rrCreate.Code, rrCreate.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rrCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	idVal, ok := created["id"].(float64)
	if !ok || int64(idVal) <= 0 {
		t.Fatalf("missing id in create response: %v", created)
	}
	jobID := int64(idVal)

	// 3) Poll job progress.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		reqGet := httptest.NewRequest(http.MethodGet, "/api/app/jobs/1", nil)
		reqGet = withChiURLParamAppJobs(reqGet, "id", strconv.FormatInt(jobID, 10))
		reqGet = reqGet.WithContext(context.WithValue(reqGet.Context(), auth.SessionContextKey, sess))
		rrGet := httptest.NewRecorder()
		jobsH.Get(rrGet, reqGet)
		if rrGet.Code != http.StatusOK {
			t.Fatalf("get expected 200, got %d (%s)", rrGet.Code, rrGet.Body.String())
		}
		var resp struct {
			Job store.AppJob `json:"job"`
		}
		if err := json.Unmarshal(rrGet.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode get: %v", err)
		}
		if resp.Job.ID != jobID {
			t.Fatalf("wrong job id")
		}
		if resp.Job.Status == appjobs.StatusFinished {
			if resp.Job.Progress != 100 {
				t.Fatalf("expected progress 100 on finished, got %d", resp.Job.Progress)
			}
			if resp.Job.FinishedAt == nil {
				t.Fatalf("expected finished_at set")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job did not finish in time")
}
