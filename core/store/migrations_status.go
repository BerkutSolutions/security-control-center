package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

type MigrationStatus struct {
	NowUTC time.Time `json:"now_utc"`

	LegacyDatabase bool  `json:"legacy_database"`
	HasGooseTable  bool  `json:"has_goose_table"`
	CurrentVersion int64 `json:"current_version"`
	LatestVersion  int64 `json:"latest_version"`
	HasPending     bool  `json:"has_pending"`
}

func GetMigrationStatus(ctx context.Context, db *sql.DB) (MigrationStatus, error) {
	now := time.Now().UTC()
	latest, err := latestGooseMigrationVersion()
	if err != nil {
		return MigrationStatus{NowUTC: now}, err
	}
	if db == nil {
		return MigrationStatus{NowUTC: now, LatestVersion: latest}, fmt.Errorf("nil db")
	}

	hasGoose, err := tableExists(ctx, db, gooseTable)
	if err != nil {
		return MigrationStatus{NowUTC: now, LatestVersion: latest}, err
	}

	legacy, err := isLegacyDatabase(ctx, db, hasGoose)
	if err != nil {
		return MigrationStatus{NowUTC: now, LatestVersion: latest}, err
	}
	if legacy {
		return MigrationStatus{
			NowUTC:         now,
			LegacyDatabase: true,
			HasGooseTable:  false,
			CurrentVersion: 0,
			LatestVersion:  latest,
			HasPending:     true,
		}, nil
	}

	current := int64(0)
	if hasGoose {
		cur, derr := getGooseDBVersion(ctx, db)
		if derr != nil {
			return MigrationStatus{NowUTC: now, LatestVersion: latest}, derr
		}
		current = cur
	}
	return MigrationStatus{
		NowUTC:         now,
		LegacyDatabase: false,
		HasGooseTable:  hasGoose,
		CurrentVersion: current,
		LatestVersion:  latest,
		HasPending:     latest > current,
	}, nil
}

func isLegacyDatabase(ctx context.Context, db *sql.DB, hasGoose bool) (bool, error) {
	if hasGoose {
		return false, nil
	}
	var userTables int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.tables
		WHERE table_schema='public'
			AND table_type='BASE TABLE'
			AND table_name <> $1
	`, gooseTable).Scan(&userTables); err != nil {
		return false, err
	}
	return userTables > 0, nil
}

func getGooseDBVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var v int64
	// goose_db_version stores applied versions; this query returns max applied version.
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id), 0) FROM `+gooseTable).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func latestGooseMigrationVersion() (int64, error) {
	entries, err := fs.Glob(gooseMigrationsPgFS, "migrations_pg/*.sql")
	if err != nil {
		return 0, err
	}
	var max int64
	for _, p := range entries {
		base := p
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		// filename: 00026_app_jobs.sql
		parts := strings.SplitN(base, "_", 2)
		if len(parts) == 0 {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimLeft(parts[0], "0"), 10, 64)
		if err != nil {
			// allow "00001" -> empty after TrimLeft
			if strings.Trim(parts[0], "0") == "" {
				n = 0
			} else {
				continue
			}
		}
		if n > max {
			max = n
		}
	}
	return max, nil
}
