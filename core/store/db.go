package store

import (
	"database/sql"
	"errors"
	"flag"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/utils"
	_ "modernc.org/sqlite"
)

func NewDB(cfg *config.AppConfig, logger *utils.Logger) (*sql.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	if driver == "" {
		switch {
		case strings.TrimSpace(cfg.DBURL) != "":
			driver = "postgres"
		case isTestRuntime() && strings.TrimSpace(cfg.DBPath) != "":
			driver = "sqlite"
		default:
			driver = "postgres"
		}
	}
	switch driver {
	case "postgres", "pg":
		if strings.TrimSpace(cfg.DBURL) == "" {
			return nil, errors.New("BERKUT_DB_URL is required for postgres")
		}
		db, err := sql.Open(postgresDriverName, cfg.DBURL)
		if err != nil {
			if logger != nil {
				logger.Errorf("db open failed: %v", err)
			}
			return nil, err
		}
		if logger != nil {
			logger.Printf("db open postgres")
		}
		return db, nil
	case "sqlite":
		if !isTestRuntime() {
			return nil, errors.New("sqlite driver is supported only in go test runtime")
		}
		if strings.TrimSpace(cfg.DBPath) == "" {
			return nil, errors.New("DBPath is required for sqlite")
		}
		db, err := sql.Open("sqlite", cfg.DBPath)
		if err != nil {
			if logger != nil {
				logger.Errorf("db open failed: %v", err)
			}
			return nil, err
		}
		// Improve concurrent read/write behavior for integration-style tests.
		// modernc.org/sqlite supports these pragmas.
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
		_, _ = db.Exec("PRAGMA journal_mode = WAL;")
		if logger != nil {
			logger.Printf("db open sqlite (test runtime)")
		}
		return db, nil
	default:
		return nil, errors.New("unsupported db driver: " + driver)
	}
}

func isTestRuntime() bool {
	return flag.Lookup("test.v") != nil
}
