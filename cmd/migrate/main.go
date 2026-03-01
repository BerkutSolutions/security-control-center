package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/appmeta"
	"berkut-scc/core/backups"
	backupsstore "berkut-scc/core/backups/store"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := maybeCreatePreUpgradeBackup(context.Background(), cfg, db, logger); err != nil {
		logger.Fatalf("pre-upgrade backup: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		logger.Fatalf("migrations: %v", err)
	}
	logger.Printf("migrations applied")
}

func maybeCreatePreUpgradeBackup(ctx context.Context, cfg *config.AppConfig, db *sql.DB, logger *utils.Logger) error {
	if cfg == nil || db == nil {
		return nil
	}
	if !cfg.Upgrade.BackupBeforeMigrate {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	status, err := store.GetMigrationStatus(checkCtx, db)
	if err != nil {
		return err
	}
	// Only create a pre-upgrade backup if DB is already goose-versioned and has pending migrations.
	// This avoids breaking first-time installs where the DB is still empty.
	if status.LegacyDatabase || !status.HasGooseTable || !status.HasPending {
		return nil
	}

	label := strings.TrimSpace(cfg.Upgrade.BackupLabel)
	if label == "" {
		label = fmt.Sprintf("preupgrade:%s:%s", appmeta.AppVersion, time.Now().UTC().Format("20060102-150405"))
	}
	repo := backupsstore.NewRepository(db)
	audits := store.NewAuditStore(db)
	svc := backups.NewService(cfg, db, repo, audits, logger)
	_, err = svc.CreateBackupWithOptions(ctx, backups.CreateBackupOptions{
		Label:        label,
		IncludeFiles: cfg.Upgrade.BackupIncludeFiles,
		RequestedBy:  "upgrade",
	})
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("pre-upgrade backup created label=%s", label)
	}
	return nil
}
