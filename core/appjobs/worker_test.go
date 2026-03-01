package appjobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func mustTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "tmp.db"), Pepper: "pepper", DBURL: ""}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWorkerProcessesQueuedJob(t *testing.T) {
	db := mustTestDB(t)
	jobs := store.NewAppJobsStore(db)
	modules := store.NewAppModuleStateStore(db)
	audits := store.NewAuditStore(db)

	cfg := &config.AppConfig{
		Docs:      config.DocsConfig{StorageDir: filepath.Join(t.TempDir(), "docs")},
		Incidents: config.IncidentsConfig{StorageDir: filepath.Join(t.TempDir(), "incidents")},
		Backups:   config.BackupsConfig{Path: filepath.Join(t.TempDir(), "backups")},
	}
	worker := NewWorker(cfg, db, jobs, modules, audits, utils.NewLogger())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	worker.StartWithContext(ctx)
	t.Cleanup(func() { _ = worker.StopWithContext(context.Background()) })

	id, err := jobs.Create(context.Background(), store.AppJobCreate{
		Type:      "reinit",
		Scope:     "module",
		ModuleID:  "monitoring",
		Mode:      "partial",
		StartedBy: "u1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if job != nil && (job.Status == StatusFinished || job.Status == StatusFailed || job.Status == StatusCanceled) {
			if job.Status != StatusFinished {
				t.Fatalf("expected finished, got %s", job.Status)
			}
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("job did not finish in time")
}
