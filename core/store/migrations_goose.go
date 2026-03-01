package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"berkut-scc/core/utils"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
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
	if err := enforceHardcutMigrationPolicy(ctx, db, isPG); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("applying goose migrations")
	}
	migrationsFS, err := fs.Sub(gooseMigrationsPgFS, "migrations_pg")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker(
		lock.WithLockID(5887940537704921958),
		// Wait up to ~5 minutes for the lock (60 * 5s).
		lock.WithLockTimeout(5, 60),
	)
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
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
