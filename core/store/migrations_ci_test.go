package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/utils"
)

func TestGooseMigrationsSequenceNoGaps(t *testing.T) {
	entries, err := fs.Glob(gooseMigrationsPgFS, "migrations_pg/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 pg migrations, got %d", len(entries))
	}
	versions := make([]int, 0, len(entries))
	for _, e := range entries {
		base := filepath.Base(e)
		if len(base) < 5 {
			t.Fatalf("bad migration filename: %s", base)
		}
		v, convErr := strconv.Atoi(base[:5])
		if convErr != nil {
			t.Fatalf("parse version from %s: %v", base, convErr)
		}
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for i := 1; i < len(versions); i++ {
		if versions[i] != versions[i-1]+1 {
			t.Fatalf("migration version gap: prev=%d current=%d", versions[i-1], versions[i])
		}
	}
}

func TestSQLiteLatestMinusOneUpgradeSmoke(t *testing.T) {
	// "latest-1 -> latest" smoke for sqlite runtime: apply all migrations, then apply again.
	// The second apply simulates upgrade idempotency on a DB that is already on previous state.
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "upgrade.db"), Pepper: strings.Repeat("a", 32)}
	logger := utils.NewLogger()

	db, err := NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestSQLiteLatestMinusOneUpgradePreservesDataAndModuleCompatibility(t *testing.T) {
	if len(migrations) < 2 {
		t.Fatalf("expected at least 2 sqlite migrations, got %d", len(migrations))
	}

	ctx := context.Background()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "upgrade_preserve.db"), Pepper: strings.Repeat("b", 32)}
	logger := utils.NewLogger()

	db, err := NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Simulate latest-1 state.
	for i, stmt := range migrations[:len(migrations)-1] {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("apply sqlite latest-1 migration #%d: %v", i+1, err)
		}
	}

	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users(username, email, password_hash, salt, require_password_change, active, created_at, updated_at)
		VALUES(?, ?, ?, ?, 0, 1, ?, ?)
	`, "migr_user", "migr@example.local", "hash", "salt", now, now); err != nil {
		t.Fatalf("seed user on latest-1: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO app_module_state(module_id, applied_schema_version, applied_behavior_version, initialized_at, updated_at, last_error)
		VALUES(?, 1, 1, ?, ?, ?)
	`, "services.catalog", now, now, `{"items":[{"name":"svc-a"},{"name":"svc-b"}]}`); err != nil {
		t.Fatalf("seed app_module_state on latest-1: %v", err)
	}

	// Upgrade to latest.
	if err := ApplyMigrations(ctx, db, logger); err != nil {
		t.Fatalf("upgrade latest-1 -> latest failed: %v", err)
	}

	// Data must survive upgrade.
	var usersCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE username=?`, "migr_user").Scan(&usersCount); err != nil {
		t.Fatalf("verify users after upgrade: %v", err)
	}
	if usersCount != 1 {
		t.Fatalf("expected migrated user to survive upgrade, got count=%d", usersCount)
	}
	var moduleState string
	if err := db.QueryRowContext(ctx, `SELECT last_error FROM app_module_state WHERE module_id=?`, "services.catalog").Scan(&moduleState); err != nil {
		t.Fatalf("verify module state after upgrade: %v", err)
	}
	if !strings.Contains(moduleState, `"svc-a"`) || !strings.Contains(moduleState, `"svc-b"`) {
		t.Fatalf("module state payload was changed/lost after upgrade: %s", moduleState)
	}

	// Module compatibility after upgrade.
	modules := NewAppModuleStateStore(db)
	st, err := modules.Get(ctx, "services.catalog")
	if err != nil {
		t.Fatalf("read module state through store after upgrade: %v", err)
	}
	if st == nil || !strings.Contains(st.LastError, `"svc-a"`) {
		t.Fatalf("module store is incompatible after upgrade: %#v", st)
	}

	monitoringStore := NewMonitoringStore(db)
	settings, err := monitoringStore.GetSettings(ctx)
	if err != nil {
		t.Fatalf("monitoring settings incompatible after upgrade: %v", err)
	}
	if settings.RetentionDays <= 0 {
		t.Fatalf("unexpected monitoring settings after upgrade: %+v", settings)
	}

}
