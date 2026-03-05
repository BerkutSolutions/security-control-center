package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

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
