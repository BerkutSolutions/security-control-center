package appjobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
)

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type ModuleDeps struct {
	DB      *sql.DB
	Cfg     *config.AppConfig
	Modules store.AppModuleStateStore
	NowUTC  time.Time
}

type ModuleResult struct {
	Counts      map[string]int64
	FilesCounts map[string]int64
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) (ModuleResult, error)) (ModuleResult, error) {
	if db == nil {
		return ModuleResult{}, errors.New("nil db")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ModuleResult{}, err
	}
	res, runErr := fn(tx)
	if runErr != nil {
		_ = tx.Rollback()
		return ModuleResult{}, runErr
	}
	if err := tx.Commit(); err != nil {
		return ModuleResult{}, err
	}
	return res, nil
}

func ensureCounts(m map[string]int64) map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return m
}

func ensureFileCounts(m map[string]int64) map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return m
}

func deleteAll(ctx context.Context, tx *sql.Tx, table string) (int64, error) {
	t := strings.TrimSpace(table)
	if t == "" {
		return 0, errors.New("empty table")
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM "+t)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func deleteWhere(ctx context.Context, tx *sql.Tx, table, where string, args ...any) (int64, error) {
	t := strings.TrimSpace(table)
	w := strings.TrimSpace(where)
	if t == "" {
		return 0, errors.New("empty table")
	}
	if w == "" {
		return 0, errors.New("empty where")
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM "+t+" WHERE "+w, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func deleteTablesInOrder(ctx context.Context, tx *sql.Tx, tables []string) (map[string]int64, error) {
	out := map[string]int64{}
	for _, table := range tables {
		name := strings.TrimSpace(table)
		if name == "" {
			continue
		}
		exists, err := tableExists(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		n, err := deleteAll(ctx, tx, name)
		if err != nil {
			return nil, fmt.Errorf("delete %s: %w", name, err)
		}
		out[name] = n
	}
	return out, nil
}

func tableExists(ctx context.Context, q queryRower, table string) (bool, error) {
	name := strings.TrimSpace(table)
	if name == "" {
		return false, errors.New("empty table")
	}
	// Try Postgres first.
	var pgCount int
	err := q.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.tables
		WHERE table_schema='public' AND table_name=$1
	`, name).Scan(&pgCount)
	if err == nil {
		return pgCount > 0, nil
	}
	// Fallback to SQLite.
	var sqliteCount int
	err2 := q.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?
	`, name).Scan(&sqliteCount)
	if err2 == nil {
		return sqliteCount > 0, nil
	}
	return false, err
}

func upsertModuleStateCompatible(ctx context.Context, deps ModuleDeps, moduleID string, schemaVersion, behaviorVersion int, resetError string) error {
	if deps.Modules == nil {
		return nil
	}
	now := deps.NowUTC
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var init *time.Time
	t := now.UTC()
	init = &t
	return deps.Modules.Upsert(ctx, &store.AppModuleState{
		ModuleID:               moduleID,
		AppliedSchemaVersion:   schemaVersion,
		AppliedBehaviorVersion: behaviorVersion,
		InitializedAt:          init,
		LastError:              strings.TrimSpace(resetError),
	})
}

func safeRemoveAllDir(dir string) (int64, error) {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return 0, errors.New("refusing to remove empty/root dir")
	}
	info, err := os.Stat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("not a directory: %s", clean)
	}
	var count int64
	entries, err := os.ReadDir(clean)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		p := filepath.Join(clean, e.Name())
		if err := os.RemoveAll(p); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
