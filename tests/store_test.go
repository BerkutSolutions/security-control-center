package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/bootstrap"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestMigrations(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "test.db")}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	_, err = db.Exec(`SELECT 1 FROM users LIMIT 1`)
	if err != nil {
		t.Fatalf("table users missing: %v", err)
	}
}

func TestDefaultAdminSeed(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "seed.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer db.Close()
	_ = store.ApplyMigrations(context.Background(), db, logger)
	if err := bootstrap.EnsureDefaultAdmin(context.Background(), db, cfg, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}
	row := db.QueryRow(`SELECT username FROM users LIMIT 1`)
	var u string
	if err := row.Scan(&u); err != nil || u != "admin" {
		t.Fatalf("admin not created: %v %s", err, u)
	}
	os.Remove(cfg.DBPath)
}
