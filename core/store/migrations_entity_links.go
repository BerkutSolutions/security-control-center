package store

import (
	"context"
	"database/sql"
	"fmt"
)

func ensureEntityLinksSchema(ctx context.Context, db *sql.DB) error {
	cols, err := tableInfo(ctx, db, "entity_links")
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}
	var hasSourceType bool
	var hasSourceID bool
	var hasRelationType bool
	var docIDNotNull bool
	for _, c := range cols {
		switch c.Name {
		case "source_type":
			hasSourceType = true
		case "source_id":
			hasSourceID = true
		case "relation_type":
			hasRelationType = true
		case "doc_id":
			docIDNotNull = c.NotNull
		}
	}
	if docIDNotNull || !hasSourceType || !hasSourceID || !hasRelationType {
		if err := rebuildEntityLinks(ctx, db, hasSourceType, hasSourceID, hasRelationType); err != nil {
			return err
		}
		hasSourceType = true
		hasSourceID = true
		hasRelationType = true
	}
	if !hasSourceType {
		if _, err := db.ExecContext(ctx, "ALTER TABLE entity_links ADD COLUMN source_type TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add column entity_links.source_type: %w", err)
		}
	}
	if !hasSourceID {
		if _, err := db.ExecContext(ctx, "ALTER TABLE entity_links ADD COLUMN source_id TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add column entity_links.source_id: %w", err)
		}
	}
	if !hasRelationType {
		if _, err := db.ExecContext(ctx, "ALTER TABLE entity_links ADD COLUMN relation_type TEXT NOT NULL DEFAULT 'related'"); err != nil {
			return fmt.Errorf("add column entity_links.relation_type: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE entity_links
		SET source_type='doc', source_id=CAST(doc_id AS TEXT)
		WHERE (source_type IS NULL OR TRIM(source_type)='') AND doc_id IS NOT NULL
	`); err != nil {
		return fmt.Errorf("backfill entity_links sources: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE entity_links
		SET relation_type='related'
		WHERE relation_type IS NULL OR TRIM(relation_type)=''
	`); err != nil {
		return fmt.Errorf("backfill entity_links relation_type: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_entity_links_source ON entity_links(source_type, source_id)`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_entity_links_target ON entity_links(target_type, target_id)`); err != nil {
		return err
	}
	return nil
}

type tableColumn struct {
	Name    string
	NotNull bool
}

func tableInfo(ctx context.Context, db *sql.DB, table string) ([]tableColumn, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []tableColumn
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, tableColumn{Name: name, NotNull: notnull == 1})
	}
	return cols, rows.Err()
}

func rebuildEntityLinks(ctx context.Context, db *sql.DB, hasSourceType, hasSourceID, hasRelationType bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS entity_links_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			doc_id INTEGER,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation_type TEXT NOT NULL DEFAULT 'related',
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY(doc_id) REFERENCES docs(id) ON DELETE CASCADE
		);`); err != nil {
		tx.Rollback()
		return err
	}
	var insertSQL string
	if hasSourceType && hasSourceID && hasRelationType {
		insertSQL = `
			INSERT INTO entity_links_new (id, doc_id, source_type, source_id, target_type, target_id, relation_type, created_at)
			SELECT id, doc_id,
				COALESCE(NULLIF(source_type, ''), 'doc'),
				COALESCE(NULLIF(source_id, ''), CAST(doc_id AS TEXT)),
				target_type, target_id, COALESCE(NULLIF(relation_type, ''), 'related'), created_at
			FROM entity_links;`
	} else if hasSourceType && hasSourceID {
		insertSQL = `
			INSERT INTO entity_links_new (id, doc_id, source_type, source_id, target_type, target_id, relation_type, created_at)
			SELECT id, doc_id, 'doc', CAST(doc_id AS TEXT), target_type, target_id, created_at
			FROM entity_links;`
	} else {
		insertSQL = `
			INSERT INTO entity_links_new (id, doc_id, source_type, source_id, target_type, target_id, relation_type, created_at)
			SELECT id, doc_id, 'doc', CAST(doc_id AS TEXT), target_type, target_id, 'related', created_at
			FROM entity_links;`
	}
	if _, err := tx.ExecContext(ctx, insertSQL); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE entity_links`); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE entity_links_new RENAME TO entity_links`); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
