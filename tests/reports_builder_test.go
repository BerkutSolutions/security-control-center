package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	taskstore "berkut-scc/tasks/store"
)

type reportEnv struct {
	cfg       *config.AppConfig
	user      *store.User
	handler   *handlers.ReportsHandler
	docsSvc   *docs.Service
	docs      store.DocsStore
	users     store.UsersStore
	reports   store.ReportsStore
	incidents store.IncidentsStore
	cleanup   func()
}

func setupReportsBuilder(t *testing.T) reportEnv {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath: filepath.Join(dir, "test.db"),
		Docs: config.DocsConfig{
			StoragePath:       filepath.Join(dir, "docs"),
			EncryptionKey:     "12345678901234567890123456789012",
			RegTemplate:       "{level}.{year}.{seq}",
			VersionLimit:      5,
			PerFolderSequence: false,
			Watermark: config.WatermarkConfig{
				Enabled:  false,
				MinLevel: "CONFIDENTIAL",
			},
			Converters: config.ConvertersConfig{Enabled: false},
		},
		Incidents: config.IncidentsConfig{
			RegNoFormat: "INC-{year}-{seq}",
			StorageDir:  filepath.Join(dir, "incidents"),
		},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := store.NewUsersStore(db)
	u := &store.User{
		Username:       "admin",
		FullName:       "Admin User",
		Department:     "Sec",
		Position:       "Lead",
		ClearanceLevel: int(docs.ClassificationTopSecret),
		ClearanceTags:  docs.TagList,
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	uid, err := users.Create(context.Background(), u, []string{"admin"})
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	u.ID = uid
	docsStore := store.NewDocsStore(db)
	reportsStore := store.NewReportsStore(db)
	incStore := store.NewIncidentsStore(db)
	ctrlStore := store.NewControlsStore(db)
	monStore := store.NewMonitoringStore(db)
	audits := store.NewAuditStore(db)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	docsSvc, err := docs.NewService(cfg, docsStore, users, audits, logger)
	if err != nil {
		t.Fatalf("docs svc: %v", err)
	}
	incSvc, err := incidents.NewService(cfg, audits)
	if err != nil {
		t.Fatalf("inc svc: %v", err)
	}
	taskSvc := tasks.NewService(taskstore.NewStore(db))
	handler := handlers.NewReportsHandler(cfg, docsStore, reportsStore, users, policy, docsSvc, incStore, incSvc, ctrlStore, monStore, taskSvc, audits, logger)
	return reportEnv{
		cfg:       cfg,
		user:      u,
		handler:   handler,
		docsSvc:   docsSvc,
		docs:      docsStore,
		users:     users,
		reports:   reportsStore,
		incidents: incStore,
		cleanup:   func() { db.Close() },
	}
}

func TestReportBuildCreatesSnapshotAndVersion(t *testing.T) {
	env := setupReportsBuilder(t)
	defer env.cleanup()
	doc := &store.Document{
		Title:               "Builder Report",
		Status:              docs.StatusDraft,
		ClassificationLevel: int(docs.ClassificationInternal),
		DocType:             "report",
		InheritACL:          true,
		CreatedBy:           env.user.ID,
	}
	acl := []store.ACLRule{
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "view"},
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "edit"},
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "manage"},
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "export"},
	}
	docID, err := env.docs.CreateDocument(context.Background(), doc, acl, env.cfg.Docs.RegTemplate, env.cfg.Docs.PerFolderSequence)
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}
	doc.ID = docID
	if err := env.reports.UpsertReportMeta(context.Background(), &store.ReportMeta{DocID: docID, Status: "draft"}); err != nil {
		t.Fatalf("report meta: %v", err)
	}
	sections := []store.ReportSection{
		{SectionType: "custom_md", Title: "Custom", IsEnabled: true, Config: map[string]any{"key": "custom", "markdown": "## Custom\\n\\nBody"}},
	}
	if err := env.reports.ReplaceReportSections(context.Background(), docID, sections); err != nil {
		t.Fatalf("sections: %v", err)
	}
	v, err := env.docsSvc.SaveVersion(context.Background(), docs.SaveRequest{
		Doc:      doc,
		Author:   env.user,
		Format:   docs.FormatMarkdown,
		Content:  []byte("initial"),
		Reason:   "init",
		IndexFTS: true,
	})
	if err != nil {
		t.Fatalf("save init: %v", err)
	}
	doc.CurrentVersion = v.Version
	_ = env.docs.UpdateDocument(context.Background(), doc)
	body, _ := json.Marshal(map[string]any{"reason": "build", "mode": "replace"})
	req := httptest.NewRequest(http.MethodPost, "/api/reports/1/build", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": fmt.Sprintf("%d", doc.ID)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: env.user.ID, Username: env.user.Username}))
	rr := httptest.NewRecorder()
	env.handler.Build(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("build status: %d", rr.Code)
	}
	vers, _ := env.docs.ListVersions(context.Background(), doc.ID)
	if len(vers) < 2 {
		t.Fatalf("expected new version, got %d", len(vers))
	}
	snaps, err := env.reports.ListReportSnapshots(context.Background(), doc.ID)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("expected snapshot")
	}
	snap, _, err := env.reports.GetReportSnapshot(context.Background(), snaps[0].ID)
	if err != nil || snap == nil {
		t.Fatalf("snapshot fetch failed")
	}
	sum := sha256.Sum256([]byte(snap.SnapshotJSON))
	if hex.EncodeToString(sum[:]) != snap.Sha256 {
		t.Fatalf("snapshot sha mismatch")
	}
}

func TestCreateReportFromIncident(t *testing.T) {
	env := setupReportsBuilder(t)
	defer env.cleanup()
	incident := &store.Incident{
		Title:               "Test incident",
		Description:         "Summary",
		Severity:            "high",
		Status:              "open",
		OwnerUserID:         env.user.ID,
		ClassificationLevel: int(docs.ClassificationInternal),
		ClassificationTags:  []string{},
		CreatedBy:           env.user.ID,
		UpdatedBy:           env.user.ID,
		CreatedAt:           time.Now().UTC().Add(-time.Hour),
		UpdatedAt:           time.Now().UTC(),
	}
	acl := []store.ACLRule{{SubjectType: "user", SubjectID: env.user.Username, Permission: "view"}}
	incidentID, err := env.incidents.CreateIncident(context.Background(), incident, nil, acl, env.cfg.Incidents.RegNoFormat)
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	incident.ID = incidentID
	body, _ := json.Marshal(map[string]any{"incident_id": incidentID})
	req := httptest.NewRequest(http.MethodPost, "/api/reports/from-incident", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: env.user.ID, Username: env.user.Username}))
	rr := httptest.NewRecorder()
	env.handler.CreateFromIncident(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var resp struct {
		ReportID int64 `json:"report_id"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ReportID == 0 {
		t.Fatalf("missing report_id")
	}
	sections, _ := env.reports.ListReportSections(context.Background(), resp.ReportID)
	if len(sections) < 4 {
		t.Fatalf("expected incident report sections, got %d", len(sections))
	}
}
