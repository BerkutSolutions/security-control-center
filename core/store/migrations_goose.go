package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"berkut-scc/core/utils"
	"github.com/pressly/goose/v3"
)

//go:embed migrations_pg/*.sql
var gooseMigrationsPgFS embed.FS

const gooseTable = "goose_db_version"

func applyGooseMigrations(ctx context.Context, db *sql.DB, logger *utils.Logger) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	isPG, err := isPostgresDB(ctx, db)
	if err != nil {
		return err
	}
	if !isPG {
		return fmt.Errorf("only postgres is supported in hardcut mode")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetBaseFS(gooseMigrationsPgFS)
	if err := enforceHardcutMigrationPolicy(ctx, db, isPG); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("applying goose migrations")
	}
	if err := goose.UpContext(ctx, db, "migrations_pg"); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("goose migrations applied")
	}
	return nil
}

// Hardcut policy: if DB has user tables but is not goose-versioned, reject startup.
func enforceHardcutMigrationPolicy(ctx context.Context, db *sql.DB, isPG bool) error {
	hasGoose, err := tableExists(ctx, db, gooseTable)
	if err != nil {
		return err
	}
	if hasGoose {
		return nil
	}
	var userTables int
	if isPG {
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(1)
			FROM information_schema.tables
			WHERE table_schema='public'
				AND table_type='BASE TABLE'
				AND table_name <> $1
		`, gooseTable).Scan(&userTables); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("only postgres is supported in hardcut mode")
	}
	if userTables > 0 {
		return fmt.Errorf("legacy database is not supported in hardcut mode: reset DB and run fresh migrations")
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	isPG, err := isPostgresDB(ctx, db)
	if err != nil {
		return false, err
	}
	if !isPG {
		return false, fmt.Errorf("only postgres is supported in hardcut mode")
	}
	var n int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.tables
		WHERE table_schema='public' AND table_name=$1
	`, name).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func isPostgresDB(ctx context.Context, db *sql.DB) (bool, error) {
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return false, err
	}
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return false, nil
	}
	return true, nil
}
