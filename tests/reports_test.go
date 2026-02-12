package tests

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/docs"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func setupReports(t *testing.T) (context.Context, *config.AppConfig, *store.User, store.DocsStore, store.ReportsStore, *docs.Service, func()) {
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
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ds := store.NewDocsStore(db)
	rs := store.NewReportsStore(db)
	us := store.NewUsersStore(db)
	audits := store.NewAuditStore(db)
	svc, err := docs.NewService(cfg, ds, us, audits, logger)
	if err != nil {
		t.Fatalf("svc: %v", err)
	}
	u := &store.User{
		Username:       "reporter",
		FullName:       "Report Owner",
		Department:     "Sec",
		Position:       "Analyst",
		ClearanceLevel: int(docs.ClassificationTopSecret),
		ClearanceTags:  docs.TagList,
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	uid, err := us.Create(context.Background(), u, []string{"admin"})
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	u.ID = uid
	cleanup := func() {
		db.Close()
	}
	return context.Background(), cfg, u, ds, rs, svc, cleanup
}

func createReportDoc(t *testing.T, ctx context.Context, ds store.DocsStore, u *store.User, cfg *config.AppConfig) *store.Document {
	t.Helper()
	doc := &store.Document{
		Title:               "Report Test",
		Status:              docs.StatusDraft,
		ClassificationLevel: int(docs.ClassificationInternal),
		ClassificationTags:  []string{},
		DocType:             "report",
		InheritACL:          true,
		CreatedBy:           u.ID,
	}
	acl := []store.ACLRule{
		{SubjectType: "user", SubjectID: u.Username, Permission: "view"},
		{SubjectType: "user", SubjectID: u.Username, Permission: "edit"},
		{SubjectType: "user", SubjectID: u.Username, Permission: "manage"},
		{SubjectType: "user", SubjectID: u.Username, Permission: "export"},
	}
	if _, err := ds.CreateDocument(ctx, doc, acl, cfg.Docs.RegTemplate, cfg.Docs.PerFolderSequence); err != nil {
		t.Fatalf("create report doc: %v", err)
	}
	return doc
}

func TestReportACLPermissions(t *testing.T) {
	ctx, cfg, user, ds, _, svc, cleanup := setupReports(t)
	defer cleanup()
	doc := createReportDoc(t, ctx, ds, user, cfg)
	docACL, _ := ds.GetDocACL(ctx, doc.ID)
	low := &store.User{ID: 99, Username: "low", ClearanceLevel: int(docs.ClassificationInternal), ClearanceTags: []string{}}
	if svc.CheckACL(low, []string{}, doc, docACL, nil, "view") {
		t.Fatalf("expected ACL deny for non-member")
	}
	if !svc.CheckACL(user, []string{"admin"}, doc, docACL, nil, "view") {
		t.Fatalf("expected ACL allow for owner")
	}
}

func TestReportContentVersioning(t *testing.T) {
	ctx, cfg, user, ds, _, svc, cleanup := setupReports(t)
	defer cleanup()
	doc := createReportDoc(t, ctx, ds, user, cfg)
	if _, err := svc.SaveVersion(ctx, docs.SaveRequest{Doc: doc, Author: user, Format: docs.FormatMarkdown, Content: []byte("v1"), Reason: "init", IndexFTS: true}); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	if _, err := svc.SaveVersion(ctx, docs.SaveRequest{Doc: doc, Author: user, Format: docs.FormatMarkdown, Content: []byte("v2"), Reason: "update", IndexFTS: true}); err != nil {
		t.Fatalf("save v2: %v", err)
	}
	vers, err := ds.ListVersions(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(vers) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(vers))
	}
}

func TestReportTemplateCRUD(t *testing.T) {
	ctx, _, _, _, rs, _, cleanup := setupReports(t)
	defer cleanup()
	tpl := &store.ReportTemplate{
		Name:            "Template 1",
		Description:     "Desc",
		TemplateMarkdown: "# Title",
		CreatedBy:       1,
	}
	if err := rs.SaveReportTemplate(ctx, tpl); err != nil {
		t.Fatalf("save template: %v", err)
	}
	list, err := rs.ListReportTemplates(ctx)
	if err != nil || len(list) == 0 {
		t.Fatalf("list templates failed")
	}
	tpl.TemplateMarkdown = "# Updated"
	if err := rs.SaveReportTemplate(ctx, tpl); err != nil {
		t.Fatalf("update template: %v", err)
	}
	if err := rs.DeleteReportTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}
}

func TestReportExportMarkdownAlwaysWorks(t *testing.T) {
	ctx, cfg, user, ds, _, svc, cleanup := setupReports(t)
	defer cleanup()
	doc := createReportDoc(t, ctx, ds, user, cfg)
	if _, err := svc.SaveVersion(ctx, docs.SaveRequest{Doc: doc, Author: user, Format: docs.FormatMarkdown, Content: []byte("body"), Reason: "init", IndexFTS: true}); err != nil {
		t.Fatalf("save: %v", err)
	}
	content := []byte("## Header\n\nbody")
	out, _, err := svc.ConvertMarkdown(ctx, "md", content, "")
	if err != nil {
		t.Fatalf("export md: %v", err)
	}
	if !strings.Contains(string(out), "Header") {
		t.Fatalf("expected header in export output")
	}
}
