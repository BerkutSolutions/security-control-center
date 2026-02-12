package store

import (
	"database/sql"
	"os"
	"path/filepath"

	"berkut-scc/config"
	"berkut-scc/core/utils"
	_ "modernc.org/sqlite"
)

func NewDB(cfg *config.AppConfig, logger *utils.Logger) (*sql.DB, error) {
	if err := ensureDBDir(cfg.DBPath); err != nil {
		if logger != nil {
			logger.Errorf("db dir create failed: %v", err)
		}
		return nil, err
	}
	db, err := sql.Open("sqlite", cfg.DBPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		if logger != nil {
			logger.Errorf("db open failed: %v", err)
		}
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if logger != nil {
		logger.Printf("db open at %s", cfg.DBPath)
	}
	return db, nil
}

func ensureDBDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
